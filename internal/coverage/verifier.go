package coverage

// StructuralVerifier yields the entities a codebase actually has — the
// "does the code really contain this?" side of coverage, independent of
// how that ground truth is derived.
//
// A claim (DocRef) asserts that some construct exists; a verifier supplies
// the set of constructs that genuinely do, so claims can be checked against
// structure rather than against a doc author's word. Tree-sitter entity
// extraction is the first, default implementation. A mache-backed verifier
// (canonical v_defs/v_refs AST views + the smell-rule engine) is a second
// implementation that checks the same claims against producer-agnostic
// structural rules; both are selectable behind this one interface.
//
// This mirrors ClaimSource on the docs side: ClaimSource is "what the docs
// claim", StructuralVerifier is "what the code has". ComputeFromVerifier
// joins the two.
type StructuralVerifier interface {
	// Name identifies the backend ("tree-sitter", "mache") for reporting.
	Name() string

	// Available reports whether this backend can run in the current
	// environment. A backend whose external dependency (a built .db, a
	// helper binary on PATH) is missing returns false rather than faking
	// success, so callers can fall back to an available backend.
	Available() bool

	// Entities returns the documentable constructs the code actually has.
	Entities() ([]Entity, error)
}

// ComputeFromVerifier derives the code's entity set from a StructuralVerifier
// and computes coverage against the given claim-references. It is the verifier
// analogue of ComputeFromSources: where that abstracts "what the docs claim",
// this abstracts "what the code has", so either side of the coverage join can
// vary independently behind its interface.
func ComputeFromVerifier(v StructuralVerifier, refs []DocRef, fuzzyThreshold float64) (*CoverageResult, error) {
	entities, err := v.Entities()
	if err != nil {
		return nil, err
	}
	return ComputeWithThreshold(entities, refs, fuzzyThreshold), nil
}
