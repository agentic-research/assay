package coverage

import (
	"fmt"
	"sort"
	"strings"
)

// maxNamedEntities bounds how many offending entity names a gate failure
// prints. The full list is available from the report itself; the error is a
// summary, and an unbounded one buries the CI log.
const maxNamedEntities = 10

// GateOptions configures Gate. The zero value disables every check, so adding
// a gate to an existing caller is opt-in.
type GateOptions struct {
	// MaxUncovered fails when more than N exported entities have no
	// documentation reference. Zero means "no entity may be undocumented";
	// use a negative value to disable. Because the zero value must disable
	// the whole gate, callers signal "budget of 0" via MaxUncoveredSet.
	MaxUncovered int

	// MaxUncoveredSet distinguishes an explicit budget of 0 from the unset
	// zero value. Set it when MaxUncovered came from a user-supplied flag.
	MaxUncoveredSet bool

	// MinCoverage fails when the covered ratio drops below this floor.
	// Zero disables the check.
	MinCoverage float64
}

// Gate applies the configured thresholds to a computed result and returns a
// non-nil error describing every violation, or nil when the result passes.
//
// It deliberately gates only the COVERAGE direction — exported entities that
// no documentation mentions. The reverse direction (CoverageResult.Stale: doc
// references matching no entity) is computed but NOT gated, because it is not
// yet precise enough to gate on: every backtick-fenced span in markdown is
// treated as a claimed code symbol, so filenames, CLI flags, table names, bead
// IDs and other projects' types all register as "stale". Measured on assay's
// own tree, staleness is 92.6% (8,355 distinct spans, of which only 861 are
// even Go-identifier-shaped). Gating that would fail permanently while
// signalling nothing.
//
// Making the stale direction gateable needs a preprocessor that decides which
// backtick spans actually assert a code symbol. mache's
// drift_doc_dead_symbol_reference rule is blocked on exactly the same missing
// piece and ships as a no-op (`WHERE 1=0`) for the same reason.
//
// The coverage direction has no such problem: entities come from the parsed
// source, not from prose, so every finding names a real exported symbol.
func Gate(result *CoverageResult, opts GateOptions) error {
	if result == nil {
		return nil
	}
	// With nothing to document, both checks would fire on a repo that simply
	// has no exported entities yet. That is a false positive, not drift.
	if len(result.Entities) == 0 {
		return nil
	}

	var violations []string

	if opts.MaxUncoveredSet && opts.MaxUncovered >= 0 {
		if n := len(result.Uncovered); n > opts.MaxUncovered {
			violations = append(violations,
				fmt.Sprintf("%d uncovered %s (budget %d): %s",
					n, plural(n, "entity", "entities"), opts.MaxUncovered,
					namesOf(result.Uncovered)))
		}
	}

	if opts.MinCoverage > 0 && result.Coverage < opts.MinCoverage {
		violations = append(violations,
			fmt.Sprintf("coverage %.4f below floor %.4f", result.Coverage, opts.MinCoverage))
	}

	if len(violations) == 0 {
		return nil
	}
	return fmt.Errorf("doc gate failed: %s", strings.Join(violations, "; "))
}

// namesOf renders a stable, bounded, package-qualified list of entity names.
func namesOf(entities []Entity) string {
	names := make([]string, 0, len(entities))
	for _, e := range entities {
		if e.Package != "" {
			names = append(names, e.Package+"."+e.Name)
		} else {
			names = append(names, e.Name)
		}
	}
	sort.Strings(names)

	if len(names) > maxNamedEntities {
		omitted := len(names) - maxNamedEntities
		names = names[:maxNamedEntities]
		return strings.Join(names, ", ") + fmt.Sprintf(" (+%d more)", omitted)
	}
	return strings.Join(names, ", ")
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}
