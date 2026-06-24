# assay: deterministic artifact/usage graph (design)

- **Status:** Draft for review
- **Date:** 2026-06-22
- **Supersedes:** the "documentation coverage via set operations" framing in the current `CLAUDE.md` / `README.md`
- **Related work:** beads `assay-dk6` (epic) and children `dk6.1`–`dk6.4`; `mache` ADR-0018 (doc-drift), ADR-0013 (canonical v_defs/v_refs)

## TL;DR

assay derives the **artifact/usage graph** of a codebase — what is produced, what consumes it, and the edges between — **deterministically from source, build, and CI signals**. That graph *is* the architecture/seam map. Because it is derived from reality, it cannot drift, so you never hand-check whether the docs are still true.

The arrow is **code → docs** (derive), not docs → code (grade). Grading hand-written docs against the derived map is a *fallback* mode, not the main act.

"Repo" is not a first-class concept — it is a **scan root**. Mono-repo and multi-repo are the same problem: a graph of globally-identified artifacts whose edges may cross roots. The distinctive value is the **cross-root / build-artifact edges** (e.g. "image built in repo A, consumed by repo B") that no single-repo, AST-only tool sees.

## Problem

The current assay measures documentation coverage as `|C ∩ D| / |C|` — the fraction of code entities named somewhere in the docs. Two problems:

1. **It measures presence, not truth.** Naming `Foo` in a doc scores the same whether the doc describes `Foo` correctly or wrongly. The number rewards mentioning symbols, not making true statements.
2. **It points the wrong way.** It assumes a human authors docs and the tool grades them. The real pain is the *authoring and re-checking*: keeping an architecture/seam description true as the code moves. The crowded doc-drift tool space (docvet, Drift, etc.) all sit here, all at signature/presence level.

The thing actually wanted is **the deterministic version of the `seam-discovery` skill**: that skill's *method* (find cross-package imports, network calls, FFI boundaries, subprocess spawns, shared file paths, feature flags, CI wiring) is already mechanical — only its *executor* (an LLM exploring and writing prose) is non-deterministic and re-drifts. Replace the executor with a deterministic extractor and the map stays current for free.

A motivating edge that no single-repo AST tool captures: **a container image built in one repo and consumed by another.** That seam lives in a `Dockerfile` `FROM`, a CI workflow, an image ref — not in any `.go` file — and is almost never correctly documented.

## Core model

assay is a deterministic function:

```
sources (code + build + CI + manifests, across N scan roots)
        │
        ▼  [extractors]            emit producers & consumers
        ▼  [resolver]             match consumer references → producer ids
        ▼
artifact/usage graph  ──►  the seam/usage map (the doc)
                      └─►  drift report (fallback: derived map vs hand-written claims)
```

### Concepts

- **Artifact** — a producible/consumable unit with a **stable global identity**. The identity is what makes the model repo-agnostic. Examples:
  | Artifact kind | Identity (example) | Produced by | Consumed by |
  |---|---|---|---|
  | Container image | `ghcr.io/agentic-research/mache:0.3` | `Dockerfile` build target / CI publish | `FROM …`, CI `image:`, k8s manifest |
  | Go module | `github.com/agentic-research/assay` | `go.mod` module decl | `require` in another `go.mod`, imports |
  | Go package/symbol | `…/internal/coverage.ComputeFromSources` | declaration | import + reference |
  | CLI / binary | `assay`, `leyline` | `main` package / build | `exec.Command`, CI step, Dockerfile `RUN` |
  | Service endpoint | `unix:///…/daemon.sock`, an HTTP route | server bind | client dial |
  | Schema / format | a capnp schema, a SQLite view (`v_defs`) | definition | reader |

- **Edge** — a directed `producer → consumer` usage relation, created when the **resolver** matches a consumer's *reference* to a producer's *id*. Repo boundaries are invisible to resolution: it keys on the global identity, so an edge resolves the same whether the producer is in the same tree or another.

- **Extractor** — a deterministic, source-kind-specific parser that emits `producer` and `consumer` records (with provenance: file, line). Pluggable. This is the generalization of this week's `dk6.1 ClaimSource` / `dk6.3 StructuralVerifier` interfaces from "claims/entities" to "evidence." An extractor never matches edges — it only emits typed producer/consumer facts; matching is the resolver's job.

- **Resolver** — joins consumer references to producer ids by artifact kind. Produces three buckets:
  - **resolved** edge (producer in some scanned root),
  - **external** (consumer references an id no scanned root produces — a real, surfaced dependency on the outside world),
  - **dangling** (a producer nothing consumes — candidate dead surface).

- **Map** — the resolved graph, emitted as the architecture/seam artifact. Regenerable; the source of truth.

### Why repo-agnostic falls out

Because resolution keys on artifact identity, not location, the only difference between mono-repo and multi-repo is **how many roots you scan** and whether an edge's endpoints happen to live in the same root. "External vs internal" is computed, not configured. Cross-repo is therefore not a separate mode — it is the same engine given more roots.

## Architecture

Packages (proposed; reuse existing where noted):

- `internal/artifact` — `Artifact`, `Identity`, `Producer`, `Consumer`, `Edge`, kinds. The vocabulary.
- `internal/extract` — the `Extractor` interface + registry. (Generalizes `internal/coverage.ClaimSource` and the `StructuralVerifier` seam from dk6.1/dk6.3.)
  - `extract/gocode` — Go imports/symbols. **Prefer consuming mache** (`v_defs`/`v_refs`, architecture) for the in-repo code layer; fall back to in-tree tree-sitter (existing `internal/code`) when mache is unavailable.
  - `extract/dockerfile` — `FROM`, build stages, `COPY --from`, published tags.
  - `extract/ci` — one CI format to start (GitHub Actions YAML): jobs that build/publish/pull images, run binaries.
  - `extract/gomod` — module decl + `require` edges.
- `internal/resolve` — identity matching, edge bucketing (resolved/external/dangling).
- `internal/graph` — the graph + serialization.
- `internal/report` — emit the map (and the drift fallback). Reuse/retarget existing `internal/coverage/report.go`.
- `cmd` — `assay map <root...>` (derive + emit) and `assay drift <root...> --against <doc>` (fallback).

Data flow is one pass per root to gather producer/consumer facts, then a single global resolve across all roots, then emit.

## v1 scope

In:
- Scan roots: one or more local paths (covers mono-repo and multi-repo identically).
- Extractors: **Go code** (via mache, tree-sitter fallback) + **Dockerfile** + **GitHub Actions CI** + **go.mod**.
- Artifact kinds: Go module, Go package/symbol, container image, CLI/binary.
- Output: the derived map (machine-readable JSON + a human Markdown/mermaid rendering), with resolved / external / dangling buckets.
- The motivating case must work end to end: image built in root A, `FROM`/pulled in root B → one resolved cross-root edge.

Out (v1):
- The `drift` fallback can be a thin v1 (does a hand-written ARCHITECTURE.md mention each resolved seam?) or deferred — decide during planning.
- Non-Go languages, additional CI providers, k8s/Helm manifests, service-endpoint extraction — later extractors behind the same interface.

## Relationship to mache

mache derives the **in-repo symbol graph** deterministically (`get_architecture`, `find_callers`, `v_defs`/`v_refs`, `get_diagram`). assay does **not** rebuild that. assay adds the layer mache lacks: **build/CI/artifact edges** and **cross-root resolution**. The `extract/gocode` extractor *consumes* mache for the code layer (process boundary, the established ecosystem pattern) rather than duplicating it. This keeps assay in Go, beside mache, and matches how the ecosystem joins tools (wire/subprocess, not rewrites).

## Relationship to the dk6 work

- **`dk6.1 ClaimSource`** and **`dk6.3 StructuralVerifier`** are the seed of the `Extractor` model — keep and generalize them; the interface designs survive. The Go branches (`assay-dk6.1-claimsource`, `assay-dk6.3-structural`) are the starting point.
- **`dk6.2` (HTML/DOM source)** — **parked.** Out of scope for the artifact-graph shape.
- **`dk6.4` (semantic matcher / HDC)** — **parked.** The deterministic graph needs no semantic matching.
- The doc-coverage metric becomes, at most, one weak derived view; the artifact/usage map is the primary product.

## Non-goals / explicitly parked

These were explored and deliberately set aside; record so they are not silently reintroduced:

- **Rust rewrite** — not needed. assay stays Go to sit beside mache (its integration partner) and follow the ecosystem's "Go tool + Rust substrate over wire/subprocess" pattern.
- **ley-line CAS (BlobStore/RootPointer), HDC dual-readout, sheaf** — research-stack; none required for deterministic extraction + a graph. Revisit only if a real *semantic* ("do these two things *mean* the same?") need appears — it has not.
- **Content-addressing of claims/verdicts** — possible future durability layer (cloister's `BlobStore`/`RefStore` pattern over BLAKE3 FFI), not v1.
- **Grading-first / docs-as-input** — the arrow is derive-first; grading is a fallback.

## Testing strategy

Determinism is the core property, so test it directly:

- **Golden graphs:** fixture root trees → assert exact emitted graph. Same input → identical output (the property the LLM `seam-discovery` lacks).
- **Per-extractor unit tests:** small fixtures (a `Dockerfile`, a workflow YAML, a `go.mod`) → expected producer/consumer records with provenance.
- **Resolver tests:** the three buckets — resolved (multi-root), external (unscanned producer), dangling (unused producer).
- **The motivating case as an integration test:** two fixture roots, image built in one and consumed in the other → exactly one resolved cross-root edge.
- TDD throughout (consistent with how dk6.1–dk6.4 were built).

## Open questions (resolve during planning)

1. **mache coupling:** subprocess/MCP call, or read its `.db`/canonical views directly? (Affects whether mache must be running.)
2. **Graph serialization:** roll our own JSON, or emit something already consumable (e.g. a shape mache or a diagram tool can ingest)?
3. **Drift fallback in v1** or deferred to v2?
4. **Identity normalization:** how aggressively to canonicalize image refs (tags/digests/registries) so A's published tag matches B's `FROM` reliably.
