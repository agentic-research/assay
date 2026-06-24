// Package gomod extracts artifact facts from a go.mod file: the module it
// declares (a Producer) and every module it references via require and replace
// directives (Consumers). It emits RAW references as facts and never decides
// whether a reference is internal or external — that is the resolver's job.
//
// It applies only the identity normalization needed for producer ids and
// consumer refs to match (decision 0002): module versions and pseudo-versions
// are stripped, +incompatible is dropped, major-version /vN suffixes are kept,
// and a module path deeper than its owning repo emits BOTH the full module-path
// identity and the owning-repo identity.
//
// Commented-out replace directives are intentionally included: in this corpus a
// line like `// replace github.com/agentic-research/mache => ../mache` encodes a
// local-dev cross-root link that is toggled off for CI, and the seam it names is
// exactly what the graph wants to surface.
package gomod

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/mod/modfile"

	"github.com/agentic-research/assay/internal/artifact"
	"github.com/agentic-research/assay/internal/extract"
)

// compile-time assertion: Extractor honors the extract.Extractor contract.
var _ extract.Extractor = Extractor{}

// Extractor parses the go.mod at the root of a scan root. It has no environment
// dependencies (pure file parsing), so it is always available.
type Extractor struct{}

// New returns a go.mod Extractor.
func New() Extractor { return Extractor{} }

// Name is the stable identifier used to attribute this extractor's facts.
func (Extractor) Name() string { return "gomod" }

// Available always reports true: parsing a go.mod needs no external binary.
func (Extractor) Available() (bool, string) { return true, "" }

// Extract walks the scan root for every go.mod and emits each one's module
// Producer and its require / replace Consumers. It finds nested modules, not just
// <root>/go.mod: a repo whose Go module lives in a subdirectory (e.g.
// ley-line-open's `clients/go/leyline-schema`, or any multi-module repo) still
// contributes its producer, so a require elsewhere resolves to it. A root with no
// go.mod is not an error — it simply contributes no Go-module facts.
func (Extractor) Extract(root string) ([]artifact.Producer, []artifact.Consumer, error) {
	var producers []artifact.Producer
	var consumers []artifact.Consumer

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if _, skip := skipDirs[d.Name()]; skip {
				return fs.SkipDir
			}
			return nil
		}
		if d.Name() != "go.mod" {
			return nil
		}
		p, c, err := extractFile(path)
		if err != nil {
			return err
		}
		producers = append(producers, p...)
		consumers = append(consumers, c...)
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return producers, consumers, nil
}

// skipDirs are directory names never worth walking for go.mod files: VCS and
// dependency/build caches, and agent worktree scratch trees whose throwaway
// modules would pollute the graph.
var skipDirs = map[string]struct{}{
	".git":         {},
	".claude":      {},
	"node_modules": {},
	"vendor":       {},
	"testdata":     {},
}

// extractFile parses a single go.mod and emits its module Producer and require /
// replace Consumers.
func extractFile(path string) ([]artifact.Producer, []artifact.Consumer, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}

	// Lax parsing: a require/replace with an odd version must not abort the
	// whole extraction. We only read declarations, never rewrite the file.
	f, err := modfile.Parse(path, data, nil)
	if err != nil {
		return nil, nil, err
	}

	var producers []artifact.Producer
	var consumers []artifact.Consumer

	if f.Module != nil {
		prov := artifact.Provenance{File: path, Line: lineOf(f.Module.Syntax)}
		for _, ref := range identitiesForPath(f.Module.Mod.Path) {
			producers = append(producers, artifact.Producer{
				Identity:   artifact.NewIdentity(artifact.KindGoModule, ref),
				Provenance: prov,
			})
		}
	}

	for _, req := range f.Require {
		prov := artifact.Provenance{File: path, Line: lineOf(req.Syntax)}
		for _, ref := range identitiesForPath(req.Mod.Path) {
			consumers = append(consumers, artifact.Consumer{
				Identity:   artifact.NewIdentity(artifact.KindGoModule, ref),
				Provenance: prov,
			})
		}
	}

	// Active replace directives: the left-hand (Old) module path is the
	// consumed identity — the resolver keys on the original path so the edge
	// still resolves to the producing root (decision 0002).
	for _, rep := range f.Replace {
		prov := artifact.Provenance{File: path, Line: lineOf(rep.Syntax)}
		for _, ref := range identitiesForPath(rep.Old.Path) {
			consumers = append(consumers, artifact.Consumer{
				Identity:   artifact.NewIdentity(artifact.KindGoModule, ref),
				Provenance: prov,
			})
		}
	}

	// Commented-out replace directives. modfile does not parse these into
	// f.Replace, so we recover them from the raw comment text. They carry the
	// same local-dev cross-root links as active replaces.
	for _, c := range commentedReplaces(f.Syntax) {
		prov := artifact.Provenance{File: path, Line: c.line}
		for _, ref := range identitiesForPath(c.oldPath) {
			consumers = append(consumers, artifact.Consumer{
				Identity:   artifact.NewIdentity(artifact.KindGoModule, ref),
				Provenance: prov,
			})
		}
	}

	return producers, consumers, nil
}

// lineOf returns the 1-based source line of a syntax node, or 0 if unknown.
func lineOf(line *modfile.Line) int {
	if line == nil {
		return 0
	}
	return line.Start.Line
}

// identitiesForPath returns the identity refs a module path contributes. It is
// always the normalized module path itself; for a path deeper than its owning
// repo it also yields the owning-repo identity (decision 0002, "subdir module →
// owning repo"). The slice is deterministic and never contains duplicates.
func identitiesForPath(path string) []string {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil
	}
	refs := []string{path}
	if owner := owningRepo(path); owner != "" && owner != path {
		refs = append(refs, owner)
	}
	return refs
}

// owningRepo derives the host/owner/repo identity from a module path for the
// known forge hosts (decision 0002: "first three path segments" for
// github.com/gitlab.com-style hosts). It returns "" when the host is not one of
// these or the path is too shallow to carry owner/repo — in which case the
// module path is its own owning repo and no second identity is emitted.
func owningRepo(path string) string {
	segs := strings.Split(path, "/")
	if len(segs) < 3 {
		return ""
	}
	switch segs[0] {
	case "github.com", "gitlab.com", "bitbucket.org":
		return strings.Join(segs[:3], "/")
	default:
		return ""
	}
}

// commentedReplace is a replace directive recovered from comment text.
type commentedReplace struct {
	oldPath string
	line    int
}

// commentedReplaces scans every comment in the file for text that parses as a
// `replace OLD [version] => NEW [version]` directive and returns the left-hand
// module path of each, with the comment's source line. Order follows source
// position so extraction stays deterministic.
func commentedReplaces(f *modfile.FileSyntax) []commentedReplace {
	if f == nil {
		return nil
	}
	var out []commentedReplace
	for _, stmt := range f.Stmt {
		for _, c := range allComments(stmt) {
			if old, ok := parseCommentedReplace(c.Token); ok {
				out = append(out, commentedReplace{oldPath: old, line: c.Start.Line})
			}
		}
	}
	return out
}

// allComments returns every comment attached to an expression: standalone
// blocks land in Before, trailing notes in Suffix, and top-level trailing
// comments in After.
func allComments(e modfile.Expr) []modfile.Comment {
	c := e.Comment()
	if c == nil {
		return nil
	}
	out := make([]modfile.Comment, 0, len(c.Before)+len(c.Suffix)+len(c.After))
	out = append(out, c.Before...)
	out = append(out, c.Suffix...)
	out = append(out, c.After...)
	return out
}

// parseCommentedReplace pulls the left-hand module path out of a comment whose
// text is a commented replace directive. It accepts the forms
// `// replace OLD => NEW` and `// replace OLD vX => NEW vY`, returning false for
// any comment that is not a replace directive.
func parseCommentedReplace(token string) (string, bool) {
	text := strings.TrimSpace(strings.TrimLeft(token, "/"))
	rest, ok := strings.CutPrefix(text, "replace ")
	if !ok {
		return "", false
	}
	lhs, _, ok := strings.Cut(rest, "=>")
	if !ok {
		return "", false
	}
	// The left side is `OLD` or `OLD version`; the module path is the first
	// field.
	fields := strings.Fields(lhs)
	if len(fields) == 0 {
		return "", false
	}
	return fields[0], true
}
