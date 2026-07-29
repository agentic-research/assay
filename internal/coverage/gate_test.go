package coverage

import (
	"strings"
	"testing"
)

// entity/ref helpers keep the table cases below readable.
func ent(name string) Entity {
	return Entity{Name: name, Kind: "function", Package: "pkg", Exported: true}
}

func ref(text string) DocRef {
	return DocRef{Text: text, Kind: "code_span", File: "README.md", Line: 1}
}

// TestGate_FiresOnDrift_NotOnClean is the load-bearing pair. A gate that only
// ever passes is indistinguishable from no gate at all, so the clean case and
// the drifted case are asserted together: same options, opposite verdicts.
//
// The drift modelled here is the one the gate actually detects — an exported
// entity that no documentation mentions. (The reverse direction, a doc
// referencing a symbol that no longer exists, is NOT gated; see Gate's doc
// comment for why.)
func TestGate_FiresOnDrift_NotOnClean(t *testing.T) {
	opts := GateOptions{MaxUncovered: 0, MaxUncoveredSet: true}

	t.Run("clean: every entity documented", func(t *testing.T) {
		r := ComputeWithThreshold(
			[]Entity{ent("Alpha"), ent("Beta")},
			[]DocRef{ref("Alpha"), ref("Beta")},
			DefaultFuzzyThreshold,
		)
		if err := Gate(r, opts); err != nil {
			t.Fatalf("clean corpus must pass the gate, got: %v", err)
		}
	})

	t.Run("drifted: docs stopped mentioning Beta", func(t *testing.T) {
		r := ComputeWithThreshold(
			[]Entity{ent("Alpha"), ent("Beta")},
			[]DocRef{ref("Alpha")},
			DefaultFuzzyThreshold,
		)
		err := Gate(r, opts)
		if err == nil {
			t.Fatal("undocumented entity must fail the gate, got nil")
		}
		if !strings.Contains(err.Error(), "Beta") {
			t.Errorf("gate error must name the offending entity, got: %v", err)
		}
	})
}

// TestGate_RatchetBudget pins the adoption path: a repo with existing debt sets
// the budget at its current count and the gate holds the line without demanding
// the debt be paid first. One regression past the budget fails.
func TestGate_RatchetBudget(t *testing.T) {
	r := ComputeWithThreshold(
		[]Entity{ent("Alpha"), ent("Beta"), ent("Gamma")},
		[]DocRef{ref("Alpha")},
		DefaultFuzzyThreshold,
	)
	if got := len(r.Uncovered); got != 2 {
		t.Fatalf("fixture drifted: want 2 uncovered, got %d", got)
	}

	if err := Gate(r, GateOptions{MaxUncovered: 2, MaxUncoveredSet: true}); err != nil {
		t.Errorf("budget == actual must pass, got: %v", err)
	}
	if err := Gate(r, GateOptions{MaxUncovered: 1, MaxUncoveredSet: true}); err == nil {
		t.Error("budget < actual must fail")
	}
	if err := Gate(r, GateOptions{MaxUncovered: 3, MaxUncoveredSet: true}); err != nil {
		t.Errorf("budget > actual must pass, got: %v", err)
	}
}

// TestGate_DisabledByDefault guards the compatibility contract: the zero value
// of GateOptions must not turn `assay verify` into a failing command for every
// existing caller.
func TestGate_DisabledByDefault(t *testing.T) {
	r := ComputeWithThreshold(
		[]Entity{ent("Alpha"), ent("Beta")},
		nil,
		DefaultFuzzyThreshold,
	)
	if len(r.Uncovered) == 0 {
		t.Fatal("fixture drifted: expected uncovered entities")
	}
	if err := Gate(r, GateOptions{}); err != nil {
		t.Errorf("zero-value options must disable the gate, got: %v", err)
	}
}

// TestGate_MinCoverage covers the ratio form, which is what a caller wants when
// the entity count itself is expected to grow.
func TestGate_MinCoverage(t *testing.T) {
	r := ComputeWithThreshold(
		[]Entity{ent("Alpha"), ent("Beta"), ent("Gamma"), ent("Delta")},
		[]DocRef{ref("Alpha"), ref("Beta"), ref("Gamma")},
		DefaultFuzzyThreshold,
	)
	if r.Coverage < 0.74 || r.Coverage > 0.76 {
		t.Fatalf("fixture drifted: want coverage ~0.75, got %v", r.Coverage)
	}

	if err := Gate(r, GateOptions{MinCoverage: 0.75}); err != nil {
		t.Errorf("coverage == floor must pass, got: %v", err)
	}
	if err := Gate(r, GateOptions{MinCoverage: 0.80}); err == nil {
		t.Error("coverage < floor must fail")
	}
}

// TestGate_ReportsBothFailures ensures a caller setting both knobs learns about
// both violations from one run rather than fixing them one CI cycle at a time.
func TestGate_ReportsBothFailures(t *testing.T) {
	r := ComputeWithThreshold(
		[]Entity{ent("Alpha"), ent("Beta")},
		nil,
		DefaultFuzzyThreshold,
	)
	err := Gate(r, GateOptions{MaxUncovered: 0, MaxUncoveredSet: true, MinCoverage: 0.9})
	if err == nil {
		t.Fatal("expected failure")
	}
	msg := err.Error()
	if !strings.Contains(msg, "uncovered") || !strings.Contains(msg, "coverage") {
		t.Errorf("both violations must be reported, got: %v", msg)
	}
}

// TestGate_NoEntities pins the empty-input edge: with nothing to document,
// Compute reports coverage 0, and a MinCoverage floor would fail a repo that
// simply has no exported entities yet. That is a false positive, so it passes.
func TestGate_NoEntities(t *testing.T) {
	r := ComputeWithThreshold(nil, nil, DefaultFuzzyThreshold)
	if err := Gate(r, GateOptions{MaxUncovered: 0, MaxUncoveredSet: true, MinCoverage: 0.9}); err != nil {
		t.Errorf("no entities must not fail the gate, got: %v", err)
	}
}
