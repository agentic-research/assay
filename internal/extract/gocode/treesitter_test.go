package gocode

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/agentic-research/assay/internal/artifact"
)

// writeGoTree lays down a minimal module rooted at dir: a go.mod declaring
// module path mod, plus the given files (relative path → contents).
func writeGoTree(t *testing.T, dir, mod string, files map[string]string) {
	t.Helper()
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "go.mod"),
		[]byte("module "+mod+"\n\ngo 1.26\n"),
		0o644,
	))
	for rel, body := range files {
		full := filepath.Join(dir, rel)
		require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
		require.NoError(t, os.WriteFile(full, []byte(body), 0o644))
	}
}

func TestTreeSitterBackend_EmitsSymbolProducersAndImportConsumers(t *testing.T) {
	dir := t.TempDir()
	writeGoTree(t, dir, "example.com/proj", map[string]string{
		"foo/foo.go": `package foo

import (
	"fmt"

	"example.com/proj/bar"
)

// Greet builds a greeting.
func Greet(name string) string {
	bar.Helper()
	return fmt.Sprintf("hi %s", name)
}

type Config struct{ N int }
`,
		"bar/bar.go": `package bar

// Helper is a helper.
func Helper() {}
`,
	})

	b := newTreeSitterBackend(false)
	producers, consumers, err := b.extract(dir)
	require.NoError(t, err)

	// Symbol producers carry the full import-path-qualified identity.
	prodRefs := refSet(t, producers, artifact.KindGoPackageSymbol)
	assert.Contains(t, prodRefs, "example.com/proj/foo.Greet")
	assert.Contains(t, prodRefs, "example.com/proj/foo.Config")
	assert.Contains(t, prodRefs, "example.com/proj/bar.Helper")

	// Import consumers carry the imported package path as a go_module ref.
	consRefs := refSet(t, consumers, artifact.KindGoModule)
	assert.Contains(t, consRefs, "fmt")
	assert.Contains(t, consRefs, "example.com/proj/bar")

	// Provenance is attached: every producer has a file, every import a line.
	for _, p := range producers {
		assert.NotEmpty(t, p.Provenance.File, "producer %s missing file", p.Identity.Ref)
	}
	for _, c := range consumers {
		assert.NotZero(t, c.Provenance.Line, "consumer %s missing line", c.Identity.Ref)
	}
}

func TestTreeSitterBackend_Deterministic(t *testing.T) {
	dir := t.TempDir()
	writeGoTree(t, dir, "example.com/proj", map[string]string{
		"a/a.go": "package a\n\nfunc A() {}\nfunc B() {}\n",
		"b/b.go": "package b\n\nimport \"example.com/proj/a\"\n\nfunc C() { a.A() }\n",
	})

	b := newTreeSitterBackend(false)
	p1, c1, err := b.extract(dir)
	require.NoError(t, err)
	p2, c2, err := b.extract(dir)
	require.NoError(t, err)
	assert.Equal(t, p1, p2, "producers must be deterministic")
	assert.Equal(t, c1, c2, "consumers must be deterministic")
}

func TestTreeSitterBackend_ExportedOnly(t *testing.T) {
	dir := t.TempDir()
	writeGoTree(t, dir, "example.com/proj", map[string]string{
		"a/a.go": "package a\n\nfunc Exported() {}\nfunc unexported() {}\n",
	})

	b := newTreeSitterBackend(true)
	producers, _, err := b.extract(dir)
	require.NoError(t, err)
	prodRefs := refSet(t, producers, artifact.KindGoPackageSymbol)
	assert.Contains(t, prodRefs, "example.com/proj/a.Exported")
	assert.NotContains(t, prodRefs, "example.com/proj/a.unexported")
}

// refSet collects the Ref strings of producers of the given kind.
func refSet[T interface {
	Kind() artifact.Kind
}](t *testing.T, items []T, kind artifact.Kind) map[string]bool {
	t.Helper()
	out := map[string]bool{}
	for _, it := range items {
		if it.Kind() != kind {
			continue
		}
		switch v := any(it).(type) {
		case artifact.Producer:
			out[v.Identity.Ref] = true
		case artifact.Consumer:
			out[v.Identity.Ref] = true
		}
	}
	return out
}
