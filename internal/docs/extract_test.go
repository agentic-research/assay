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

// TestExtractSource_TableCells pins the pipe-table gap: tree-sitter-markdown
// emits pipe_table_cell as a BLOCK node with no inline subtree attached, so a
// walk that only descends into node.Inline never sees a code span inside a
// table. assay's own ARCHITECTURE.md documents nearly every entity in tables,
// so this silently made those mentions invisible to the coverage gate.
func TestExtractSource_TableCells(t *testing.T) {
	src := []byte(`# Doc

| Symbol | Notes |
|--------|-------|
| ` + "`TableOnlySymbol`" + ` | documented only in a table |

Prose mentions ` + "`ProseSymbol`" + `.
`)

	refs, err := ExtractSource(src, "doc.md")
	require.NoError(t, err)

	texts := make(map[string]bool, len(refs))
	for _, r := range refs {
		texts[r.Text] = true
	}

	assert.True(t, texts["ProseSymbol"], "prose code span must be extracted (control)")
	assert.True(t, texts["TableOnlySymbol"], "table-cell code span must be extracted")
}

// TestExtractSource_TableCellLineNumbers asserts a table-cell ref reports the
// line it actually sits on, not the line the table starts on — a finding that
// points at the wrong line is a finding a reader cannot act on.
func TestExtractSource_TableCellLineNumbers(t *testing.T) {
	src := []byte("intro\n\n| A | B |\n|---|---|\n| `RowOne` | x |\n| `RowTwo` | y |\n")

	refs, err := ExtractSource(src, "doc.md")
	require.NoError(t, err)

	lines := map[string]int{}
	for _, r := range refs {
		lines[r.Text] = r.Line
	}

	assert.Equal(t, 5, lines["RowOne"], "RowOne sits on line 5")
	assert.Equal(t, 6, lines["RowTwo"], "RowTwo sits on line 6")
}
