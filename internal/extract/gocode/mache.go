package gocode

import (
	"database/sql"
	"fmt"
	"os"

	_ "modernc.org/sqlite" // pure-Go driver, registered as "sqlite" (no cgo)

	"github.com/agentic-research/assay/internal/artifact"
)

// canonicalViewsDDL installs mache's canonical consumer views over the base
// node_defs / node_refs tables. It mirrors ingest.CanonicalViewsDDL (mache
// ADR-0013) so assay can read a .db written by an older mache build that
// predates the views, per decision 0001. CREATE VIEW IF NOT EXISTS is
// idempotent and safe to re-run.
const canonicalViewsDDL = `
CREATE VIEW IF NOT EXISTS v_defs AS
	SELECT token, node_id, 'mention' AS fidelity FROM node_defs;

CREATE VIEW IF NOT EXISTS v_refs AS
	SELECT node_id AS referrer_node_id,
	       token,
	       NULL  AS target_node_id,
	       NULL  AS ref_uri,
	       NULL  AS ref_line,
	       'mention' AS fidelity
	FROM node_refs;
`

// macheBackend reads a mache .db read-only via the canonical v_defs/v_refs
// views (decision 0001) and emits package-symbol producers/consumers equivalent
// to the tree-sitter backend, but with mache's structural fidelity. mache need
// not be running; the .db is treated as a frozen input.
type macheBackend struct {
	dbPath string
}

func newMacheBackend(dbPath string) macheBackend {
	return macheBackend{dbPath: dbPath}
}

// available reports whether the mache backend can run, honestly: it requires a
// configured .db path that exists on disk. When the path is empty or missing it
// returns a clear reason rather than faking success — callers fall back to the
// tree-sitter floor.
func (b macheBackend) available() (bool, string) {
	if b.dbPath == "" {
		return false, "mache backend: no .db path configured"
	}
	if _, err := os.Stat(b.dbPath); err != nil {
		return false, fmt.Sprintf("mache backend: .db not readable at %s: %v", b.dbPath, err)
	}
	return true, ""
}

// open opens the .db read-only and installs the canonical views defensively so
// the read works against .db files written before the views existed.
func (b macheBackend) open() (*sql.DB, error) {
	db, err := sql.Open("sqlite", b.dbPath+"?mode=ro")
	if err != nil {
		return nil, fmt.Errorf("open mache .db %s: %w", b.dbPath, err)
	}
	if _, err := db.Exec(canonicalViewsDDL); err != nil {
		db.Close()
		return nil, fmt.Errorf("install canonical views in %s: %w", b.dbPath, err)
	}
	return db, nil
}

// extract reads producers from v_defs and consumers from v_refs, joining
// nodes/_ast for file:line provenance. The root argument is unused here (the
// .db is the source of truth) but kept for backend-symmetry with treeSitterBackend.
func (b macheBackend) extract(_ string) ([]artifact.Producer, []artifact.Consumer, error) {
	db, err := b.open()
	if err != nil {
		return nil, nil, err
	}
	defer db.Close()

	producers, err := b.queryProducers(db)
	if err != nil {
		return nil, nil, err
	}
	consumers, err := b.queryConsumers(db)
	if err != nil {
		return nil, nil, err
	}

	sortProducers(producers)
	sortConsumers(consumers)
	return producers, consumers, nil
}

// queryProducers reads defined symbols from v_defs with provenance from the
// nodes/_ast join (decision 0001's producer query shape).
func (b macheBackend) queryProducers(db *sql.DB) ([]artifact.Producer, error) {
	rows, err := db.Query(`
		SELECT d.token, n.source_file, a.start_row
		FROM v_defs d
		JOIN nodes n ON n.id = d.node_id
		LEFT JOIN _ast a ON a.node_id = d.node_id
	`)
	if err != nil {
		return nil, fmt.Errorf("query v_defs: %w", err)
	}
	defer rows.Close()

	var producers []artifact.Producer
	for rows.Next() {
		var token, sourceFile sql.NullString
		var startRow sql.NullInt64
		if err := rows.Scan(&token, &sourceFile, &startRow); err != nil {
			return nil, fmt.Errorf("scan v_defs row: %w", err)
		}
		if !token.Valid || token.String == "" {
			continue
		}
		producers = append(producers, artifact.Producer{
			Identity:   artifact.NewIdentity(artifact.KindGoPackageSymbol, token.String),
			Provenance: provenance(sourceFile, startRow),
		})
	}
	return producers, rows.Err()
}

// queryConsumers reads references from v_refs with provenance from the
// referrer node (decision 0001's consumer query shape).
func (b macheBackend) queryConsumers(db *sql.DB) ([]artifact.Consumer, error) {
	rows, err := db.Query(`
		SELECT r.token, n.source_file, a.start_row
		FROM v_refs r
		JOIN nodes n ON n.id = r.referrer_node_id
		LEFT JOIN _ast a ON a.node_id = r.referrer_node_id
	`)
	if err != nil {
		return nil, fmt.Errorf("query v_refs: %w", err)
	}
	defer rows.Close()

	var consumers []artifact.Consumer
	for rows.Next() {
		var token, sourceFile sql.NullString
		var startRow sql.NullInt64
		if err := rows.Scan(&token, &sourceFile, &startRow); err != nil {
			return nil, fmt.Errorf("scan v_refs row: %w", err)
		}
		if !token.Valid || token.String == "" {
			continue
		}
		consumers = append(consumers, artifact.Consumer{
			Identity:   artifact.NewIdentity(artifact.KindGoPackageSymbol, token.String),
			Provenance: provenance(sourceFile, startRow),
		})
	}
	return consumers, rows.Err()
}

// provenance converts a nullable source_file + 0-based start_row into a
// 1-based Provenance. A NULL start_row (mention-fidelity, no byte range) yields
// line 0 ("unknown").
func provenance(sourceFile sql.NullString, startRow sql.NullInt64) artifact.Provenance {
	p := artifact.Provenance{File: sourceFile.String}
	if startRow.Valid {
		p.Line = int(startRow.Int64) + 1
	}
	return p
}
