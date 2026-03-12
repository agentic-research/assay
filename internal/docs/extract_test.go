package docs

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/agentic-research/assay/internal/coverage"
)

func TestExtractSource_CodeSpans(t *testing.T) {
	src := []byte("# API Reference\n\nThe `GraphCache` type provides caching. Call `NewGraphCache()` to create one.\n")

	refs, err := ExtractSource(src, "api.md")
	require.NoError(t, err)

	texts := refTexts(refs)
	assert.Contains(t, texts, "GraphCache")
	assert.Contains(t, texts, "NewGraphCache()")
}

func TestExtractSource_HeadingIdentifiers(t *testing.T) {
	src := []byte("# GraphCache\n\nSome text about the cache.\n\n## MemoryStore\n\nMore text.\n")

	refs, err := ExtractSource(src, "arch.md")
	require.NoError(t, err)

	texts := refTexts(refs)
	assert.Contains(t, texts, "GraphCache")
	assert.Contains(t, texts, "MemoryStore")
}

func TestExtractSource_FiltersNonIdentifiers(t *testing.T) {
	src := []byte("Use `--verbose` flag and `go test ./...` to run.\n\nSee `~/config/file`.\n")

	refs, err := ExtractSource(src, "guide.md")
	require.NoError(t, err)

	texts := refTexts(refs)
	assert.NotContains(t, texts, "--verbose")
	assert.NotContains(t, texts, "go test ./...")
	assert.NotContains(t, texts, "~/config/file")
}

func TestExtractSource_QualifiedNames(t *testing.T) {
	src := []byte("Call `graph.NewStore` to create a store.\n")

	refs, err := ExtractSource(src, "api.md")
	require.NoError(t, err)

	require.Len(t, refs, 1)
	assert.Equal(t, "graph.NewStore", refs[0].Text)
	assert.Equal(t, "code_span", refs[0].Kind)
}

func TestExtractSource_EmptyDoc(t *testing.T) {
	refs, err := ExtractSource([]byte(""), "empty.md")
	require.NoError(t, err)
	assert.Empty(t, refs)
}

func refTexts(refs []coverage.DocRef) []string {
	var texts []string
	for _, r := range refs {
		texts = append(texts, r.Text)
	}
	return texts
}
