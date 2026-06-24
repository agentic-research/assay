package ci

import (
	"sort"

	"github.com/agentic-research/assay/internal/artifact"
	"gopkg.in/yaml.v3"
)

// workflow accumulates facts found while walking one parsed workflow document.
type workflow struct {
	rel       string
	interp    *interpolator
	producers []artifact.Producer
	consumers []artifact.Consumer
}

// walk traverses the whole workflow tree. It treats two node shapes specially:
// every env: mapping (workflow-, job-, or step-level) is scanned for image-named
// values, and every run: scalar is scanned for image pulls and binary
// invocations. All other nodes are recursed into generically so the extractor
// makes no assumptions about job/step structure or about a particular build
// action being present.
func (w *workflow) walk(node *yaml.Node) {
	switch node.Kind {
	case yaml.MappingNode:
		for i := 0; i+1 < len(node.Content); i += 2 {
			key, val := node.Content[i], node.Content[i+1]
			switch key.Value {
			case "env":
				w.walkEnv(val)
			case "run":
				if val.Kind == yaml.ScalarNode {
					w.walkRun(val)
				}
			}
			w.walk(val)
		}
	case yaml.SequenceNode:
		for _, child := range node.Content {
			w.walk(child)
		}
	}
}

// walkEnv emits a container-image producer for each env: value that, after
// interpolation, looks like an image reference. The provenance line is the
// env entry's value line — where the image name is declared.
func (w *workflow) walkEnv(env *yaml.Node) {
	if env.Kind != yaml.MappingNode {
		return
	}
	for i := 0; i+1 < len(env.Content); i += 2 {
		val := env.Content[i+1]
		if val.Kind != yaml.ScalarNode {
			continue
		}
		resolved, ok := w.interp.resolve(val.Value)
		if !ok {
			continue // unresolvable interpolation — excluded per 0002
		}
		if ref, ok := imageIdentity(resolved); ok {
			w.producers = append(w.producers, artifact.Producer{
				Identity:   artifact.NewIdentity(artifact.KindContainerImage, ref),
				Provenance: artifact.Provenance{File: w.rel, Line: val.Line},
			})
		}
	}
}

// walkRun scans a run: script line by line for docker image pulls/pushes/builds
// (image producers/consumers) and for binary invocations (CLI consumers).
func (w *workflow) walkRun(run *yaml.Node) {
	base := run.Line // first content line of the (possibly block) scalar
	for offset, raw := range splitScriptLines(run.Value) {
		line := base + offset
		w.walkRunLine(raw, line)
	}
}

func (w *workflow) walkRunLine(raw string, line int) {
	fields := commandFields(raw)
	if len(fields) == 0 {
		return
	}

	if fields[0] == "docker" {
		if img, kind, ok := w.dockerImage(fields); ok {
			w.addImage(kind, img, line)
			return
		}
	}

	// Otherwise treat the leading token as a binary invocation. Skip shell
	// noise that is not a real program being exercised.
	if bin, ok := binaryName(fields[0]); ok {
		w.consumers = append(w.consumers, artifact.Consumer{
			Identity:   artifact.NewIdentity(artifact.KindCLIBinary, bin),
			Provenance: artifact.Provenance{File: w.rel, Line: line},
		})
	}
}

// dockerImage interprets a `docker <subcmd> … <image>` command, returning the
// canonical image identity and whether it is produced (build/push) or consumed
// (pull/run).
func (w *workflow) dockerImage(fields []string) (ref string, kind factKind, ok bool) {
	if len(fields) < 2 {
		return "", 0, false
	}
	var produces bool
	switch fields[1] {
	case "build", "push":
		produces = true
	case "pull", "run":
		produces = false
	default:
		return "", 0, false
	}

	for _, tok := range dockerImageArgs(fields[2:]) {
		resolved, ok := w.interp.resolve(tok)
		if !ok {
			continue
		}
		if id, ok := imageIdentity(resolved); ok {
			if produces {
				return id, factProducer, true
			}
			return id, factConsumer, true
		}
	}
	return "", 0, false
}

// factKind distinguishes a producer fact from a consumer fact for an image.
type factKind int

const (
	factProducer factKind = iota
	factConsumer
)

func (w *workflow) addImage(kind factKind, ref string, line int) {
	id := artifact.NewIdentity(artifact.KindContainerImage, ref)
	prov := artifact.Provenance{File: w.rel, Line: line}
	switch kind {
	case factProducer:
		w.producers = append(w.producers, artifact.Producer{Identity: id, Provenance: prov})
	case factConsumer:
		w.consumers = append(w.consumers, artifact.Consumer{Identity: id, Provenance: prov})
	}
}

// dedupe collapses repeated facts for the same identity to a single fact,
// keeping the earliest-occurring provenance, and returns them in a stable order.
// A workflow that names an image in env: and again in two docker commands should
// surface one producer, not three.
func (w *workflow) dedupe() ([]artifact.Producer, []artifact.Consumer, error) {
	return dedupeProducers(w.producers), dedupeConsumers(w.consumers), nil
}

func dedupeProducers(in []artifact.Producer) []artifact.Producer {
	seen := make(map[artifact.Identity]artifact.Producer, len(in))
	for _, p := range in {
		if prev, ok := seen[p.Identity]; !ok || earlier(p.Provenance, prev.Provenance) {
			seen[p.Identity] = p
		}
	}
	out := make([]artifact.Producer, 0, len(seen))
	for _, p := range seen {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool {
		return lessProv(out[i].Identity, out[i].Provenance, out[j].Identity, out[j].Provenance)
	})
	return out
}

func dedupeConsumers(in []artifact.Consumer) []artifact.Consumer {
	seen := make(map[artifact.Identity]artifact.Consumer, len(in))
	for _, c := range in {
		if prev, ok := seen[c.Identity]; !ok || earlier(c.Provenance, prev.Provenance) {
			seen[c.Identity] = c
		}
	}
	out := make([]artifact.Consumer, 0, len(seen))
	for _, c := range seen {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool {
		return lessProv(out[i].Identity, out[i].Provenance, out[j].Identity, out[j].Provenance)
	})
	return out
}

// earlier reports whether a was observed before b within the same file.
func earlier(a, b artifact.Provenance) bool {
	if a.File != b.File {
		return a.File < b.File
	}
	return a.Line < b.Line
}

// lessProv orders facts by identity then provenance for deterministic output.
func lessProv(idA artifact.Identity, a artifact.Provenance, idB artifact.Identity, b artifact.Provenance) bool {
	if idA.Kind != idB.Kind {
		return idA.Kind < idB.Kind
	}
	if idA.Ref != idB.Ref {
		return idA.Ref < idB.Ref
	}
	return earlier(a, b)
}
