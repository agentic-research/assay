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

// Failed records a single (extractor, root) pass that returned an error — for
// example a malformed go.mod in one root. Unlike a Skipped extractor (which
// reported itself unavailable before running), a Failed pass tried to extract
// and could not. It is non-fatal by design: one unparseable input in one root
// must not abort a cross-root scan, so the failure is recorded here and the
// gather continues with the remaining extractors and roots.
type Failed struct {
	Extractor string
	Root      string
	Err       error
}

// Facts is the merged evidence gathered from one Gather pass: every Producer and
// Consumer fact emitted across all available (extractor, root) combinations,
// the extractors that were skipped and why, and the per-(extractor, root) passes
// that failed.
//
// Facts are evidence only — no edges. Matching Consumers to Producers is the
// resolver's job.
type Facts struct {
	Producers []artifact.Producer
	Consumers []artifact.Consumer
	Skipped   []Skipped
	Failed    []Failed
}

// Gather runs every available extractor over every root, in registration ×
// argument order, and merges all emitted facts into a single Facts value.
//
// This generalizes the claim-gathering shape (coverage.ComputeFromSources) from
// claims to evidence: it consumes the Extractor interface rather than any
// concrete extractor, records — rather than errors on — extractors that report
// themselves unavailable, and isolates per-(extractor, root) extraction
// failures so one bad input cannot abort the whole scan.
//
// Failure isolation is deliberate. An Extract error here is treated as an
// input-parse failure (e.g. a malformed go.mod, an unreadable file): it is
// recorded in Failed and the gather continues, so a single unparseable input in
// one root never kills a cross-root analysis. Gather itself never returns a
// non-nil error today; the error in its signature is reserved for a future
// genuine programming fault that would make the whole pass unsound. Extractors
// must therefore return input/parse problems as errors from Extract (which
// become soft Failed records) and must not use them to signal real bugs.
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
				// Non-fatal: record the failure and keep gathering. Order is
				// registration × argument order, so Failed stays deterministic.
				facts.Failed = append(facts.Failed, Failed{
					Extractor: ex.Name(),
					Root:      root,
					Err:       err,
				})
				continue
			}
			facts.Producers = append(facts.Producers, producers...)
			facts.Consumers = append(facts.Consumers, consumers...)
		}
	}
	return facts, nil
}
