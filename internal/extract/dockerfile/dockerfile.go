// Package dockerfile implements an extract.Extractor that reads Dockerfiles and
// emits container-image facts with provenance.
//
// What it observes:
//
//   - Consumers — every external image a Dockerfile pulls in: each `FROM <image>`
//     base and each `COPY --from=<image>`. References that name a *local build
//     stage* (`COPY --from=builder`, `FROM builder AS final`) are internal stage
//     references, satisfied within the same file, and are NOT emitted as
//     consumers.
//   - Producers — the build targets a Dockerfile yields: every named stage
//     (`... AS <alias>`) and the final target stage. These are the things a later
//     resolver can link to a published image reference (named at build time, e.g.
//     in a CI workflow).
//
// It never matches consumers to producers: edge resolution is the resolver's job
// (see package extract). Image references are normalized to the canonical
// identity form of ADR-0002 (default registry docker.io, default tag :latest,
// digest beats tag) so a base pulled here matches an image published elsewhere.
package dockerfile

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/moby/buildkit/frontend/dockerfile/instructions"
	"github.com/moby/buildkit/frontend/dockerfile/parser"
	"github.com/moby/buildkit/frontend/dockerfile/shell"

	"github.com/agentic-research/assay/internal/artifact"
	"github.com/agentic-research/assay/internal/extract"
)

// Extractor scans a root for Dockerfiles and emits container-image facts.
//
// The zero value is ready to use.
type Extractor struct{}

// compile-time assertion that Extractor honors the contract.
var _ extract.Extractor = Extractor{}

// Name identifies this extractor.
func (Extractor) Name() string { return "dockerfile" }

// Available always reports true: Dockerfile extraction is pure file parsing with
// no external dependency.
func (Extractor) Available() (bool, string) { return true, "" }

// skipDirs are directory names never worth walking for Dockerfiles.
var skipDirs = map[string]struct{}{
	".git":         {},
	"vendor":       {},
	"node_modules": {},
	"testdata":     {},
}

// Extract walks root for Dockerfiles and emits the producers and consumers each
// one declares, with file+line provenance. Files are visited in lexical order so
// the output is deterministic.
func (Extractor) Extract(root string) ([]artifact.Producer, []artifact.Consumer, error) {
	var (
		producers []artifact.Producer
		consumers []artifact.Consumer
	)
	walk := func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if _, skip := skipDirs[d.Name()]; skip && path != root {
				return fs.SkipDir
			}
			return nil
		}
		if !isDockerfile(d.Name()) {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			rel = path
		}
		ps, cs, err := extractFile(path, filepath.ToSlash(rel))
		if err != nil {
			return fmt.Errorf("dockerfile %s: %w", rel, err)
		}
		producers = append(producers, ps...)
		consumers = append(consumers, cs...)
		return nil
	}
	if err := filepath.WalkDir(root, walk); err != nil {
		return nil, nil, err
	}
	return producers, consumers, nil
}

// isDockerfile reports whether a filename names a Dockerfile by the conventional
// patterns: `Dockerfile`, `Dockerfile.<x>`, `<x>.Dockerfile`, or `Containerfile`
// (the OCI-neutral spelling).
func isDockerfile(name string) bool {
	switch {
	case name == "Dockerfile" || name == "Containerfile":
		return true
	case strings.HasPrefix(name, "Dockerfile."):
		return true
	case strings.HasSuffix(name, ".Dockerfile"):
		return true
	default:
		return false
	}
}

// extractFile parses one Dockerfile at path and emits its facts. rel is the
// slash-separated path relative to the scan root, used for provenance so reports
// are stable across machines.
func extractFile(path, rel string) ([]artifact.Producer, []artifact.Consumer, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	parsed, err := parser.Parse(bytes.NewReader(data))
	if err != nil {
		return nil, nil, fmt.Errorf("parse: %w", err)
	}
	stages, metaArgs, err := instructions.Parse(parsed.AST, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("instructions: %w", err)
	}

	// Meta args (ARG before the first FROM) are the only build args usable in
	// FROM lines, so they seed the interpolation environment. $BUILDPLATFORM and
	// the other automatic platform args are pre-declared with empty values: they
	// drop out of an image reference rather than leaving a literal "$BUILDPLATFORM"
	// behind.
	env := newEnv(metaArgs)
	lex := shell.NewLex(parser.DefaultEscapeToken)

	// Local stage aliases let us tell an internal stage reference
	// (`COPY --from=builder`) from an external image consumer. Docker treats
	// stage names case-insensitively, so the set is keyed on the lowercased name.
	stageNames := make(map[string]struct{}, len(stages))
	for _, st := range stages {
		if st.Name != "" {
			stageNames[strings.ToLower(st.Name)] = struct{}{}
		}
	}

	var (
		producers []artifact.Producer
		consumers []artifact.Consumer
	)
	for i, st := range stages {
		// FROM base: a consumer unless it references a local stage.
		base := expand(lex, env, st.BaseName)
		fromLine := firstLine(st.Location)
		if base != "" && !isLocalStage(stageNames, base) {
			consumers = append(consumers, imageConsumer(base, rel, fromLine))
		}

		// Each named stage is a build target this Dockerfile yields; the final
		// stage is the default target whether or not it is named.
		isFinal := i == len(stages)-1
		if st.Name != "" {
			producers = append(producers, stageProducer(stageRef(rel, st.Name), rel, fromLine))
		}
		if isFinal {
			producers = append(producers, stageProducer(targetRef(rel), rel, fromLine))
		}

		// COPY --from=<ref>: a consumer unless <ref> names a local stage.
		for _, cmd := range st.Commands {
			cp, ok := cmd.(*instructions.CopyCommand)
			if !ok || cp.From == "" {
				continue
			}
			ref := expand(lex, env, cp.From)
			if ref == "" || isLocalStage(stageNames, ref) {
				continue
			}
			consumers = append(consumers, imageConsumer(ref, rel, firstLine(cp.Location())))
		}
	}
	return producers, consumers, nil
}

// isLocalStage reports whether ref names one of this Dockerfile's build stages.
// A numeric ref (`COPY --from=0`) is a stage index — also internal.
func isLocalStage(stageNames map[string]struct{}, ref string) bool {
	if _, ok := stageNames[strings.ToLower(ref)]; ok {
		return true
	}
	return isStageIndex(ref)
}

// isStageIndex reports whether ref is an all-digit stage index (`--from=0`).
func isStageIndex(ref string) bool {
	if ref == "" {
		return false
	}
	for _, r := range ref {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// imageConsumer builds a container-image consumer fact from a raw reference,
// normalizing it to the ADR-0002 identity key.
func imageConsumer(raw, file string, line int) artifact.Consumer {
	return artifact.Consumer{
		Identity:   artifact.NewIdentity(artifact.KindContainerImage, NormalizeImageRef(raw)),
		Provenance: artifact.Provenance{File: file, Line: line},
	}
}

// stageProducer builds a container-image producer fact for a build target.
func stageProducer(ref, file string, line int) artifact.Producer {
	return artifact.Producer{
		Identity:   artifact.NewIdentity(artifact.KindContainerImage, ref),
		Provenance: artifact.Provenance{File: file, Line: line},
	}
}

// stageRef is the producer identity of a named stage: the Dockerfile path and the
// stage alias. Stage names are local to a file, so the path keeps two files'
// "builder" stages distinct.
func stageRef(rel, name string) string {
	return rel + "#" + strings.ToLower(name)
}

// targetRef is the producer identity of a Dockerfile's default build target (its
// final stage): the Dockerfile path itself.
func targetRef(rel string) string { return rel }

// expand resolves $VAR / ${VAR} references in a word against the build-arg env.
// On any lexing error it falls back to the raw word so a malformed interpolation
// degrades to a literal rather than dropping the fact.
func expand(lex *shell.Lex, env shell.EnvGetter, word string) string {
	if word == "" || !strings.ContainsRune(word, '$') {
		return word
	}
	out, _, err := lex.ProcessWord(word, env)
	if err != nil {
		return word
	}
	return out
}

// newEnv builds the interpolation environment from a Dockerfile's meta args,
// seeded with the automatic platform args so `--platform=$BUILDPLATFORM` and the
// $TARGET*/$BUILD* references resolve to empty rather than leaking a literal.
func newEnv(metaArgs []instructions.ArgCommand) shell.EnvGetter {
	vals := make(map[string]string, len(metaArgs)+len(autoPlatformArgs))
	for _, k := range autoPlatformArgs {
		vals[k] = ""
	}
	for _, arg := range metaArgs {
		for _, kv := range arg.Args {
			if kv.Value != nil {
				vals[kv.Key] = *kv.Value
			} else if _, ok := vals[kv.Key]; !ok {
				vals[kv.Key] = ""
			}
		}
	}
	slice := make([]string, 0, len(vals))
	for k, v := range vals {
		slice = append(slice, k+"="+v)
	}
	return shell.EnvsFromSlice(slice)
}

// autoPlatformArgs are the build args Docker pre-declares for every build; they
// are available in FROM lines without an explicit ARG.
var autoPlatformArgs = []string{
	"TARGETPLATFORM", "TARGETOS", "TARGETARCH", "TARGETVARIANT",
	"BUILDPLATFORM", "BUILDOS", "BUILDARCH", "BUILDVARIANT",
}

// firstLine returns the 1-based start line of an instruction's location, or 0
// when the location is unknown.
func firstLine(loc []parser.Range) int {
	if len(loc) == 0 {
		return 0
	}
	return loc[0].Start.Line
}
