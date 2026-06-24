# CLAUDE.md

## Build & Test

```bash
task build    # Build binary → bin/assay
task test     # Run all tests (go test -race -v ./...)
task lint     # golangci-lint run ./...
task fmt      # gofumpt -w -extra .
task check    # fmt + vet + lint + test
```

## Architecture

assay derives a codebase's **artifact/usage graph** from source, build, and CI signals
(`code → docs` — derive, not grade). The derived graph *is* the architecture/seam map; the
headline value is the cross-repo build edge (image/module produced in one repo, consumed in
another). "Repo" is just a scan root — mono-repo and multi-repo are the same engine.

### Data Flow

```
go.mod · Dockerfile · CI · Go source
        │  extractors (one per source kind; emit producer/consumer facts + provenance,
        │              never match edges)
        ▼
   []Producer / []Consumer        (Registry gathers across N scan roots)
        │  resolver (match consumer refs → producer ids by version-stripped identity)
        ▼
   resolved / external / dangling
        │  report
        ▼
   artifact/usage map  (JSON · mermaid · md)
```

### Key Packages

| Package | Role |
|---------|------|
| `internal/artifact/` | Vocabulary: `Identity` (canonical key), `Artifact`, `Producer`, `Consumer`, `Edge`, `Kind` |
| `internal/extract/` | `Extractor` interface + `Registry`; sub-extractors `gomod`, `dockerfile`, `ci`, `gocode` |
| `internal/resolve/` | Identity matching → resolved / external / dangling buckets |
| `internal/report/` | Emit the map as JSON / mermaid / markdown |
| `internal/code/` | Tree-sitter Go extraction (the `gocode` fallback backend) |
| `cmd/` | Cobra CLI: `map` (derive + emit), `version` |

### Identity & resolution

Artifacts carry a stable global `Identity` (kind + canonical ref). The resolver keys on a
**version-stripped** identity so repo boundaries are invisible — a producer in one root and
a consumer in another resolve to one cross-root edge. See `docs/decisions/0002-identity-normalization.md`.

### mache coupling

`gocode` reads mache's canonical `v_defs`/`v_refs` from a `.db` via pure-Go
`modernc.org/sqlite` (mache need not be running); tree-sitter is the always-available
fallback. See `docs/decisions/0001-mache-coupling.md`.

### Direction & parked non-goals

v1 = derive the map (4 extractors + resolver + `assay map`). **Parked** (do not reintroduce):
documentation-coverage set operations, semantic/HDC matching, HTML/DOM extraction, a Rust
rewrite, and the `assay drift` grading fallback (v2). Spec:
`docs/superpowers/specs/2026-06-22-assay-artifact-usage-graph-design.md`.
