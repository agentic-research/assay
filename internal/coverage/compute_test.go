package coverage

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCompute_FullCoverage(t *testing.T) {
	entities := []Entity{
		{Name: "Foo", Kind: "function", Package: "pkg"},
		{Name: "Bar", Kind: "type", Package: "pkg"},
	}
	refs := []DocRef{
		{Text: "Foo", Kind: "code_span"},
		{Text: "Bar", Kind: "code_span"},
	}

	result := Compute(entities, refs)

	assert.Equal(t, 1.0, result.Coverage)
	assert.Equal(t, 0.0, result.Staleness)
	assert.Len(t, result.Covered, 2)
	assert.Empty(t, result.Uncovered)
	assert.Empty(t, result.Stale)
}

func TestCompute_PartialCoverage(t *testing.T) {
	entities := []Entity{
		{Name: "Foo", Kind: "function", Package: "pkg"},
		{Name: "Bar", Kind: "type", Package: "pkg"},
		{Name: "Baz", Kind: "constant", Package: "pkg"},
	}
	refs := []DocRef{
		{Text: "Foo", Kind: "code_span"},
	}

	result := Compute(entities, refs)

	assert.InDelta(t, 1.0/3.0, result.Coverage, 0.01)
	assert.Len(t, result.Covered, 1)
	assert.Len(t, result.Uncovered, 2)
}

func TestCompute_StaleRefs(t *testing.T) {
	entities := []Entity{
		{Name: "Foo", Kind: "function", Package: "pkg"},
	}
	refs := []DocRef{
		{Text: "Foo", Kind: "code_span"},
		{Text: "OldFunc", Kind: "code_span", File: "api.md", Line: 42},
	}

	result := Compute(entities, refs)

	assert.Equal(t, 1.0, result.Coverage)
	assert.InDelta(t, 0.5, result.Staleness, 0.01)
	assert.Len(t, result.Stale, 1)
	assert.Equal(t, "OldFunc", result.Stale[0].Text)
}

func TestCompute_QualifiedMatch(t *testing.T) {
	entities := []Entity{
		{Name: "NewStore", Kind: "function", Package: "graph"},
	}
	refs := []DocRef{
		{Text: "graph.NewStore", Kind: "code_span"},
	}

	result := Compute(entities, refs)

	assert.Equal(t, 1.0, result.Coverage)
	assert.Len(t, result.Covered, 1)
}

func TestCompute_StripParens(t *testing.T) {
	entities := []Entity{
		{Name: "NewStore", Kind: "function", Package: "graph"},
	}
	refs := []DocRef{
		{Text: "NewStore()", Kind: "code_span"},
	}

	result := Compute(entities, refs)

	assert.Equal(t, 1.0, result.Coverage)
}

func TestCompute_Empty(t *testing.T) {
	result := Compute(nil, nil)

	assert.Equal(t, 0.0, result.Coverage)
	assert.Equal(t, 0.0, result.Staleness)
}

func TestCompute_FuzzyMatch(t *testing.T) {
	entities := []Entity{
		{Name: "MemoryStore", Kind: "type", Package: "graph"},
		{Name: "NewGraphCache", Kind: "function", Package: "graph"},
	}
	refs := []DocRef{
		{Text: "in-memory store", Kind: "code_span"},    // fuzzy match → MemoryStore
		{Text: "new graph cache", Kind: "code_span"},     // fuzzy match → NewGraphCache
		{Text: "completely unrelated", Kind: "code_span"}, // no match
	}

	result := Compute(entities, refs)

	assert.Equal(t, 1.0, result.Coverage) // both entities matched via fuzzy
	assert.Len(t, result.Covered, 2)
	assert.Len(t, result.Stale, 1) // "completely unrelated" is stale
}

func TestCompute_FuzzyDisabled(t *testing.T) {
	entities := []Entity{
		{Name: "MemoryStore", Kind: "type", Package: "graph"},
	}
	refs := []DocRef{
		{Text: "in-memory store", Kind: "code_span"},
	}

	// Exact only — no fuzzy.
	result := ComputeWithThreshold(entities, refs, 0)

	assert.Equal(t, 0.0, result.Coverage) // no exact match
	assert.Len(t, result.Stale, 1)
}

func TestCompute_FuzzyHighThreshold(t *testing.T) {
	entities := []Entity{
		{Name: "MemoryStore", Kind: "type", Package: "graph"},
	}
	refs := []DocRef{
		{Text: "store data values", Kind: "code_span"}, // weak overlap: only "store"
	}

	// High threshold — shouldn't match.
	result := ComputeWithThreshold(entities, refs, 0.8)

	assert.Equal(t, 0.0, result.Coverage)
	assert.Len(t, result.Stale, 1)
}
