package gocode

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "modernc.org/sqlite"

	"github.com/agentic-research/assay/internal/artifact"
)

// buildFixtureDB writes a minimal mache-shaped .db carrying the base tables the
// canonical views read from (node_defs / node_refs) plus nodes/_ast for
// provenance. It deliberately omits the v_defs/v_refs views so the backend must
// install them on open (the EnsureCanonicalViews tolerance path).
func buildFixtureDB(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "fixture.db")
	db, err := sql.Open("sqlite", path)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	stmts := []string{
		`CREATE TABLE nodes (
			id TEXT PRIMARY KEY,
			name TEXT,
			kind TEXT,
			parent_id TEXT,
			record TEXT,
			source_file TEXT
		)`,
		`CREATE TABLE _ast (
			node_id TEXT,
			source_id TEXT,
			start_byte INTEGER,
			end_byte INTEGER,
			start_row INTEGER,
			start_col INTEGER
		)`,
		`CREATE TABLE node_defs (token TEXT, node_id TEXT)`,
		`CREATE TABLE node_refs (node_id TEXT, token TEXT)`,

		// A defined symbol: example.com/proj/foo.Greet at foo/foo.go:9.
		`INSERT INTO nodes (id, name, kind, source_file)
		 VALUES ('foo/func/Greet', 'Greet', 'function', 'foo/foo.go')`,
		`INSERT INTO _ast (node_id, source_id, start_row) VALUES ('foo/func/Greet', 'foo/foo.go', 8)`,
		`INSERT INTO node_defs (token, node_id) VALUES ('example.com/proj/foo.Greet', 'foo/func/Greet')`,

		// A reference: foo/foo.go references bar.Helper at line 11.
		`INSERT INTO nodes (id, name, kind, source_file)
		 VALUES ('foo/call/Helper', 'Helper', 'call', 'foo/foo.go')`,
		`INSERT INTO _ast (node_id, source_id, start_row) VALUES ('foo/call/Helper', 'foo/foo.go', 10)`,
		`INSERT INTO node_refs (node_id, token) VALUES ('foo/call/Helper', 'example.com/proj/bar.Helper')`,
	}
	for _, s := range stmts {
		_, err := db.Exec(s)
		require.NoError(t, err, "stmt: %s", s)
	}
	return path
}

func TestMacheBackend_ReadsDefsAndRefs(t *testing.T) {
	dir := t.TempDir()
	dbPath := buildFixtureDB(t, dir)

	b := newMacheBackend(dbPath)
	ok, reason := b.available()
	require.True(t, ok, "fixture .db should be available: %s", reason)

	producers, consumers, err := b.extract(dir)
	require.NoError(t, err)

	// Producer from v_defs, with file:line provenance from the nodes/_ast join.
	require.Len(t, producers, 1)
	p := producers[0]
	assert.Equal(t, artifact.KindGoPackageSymbol, p.Kind())
	assert.Equal(t, "example.com/proj/foo.Greet", p.Identity.Ref)
	assert.Equal(t, "foo/foo.go", p.Provenance.File)
	assert.Equal(t, 9, p.Provenance.Line) // start_row 8 (0-based) → line 9

	// Consumer from v_refs, with provenance from the referrer node.
	require.Len(t, consumers, 1)
	c := consumers[0]
	assert.Equal(t, artifact.KindGoPackageSymbol, c.Kind())
	assert.Equal(t, "example.com/proj/bar.Helper", c.Identity.Ref)
	assert.Equal(t, "foo/foo.go", c.Provenance.File)
	assert.Equal(t, 11, c.Provenance.Line)
}

func TestMacheBackend_NoDB_ReportsClearReason(t *testing.T) {
	b := newMacheBackend("/nonexistent/path/to.db")
	ok, reason := b.available()
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestMacheBackend_EmptyPath_Unavailable(t *testing.T) {
	b := newMacheBackend("")
	ok, reason := b.available()
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestExtractor_PrefersMacheWhenDBPresent(t *testing.T) {
	dir := t.TempDir()
	dbPath := buildFixtureDB(t, dir)

	ex := New(WithMacheDB(dbPath))
	ok, _ := ex.Available()
	require.True(t, ok)
	assert.Equal(t, "mache", ex.ActiveBackend())

	producers, consumers, err := ex.Extract(dir)
	require.NoError(t, err)
	assert.NotEmpty(t, producers)
	assert.NotEmpty(t, consumers)
}
