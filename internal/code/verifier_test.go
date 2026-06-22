package code

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/agentic-research/assay/internal/coverage"
)

func TestTreeSitterVerifier_ImplementsInterface(t *testing.T) {
	var v coverage.StructuralVerifier = NewTreeSitterVerifier(".", true)
	assert.Equal(t, "tree-sitter", v.Name())
	assert.True(t, v.Available(), "tree-sitter backend is compiled in and always available")
}

func TestTreeSitterVerifier_EntitiesMatchExtractDir(t *testing.T) {
	dir := t.TempDir()
	src := `package example

// Exported is a documented function.
func Exported() {}

func unexported() {}
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "x.go"), []byte(src), 0o600))

	v := NewTreeSitterVerifier(dir, true)
	got, err := v.Entities()
	require.NoError(t, err)

	// Must equal the existing ExtractDir behavior it wraps.
	want, err := ExtractDir(dir, true)
	require.NoError(t, err)
	assert.Equal(t, want, got)

	names := entityNames(got)
	assert.Contains(t, names, "Exported")
	assert.NotContains(t, names, "unexported")
}
