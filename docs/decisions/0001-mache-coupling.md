# 0001 — mache coupling mechanism

- **Status:** Accepted (pending human review)
- **Date:** 2026-06-23
- **Bead:** `assay-925a0d` (T0-1)
- **Resolves spec open question:** #1 — "mache coupling: subprocess/MCP call, or read its `.db`/canonical views directly?"
- **Spec:** `docs/superpowers/specs/2026-06-22-assay-artifact-usage-graph-design.md`, §"Architecture" (`extract/gocode`) and §"Relationship to mache".

## Decision

`extract/gocode` reads mache's **on-disk `.db` directly via the canonical views
`v_defs` / `v_refs`**, opened **read-only** with the **pure-Go `modernc.org/sqlite`
driver** (driver name `"sqlite"`, no cgo). **mache does not need to be running.**

Concretely:

1. assay obtains a `.db` for a scan root. If one is not supplied, assay shells
   out to `mache build <root> <tmp.db>` **once** to produce it (mache's own
   documented build command; this is a build step, not a live dependency on a
   running server).
2. assay opens that `.db` with `sql.Open("sqlite", dbPath+"?mode=ro")`.
3. assay calls `ingest.EnsureCanonicalViews(db)` (exported by mache for exactly
   this case) so the read works against `.db` files written by older mache
   builds or by ley-line-open that predate the views.
4. assay queries `v_defs` / `v_refs` (+ `nodes` / `_ast` for provenance) to emit
   `producer` and `consumer` records for the Go code layer.

**stdio / MCP is an explicit fallback only**, used when there is no `.db` and
`mache build` is unavailable (no `mache` binary on PATH), or for live un-parsed
sources where re-building is not acceptable. It is not the primary path.

## Rationale

- **No running daemon.** The spec's open question is framed as "affects whether
  mache must be running." Reading the `.db` removes that coupling entirely: a
  `.db` is a static artifact assay can open with a library, exactly as mache's
  own `LoadFileIndex` already does (`sql.Open("sqlite", dbPath+"?mode=ro")`,
  `internal/ingest/sqlite_writer.go:248`). Determinism — assay's core property —
  is far easier to guarantee against a frozen file than against a live server's
  responses.
- **Pure Go, no cgo.** mache already standardised on `modernc.org/sqlite`
  (`go.mod:22`, `modernc.org/sqlite v1.52.0`; imported blank in
  `internal/ingest/sqlite_writer.go:13`). assay keeps the same driver, so the
  combined toolchain stays cgo-free and cross-compiles cleanly — matching the
  spec's "stay Go, sit beside mache" stance.
- **Stable contract.** ADR-0013 makes `v_defs` / `v_refs` the *canonical
  consumer surface*: the comment at `sqlite_writer.go:107-117` states consumers
  query the views "instead of `node_defs` / `node_refs` directly so they're
  producer-agnostic — when LSP-resolved rows land … the view definition expands
  with a UNION ALL and consumer SQL doesn't change." assay binding to the views
  inherits that forward compatibility for free: when binding-fidelity (LSP) rows
  appear, assay's edges get more precise with no code change.
- **The views are explicitly meant to be installed by external consumers.**
  mache exports `CanonicalViewsDDL` and `EnsureCanonicalViews(db *sql.DB)`
  (`sqlite_writer.go:292-328`) with the documented purpose: "Exposed so consumers
  that opened a `.db` from a different writer … can install the views on demand."
  This is mache sanctioning the read-the-.db integration pattern assay adopts.
- **MCP is heavier and less deterministic.** The MCP/serve surface answers
  semantic queries (`get_architecture`, `find_callers`) over a running process;
  it is the right tool for interactive exploration, not for a batch extractor
  that needs byte-stable output. Keeping it as fallback preserves a path for the
  un-parsed-live-source case without paying its cost on the common path.

## Contract / spec the dependent beads build against

`extract/gocode`'s mache backend MUST implement against this contract.

### Driver and connection

```go
import (
    "database/sql"
    _ "modernc.org/sqlite" // pure-Go driver, registered as "sqlite"
)

db, err := sql.Open("sqlite", dbPath+"?mode=ro") // read-only, no server
// then, defensively, install views if the .db predates them:
//   mache exposes ingest.EnsureCanonicalViews(db) / ingest.CanonicalViewsDDL
//   (assay may vendor the identical CREATE VIEW IF NOT EXISTS DDL to avoid a
//    mache import; the DDL is in the contract below).
```

If assay prefers not to take a source dependency on mache, it MAY inline the
DDL (it is `CREATE VIEW IF NOT EXISTS`, idempotent and safe to re-run):

```sql
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
```

### Canonical view shapes (the read surface)

```
v_defs(token TEXT, node_id TEXT, fidelity TEXT)
    -- "the symbol `token` is defined at `node_id`."
    -- fidelity ∈ {'mention','binding','reachability'}; today always 'mention'.

v_refs(referrer_node_id TEXT, token TEXT,
       target_node_id TEXT,   -- NULL at mention fidelity
       ref_uri TEXT,          -- NULL at mention fidelity
       ref_line INTEGER,      -- NULL at mention fidelity
       fidelity TEXT)
    -- "the node `referrer_node_id` references something named `token`."
    -- When binding rows land, target_node_id/ref_uri/ref_line are populated.
```

### Provenance (file + line for emitted records)

`node_id` is a path-like AST id (e.g. `source_file/function_declaration/identifier`).
To attach file/line provenance to a producer/consumer record, join `nodes`
(`id`, `name`, `kind`, `parent_id`, `record JSON`, `source_file`) and, for byte
ranges, the `_ast` table (`node_id`, `source_id`, `start_byte`, `end_byte`,
`start_row`, `start_col`, …). `nodes.source_file` is the path; `_ast.source_id`
+ `start_row` give file:line.

Example producer query (exported Go symbols → `producer` records):

```sql
SELECT d.token, d.node_id, n.source_file, a.start_row
FROM v_defs d
JOIN nodes n ON n.id = d.node_id
LEFT JOIN _ast a ON a.node_id = d.node_id;
```

Example consumer query (references → `consumer` records):

```sql
SELECT r.referrer_node_id, r.token, n.source_file, a.start_row
FROM v_refs r
JOIN nodes n ON n.id = r.referrer_node_id
LEFT JOIN _ast a ON a.node_id = r.referrer_node_id;
```

### Obtaining the `.db`

- Preferred: caller supplies a `.db` path (already built).
- Else: `mache build <root> <tmp.db>` (mache's documented command,
  `cmd/build.go:37`, `Use: "build [source] [output.db]"`). assay treats the
  resulting file as an opaque, frozen input.
- Fallback only: if no `.db` and no `mache` binary, fall back to the in-tree
  tree-sitter extractor (existing `internal/code`) or stdio/MCP.

### Invariants for dependent beads

- assay MUST open the `.db` read-only and MUST NOT write to it.
- assay MUST NOT assume mache is running.
- assay MUST tolerate a `.db` missing the views (call EnsureCanonicalViews / run
  the inline DDL first).
- assay MUST tolerate `fidelity = 'mention'`-only data (NULL binding columns) and
  MUST treat populated binding columns as strictly better when present.

## Worked example

mache's own incremental-index reader already demonstrates the pattern assay
copies (`internal/ingest/sqlite_writer.go:247-284`): `sql.Open("sqlite",
dbPath+"?mode=ro")`, guard for table existence via `sqlite_master`, then plain
`SELECT`. assay does the same against `v_defs` / `v_refs` rather than
`file_index`.

End to end for one Go root:
1. `mache build ~/remotes/art/mache /tmp/mache.db` (once).
2. `sql.Open("sqlite", "/tmp/mache.db?mode=ro")`.
3. `EnsureCanonicalViews(db)`.
4. `SELECT token, node_id FROM v_defs` → producer records for each Go symbol.
5. `SELECT referrer_node_id, token FROM v_refs` → consumer records.
6. Hand records to `internal/resolve`; mache process never ran during 2–6.

## Residual risk

- **Schema drift in mache.** `v_defs`/`v_refs` are ADR-0013 "Proposed" status;
  column names could still change before mache stabilises them. Mitigation:
  bind to the *views* (the sanctioned consumer surface) not the base tables, and
  pin a tested mache version in assay's integration fixtures. A golden-graph
  test over a checked-in fixture `.db` will catch a shape change immediately.
- **`mache build` backend variance.** `mache build` may use the leyline backend
  or in-process tree-sitter (`cmd/build.go` `--backend=auto`). Both write the
  same `node_defs`/`node_refs` → same views, so the read contract is unaffected,
  but emitted byte ranges could differ. For deterministic golden tests, assay
  should pin `--backend=tree-sitter` (or ship a pre-built fixture `.db`).
- **Inlining the DDL vs importing mache.** Inlining avoids a Go module dependency
  but duplicates the DDL; if mache evolves the view bodies, an inlined copy goes
  stale silently. Recommended: import `ingest.EnsureCanonicalViews` if the module
  dependency is acceptable; otherwise add a test asserting assay's inlined DDL
  matches mache's `CanonicalViewsDDL` constant.
- **`.db` for un-parsed / live sources.** Reading a `.db` can't see edits made
  since the last build. This is the explicit case the MCP fallback exists for;
  v1 accepts staleness on the build-the-.db path (determinism over freshness).
