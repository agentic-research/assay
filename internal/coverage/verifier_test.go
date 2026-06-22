package coverage

import "testing"

// staticVerifier is a trivial StructuralVerifier used to pin the contract:
// it returns a fixed entity set and reports availability unconditionally.
type staticVerifier struct {
	name     string
	entities []Entity
}

func (v staticVerifier) Name() string                { return v.name }
func (v staticVerifier) Available() bool             { return true }
func (v staticVerifier) Entities() ([]Entity, error) { return v.entities, nil }

func TestStructuralVerifier_ContractIsSatisfiable(t *testing.T) {
	want := []Entity{
		{Name: "Compute", Kind: "function", Package: "coverage"},
		{Name: "DocRef", Kind: "type", Package: "coverage"},
	}
	var v StructuralVerifier = staticVerifier{name: "static", entities: want}

	if v.Name() != "static" {
		t.Fatalf("Name() = %q, want %q", v.Name(), "static")
	}
	if !v.Available() {
		t.Fatal("Available() = false, want true")
	}
	got, err := v.Entities()
	if err != nil {
		t.Fatalf("Entities() error: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("Entities() returned %d entities, want %d", len(got), len(want))
	}
}

// TestComputeFromVerifier_ComposesWithClaimSource pins that a verifier's
// entity set flows into the existing coverage computation the same way a
// raw []Entity does, so the StructuralVerifier abstraction is a drop-in
// for the source of "what the code actually has".
func TestComputeFromVerifier_ComposesWithClaimSource(t *testing.T) {
	v := staticVerifier{
		name: "static",
		entities: []Entity{
			{Name: "Compute", Kind: "function"},
			{Name: "DocRef", Kind: "type"},
		},
	}
	refs := []DocRef{
		{Text: "Compute", Kind: "code_span", File: "README.md", Line: 1},
	}

	result, err := ComputeFromVerifier(v, refs, 0)
	if err != nil {
		t.Fatalf("ComputeFromVerifier error: %v", err)
	}
	if len(result.Covered) != 1 || result.Covered[0].Name != "Compute" {
		t.Fatalf("expected Compute covered, got %+v", result.Covered)
	}
	if len(result.Uncovered) != 1 || result.Uncovered[0].Name != "DocRef" {
		t.Fatalf("expected DocRef uncovered, got %+v", result.Uncovered)
	}
}
