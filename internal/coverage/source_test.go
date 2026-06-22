package coverage

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubSource is a minimal ClaimSource for contract tests.
type stubSource struct {
	refs []DocRef
	err  error
}

func (s stubSource) Claims() ([]DocRef, error) { return s.refs, s.err }

// Assert the stub satisfies the interface at compile time.
var _ ClaimSource = stubSource{}

func TestComputeFromSources_SingleSource(t *testing.T) {
	entities := []Entity{
		{Name: "Foo", Kind: "function", Package: "pkg"},
		{Name: "Bar", Kind: "type", Package: "pkg"},
	}
	src := stubSource{refs: []DocRef{
		{Text: "Foo", Kind: "code_span"},
		{Text: "Bar", Kind: "code_span"},
	}}

	result, err := ComputeFromSources(entities, DefaultFuzzyThreshold, src)
	require.NoError(t, err)

	assert.Equal(t, 1.0, result.Coverage)
	assert.Len(t, result.Covered, 2)
}

func TestComputeFromSources_MergesMultipleSources(t *testing.T) {
	entities := []Entity{
		{Name: "Foo", Kind: "function", Package: "pkg"},
		{Name: "Bar", Kind: "type", Package: "pkg"},
	}
	a := stubSource{refs: []DocRef{{Text: "Foo", Kind: "code_span"}}}
	b := stubSource{refs: []DocRef{{Text: "Bar", Kind: "code_span"}}}

	result, err := ComputeFromSources(entities, DefaultFuzzyThreshold, a, b)
	require.NoError(t, err)

	assert.Equal(t, 1.0, result.Coverage)
	assert.Len(t, result.DocRefs, 2)
}

func TestComputeFromSources_PropagatesError(t *testing.T) {
	want := errors.New("boom")
	src := stubSource{err: want}

	_, err := ComputeFromSources(nil, DefaultFuzzyThreshold, src)
	require.Error(t, err)
	assert.ErrorIs(t, err, want)
}
