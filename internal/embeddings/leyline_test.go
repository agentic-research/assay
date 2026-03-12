package embeddings

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseSearchOutput_Normal(t *testing.T) {
	data := []byte("0.001234\tdocs/readme\n0.045678\tdocs/notes\n0.089012\ttools/search\n")
	results, err := parseSearchOutput(data)
	require.NoError(t, err)
	require.Len(t, results, 3)

	assert.Equal(t, "docs/readme", results[0].NodeID)
	assert.InDelta(t, 0.001234, results[0].Distance, 0.000001)

	assert.Equal(t, "tools/search", results[2].NodeID)
}

func TestParseSearchOutput_NoResults(t *testing.T) {
	data := []byte("(no results)\n")
	results, err := parseSearchOutput(data)
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestParseSearchOutput_Empty(t *testing.T) {
	results, err := parseSearchOutput([]byte(""))
	require.NoError(t, err)
	assert.Empty(t, results)
}
