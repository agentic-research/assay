// Package extract defines the fact-emission contract that every artifact
// extractor implements, and a registry that runs extractors over scan roots to
// gather their facts.
//
// An extractor reads a single source format (a go.mod, a Dockerfile, a CI
// workflow, Go source) and emits typed Producer/Consumer facts WITH provenance.
// It never matches Consumers to Producers: edge resolution is a separate concern
// owned by the resolver. Extraction is observation; resolution is inference.
//
// This package imports only internal/artifact (the shared vocabulary). It knows
// nothing about coverage, code parsing, or any concrete extractor.
package extract

import "github.com/agentic-research/assay/internal/artifact"

// Extractor reads one source format from a scan root and emits the artifact
// facts it observes there.
//
// Implementations must be deterministic: the same root yields the same facts in
// the same order, so a Registry pass is reproducible.
type Extractor interface {
	// Name is a short, stable identifier for the extractor (e.g. "gomod",
	// "dockerfile"). It is used to attribute skipped extractors and facts.
	Name() string

	// Available reports whether the extractor can run in this environment,
	// honestly: when it cannot (a required binary is missing, an optional
	// dependency is absent), it returns false and a non-empty human-readable
	// reason. It must never fake availability — a false here causes the
	// Registry to skip the extractor and record the reason rather than emit
	// empty or fabricated facts.
	Available() (ok bool, reason string)

	// Extract observes the given scan root and emits the Producer and Consumer
	// facts found there, each carrying its Provenance. It MUST NOT match edges:
	// matching Consumers to Producers is the resolver's job.
	Extract(root string) (producers []artifact.Producer, consumers []artifact.Consumer, err error)
}
