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

	// FailOnDeadRefs fails when any doc reference names an entity of this
	// codebase that no longer exists (CoverageResult.Dead). Unlike the
	// coverage knobs it has no budget: a dead reference is a doc that is
	// wrong today, not debt to ratchet down.
	FailOnDeadRefs bool
}

// Gate applies the configured thresholds to a computed result and returns a
// non-nil error describing every violation, or nil when the result passes.
//
// It gates both directions of drift:
//
//   - COVERAGE: exported entities that no documentation mentions
//     (MaxUncovered / MinCoverage). Entities come from the parsed source, not
//     from prose, so every finding names a real exported symbol.
//   - DEAD REFERENCES: documentation naming one of this codebase's entities
//     that no longer exists (FailOnDeadRefs).
//
// The dead-reference check does NOT gate CoverageResult.Stale. Stale treats
// every backtick span as a claimed symbol, so filenames, CLI flags, bead IDs
// and other projects' types all register (92.6% staleness on assay's own
// tree); gating it would fail permanently while signalling nothing. Dead is
// the subset whose own shape asserts "this repo's symbol" — package- or
// receiver-qualified, resolved exactly — see DeadRefs for the rule and its
// blind spots.
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

	if opts.FailOnDeadRefs {
		if n := len(result.Dead); n > 0 {
			violations = append(violations,
				fmt.Sprintf("%d dead doc %s (names an entity the code no longer has): %s",
					n, plural(n, "reference", "references"), refsOf(result.Dead)))
		}
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

// refsOf renders a stable, bounded list of doc references with their location,
// so a failure says which doc to fix as well as which name is gone.
func refsOf(refs []DocRef) string {
	items := make([]string, 0, len(refs))
	for _, r := range refs {
		items = append(items, fmt.Sprintf("`%s` (%s:%d)", r.Text, r.File, r.Line))
	}
	sort.Strings(items)

	if len(items) > maxNamedEntities {
		omitted := len(items) - maxNamedEntities
		items = items[:maxNamedEntities]
		return strings.Join(items, ", ") + fmt.Sprintf(" (+%d more)", omitted)
	}
	return strings.Join(items, ", ")
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}
