package gocode

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/agentic-research/assay/internal/extract"
)

// Compile-time assertion: the Go-code extractor satisfies the Extractor contract.
var _ extract.Extractor = (*Extractor)(nil)

func TestExtractor_Name(t *testing.T) {
	assert.Equal(t, "gocode", New().Name())
}

func TestExtractor_AlwaysAvailable_TreeSitterFloor(t *testing.T) {
	// With no .db configured the extractor still runs: the tree-sitter floor
	// is always available, so Available() is true and names the active backend.
	ex := New()
	ok, reason := ex.Available()
	assert.True(t, ok)
	assert.Contains(t, reason, "tree-sitter")
}

func TestExtractor_TreeSitterBackend_ProducesFacts(t *testing.T) {
	dir := t.TempDir()
	writeGoTree(t, dir, "example.com/proj", map[string]string{
		"a/a.go": "package a\n\nimport \"fmt\"\n\n// A does a.\nfunc A() { fmt.Println() }\n",
	})

	ex := New()
	producers, consumers, err := ex.Extract(dir)
	require.NoError(t, err)
	assert.NotEmpty(t, producers)
	assert.NotEmpty(t, consumers)
}

func TestExtractor_MacheRequestedButUnavailable_ReportsReason(t *testing.T) {
	// When the mache backend is explicitly requested (a .db path is set) but
	// the .db does not exist, Available() reports a clear reason and does NOT
	// silently fall through to an empty success.
	ex := New(WithMacheDB("/nonexistent/path/to.db"))
	ok, reason := ex.Available()
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
	assert.Contains(t, reason, "mache")
}

func TestExtractor_ActiveBackend_ReflectsSelection(t *testing.T) {
	assert.Equal(t, "tree-sitter", New().ActiveBackend())
}
