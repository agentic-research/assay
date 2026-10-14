# CLAUDE.md

## Build & Test

```bash
task build         # Build binary → bin/assay
task test          # Run all tests (go test -race -v ./...)
task lint          # golangci-lint run ./...
task fmt           # gofumpt -w -extra .
task map           # Regenerate docs/dependency-graph.md (assay over itself)
task check         # DEV gate (mutating): fmt + vet + lint + test + map
task ci            # MERGE gate (non-mutating): fmt:check + vet + lint + test + map:check
task hooks:install # Install repo git hooks (pre-push runs `task ci`)
```

`task ci` is the single source of truth — the `assay-map` workflow and the `.githooks/pre-push`
hook both invoke it (never re-implement). `task check` (dev) regenerates the dependency-graph
snapshot so a graph change rides along in the same commit; `task ci` (CI/hook) only *verifies*
it (`map:check`), never rewrites the tree. Run `task hooks:install` once after cloning.

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
| `cmd/` | Cobra CLI: `map` (derive + emit), `verify` (doc gate), `version` |

### Identity & resolution

Artifacts carry a stable global `Identity` (kind + canonical ref). The resolver keys on a
**version-stripped** identity so repo boundaries are invisible — a producer in one root and
a consumer in another resolve to one cross-root edge. See `docs/decisions/0002-identity-normalization.md`.

### mache coupling

`gocode` reads mache's canonical `v_defs`/`v_refs` from a `.db` via pure-Go
`modernc.org/sqlite` (mache need not be running); tree-sitter is the always-available
fallback. See `docs/decisions/0001-mache-coupling.md`.

### The doc gate (`assay verify`)

`assay verify` computes documentation coverage over the same tree-sitter entity extraction the
map uses, and — with `--max-uncovered` or `--threshold` — **fails the build** when an exported
entity has no documentation reference. `task docs:check` wires it into `task ci` with a ratchet
budget (`DOC_UNCOVERED_BUDGET`); lower it as debt is paid, never raise it to make CI pass.

It gates **both directions**:

- **Coverage** — exported entity → no doc mention (`--max-uncovered` / `--threshold`). Entities
  come from parsed source, so every finding names a real symbol.
- **Dead references** — a doc naming an entity of this repo that no longer exists
  (`--fail-on-dead-refs`, no budget). The failure names the entity and the `file:line` of the doc.

The dead-reference check does **not** gate the raw `staleness` figure. That figure treats every
backtick span as a claimed symbol, so filenames, CLI flags, bead IDs and other repos' types all
register as stale (92.6% on this tree); gating it would fail permanently while signalling
nothing. Instead a span counts as an entity reference only when its own shape says "this repo's
symbol", and it is resolved **exactly** (the fuzzy coverage layers would happily match a
deleted `Gate`-prefixed name back to `Gate`):

- **package-qualified** — `pkg.Name` or `pkg.Type.Method`, where `pkg` is a package of this repo
  and `Name` is exported (`coverage.ClaimSource`; not `wrangler.toml`);
- **receiver-qualified** — `Type.Method`, where `Type` is a type of this repo
  (`MarkdownSource.Claims`).

That is how the docs here actually name entities, and on this tree it flags nothing. The blind
spots are deliberate: **bare names** (`Gate`) are not checked, because a bare identifier cannot be
told apart from a type quoted from another project; and a package whose exported entities were
*all* deleted stops being recognised as local. Write a qualified name when a doc must be held to
the code. The rule lives in `DeadRefs` (`internal/coverage/deadref.go`).

mache's `v_doc_refs` preprocessor does not help here: the view is scoped to Rust paths
(`token LIKE '%::%'`, sidestepping `ley-line-open-651909`), so it yields zero rows for Go.

The decision itself is a pure function: `Gate` in `internal/coverage/gate.go` takes a
`CoverageResult` plus `GateOptions` (`MaxUncovered` + `MaxUncoveredSet` for the ratchet,
`MinCoverage` for the ratio floor, `FailOnDeadRefs` for dead references) and returns an error
naming every violating entity, or nil.
The zero value of `GateOptions` disables every check, so adding it to a caller is opt-in.
`cmd/verify.go` returns that error rather than calling `os.Exit`, which keeps it unit-testable
and puts the offending names in the CI log.

### Two swappable seams

`verify` is a join, and each side varies behind one interface — see
[`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) for the full table.

- **`coverage.ClaimSource`** — what the docs claim. `MarkdownSource` is the default;
  `HTMLSource` is real and opt-in via `--html-docs`; `DOMSource` (live CDP capture) is a
  guarded seam.
- **`coverage.StructuralVerifier`** — what the code has. `TreeSitterVerifier` is the
  always-available default; `MacheVerifier` is selectable via `--verifier=mache --mache-db`
  and guarded.

**Guarded backends return an error, never an empty result.** An empty claim set and an empty
entity set are both meaningful in a coverage join — "the docs assert nothing" and "the code
contains nothing". A backend that cannot run must emit neither, since both silently invert the
gate's verdict. `Available()` reports false so callers fall back.

### Direction & parked non-goals

v1 = derive the map (4 extractors + resolver + `assay map`). **Parked** (do not reintroduce):
semantic/HDC matching, HTML/DOM extraction, a Rust rewrite, and the `assay drift` grading
fallback (v2). Spec: `docs/superpowers/specs/2026-06-22-assay-artifact-usage-graph-design.md`.

Note: doc-coverage set operations were previously listed as parked. They are **retained** as the
gate above — the parked item was doc coverage as a *product* (a grade), not as a *build gate*.
