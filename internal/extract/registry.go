package extract

import "github.com/agentic-research/assay/internal/artifact"

// Registry holds an ordered set of extractors and runs them over scan roots.
type Registry struct {
	extractors []Extractor
}

// NewRegistry returns a Registry over the given extractors, preserving their
// order. Registration order is the gather order, which keeps Gather's output
// deterministic.
func NewRegistry(extractors ...Extractor) *Registry {
	return &Registry{extractors: extractors}
}

// Skipped records an extractor the Registry did not run because it reported
// itself unavailable, along with the reason it gave. A skip is not an error:
// callers see what evidence was deliberately left ungathered.
type Skipped struct {
	Name   string
	Reason string
}

// Facts is the merged evidence gathered from one Gather pass: every Producer and
// Consumer fact emitted across all available (extractor, root) combinations,
// plus the extractors that were skipped and why.
//
// Facts are evidence only — no edges. Matching Consumers to Producers is the
// resolver's job.
type Facts struct {
	Producers []artifact.Producer
	Consumers []artifact.Consumer
	Skipped   []Skipped
}

// Gather runs every available extractor over every root, in registration ×
// argument order, and merges all emitted facts into a single Facts value.
//
// This generalizes the claim-gathering shape (coverage.ComputeFromSources) from
// claims to evidence: it consumes the Extractor interface rather than any
// concrete extractor, propagates the first extraction error encountered, and
// records — rather than errors on — extractors that report themselves
// unavailable.
func (r *Registry) Gather(roots ...string) (*Facts, error) {
	facts := &Facts{}
	for _, ex := range r.extractors {
		if ok, reason := ex.Available(); !ok {
			facts.Skipped = append(facts.Skipped, Skipped{Name: ex.Name(), Reason: reason})
			continue
		}
		for _, root := range roots {
			producers, consumers, err := ex.Extract(root)
			if err != nil {
				return nil, err
			}
			facts.Producers = append(facts.Producers, producers...)
			facts.Consumers = append(facts.Consumers, consumers...)
		}
	}
	return facts, nil
}
