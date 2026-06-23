# 0003 — drift fallback: v1 scope

- **Status:** Accepted (pending human review) — **product-scope call; human confirmation requested**
- **Date:** 2026-06-23
- **Bead:** `assay-927976` (T0-4)
- **Resolves spec open question:** #3 — "Drift fallback in v1 or deferred to v2?"
- **Gates:** the T6 thread (drift implementation).
- **Spec:** `docs/superpowers/specs/2026-06-22-assay-artifact-usage-graph-design.md`,
  §"TL;DR", §"v1 scope" (the "Out (v1)" note), §"Non-goals".

## Decision

**Defer the `drift` command to v2.** v1 ships **derive-and-emit only**:
`assay map <root...>` producing the artifact/usage graph (resolved / external /
dangling) as JSON + Markdown/mermaid. No `assay drift … --against <doc>` in v1.

This confirms the lead's working preference (YAGNI: keep v1 focused on deriving
the map). It is a **go/no-go that closes the T6 thread for v1** — T6 is parked,
not cancelled, with a concrete v2 scope (below) so it can be re-opened cleanly.

## Rationale

- **The spec already subordinates drift.** It states the arrow is "**code → docs**
  (derive), not docs → code (grade)" and that "Grading hand-written docs against
  the derived map is a *fallback* mode, not the main act." The §"v1 scope" "Out"
  list explicitly offers drift as "a thin v1 … or deferred — decide during
  planning." This decision takes the deferral.
- **Drift is strictly downstream of the map.** Drift = (derived map) − (claims
  parsed from a hand-written doc). It cannot exist or be tested before the map is
  correct and stable. Shipping the map first means drift in v2 builds on a frozen,
  golden-tested foundation instead of a moving one.
- **The map already delivers the headline value without drift.** The spec's
  distinctive claim — cross-root / build-artifact edges that "no single-repo,
  AST-only tool sees", and the motivating image-built-in-A-consumed-in-B edge —
  is entirely in the *map*. Drift adds nothing to that demonstration. Spending v1
  on a doc-parser + claim-matcher delays the load-bearing capability for a
  fallback feature.
- **Drift reintroduces the matching problems the spec parked.** Grading a
  hand-written `ARCHITECTURE.md` against the map needs claim extraction from prose
  and fuzzy matching of those claims to graph nodes — the exact "semantic
  matcher / HDC" territory the spec explicitly **parked** (`dk6.4`, and
  "ley-line CAS / HDC / sheaf" under Non-goals). A "thin" drift that only does
  literal substring presence would regress toward the old `|C ∩ D| / |C|`
  presence metric the spec is explicitly moving *away* from ("measures presence,
  not truth"). Doing drift *well* is a v2-sized effort; doing it *thinly* risks
  resurrecting the discredited metric. Deferring avoids both.
- **Cleaner v1 surface.** One verb (`map`) with one job is easier to test for the
  determinism property (golden graphs) and easier to document than a two-verb CLI
  where the second verb has fuzzy semantics.

## What v1 ships instead (the honest substitute)

Deferring drift does **not** mean v1 can't tell you something is undocumented.
The `dangling` and `external` buckets are *derived-side* drift signals that need
no hand-written doc:

- **`dangling`** — a producer nothing consumes (candidate dead surface).
- **`external`** — a consumer referencing an id no scanned root produces (a
  surfaced outside-world dependency, e.g. `gcr.io/distroless/...`).

These give "what's unused / what crosses the boundary" for free, which covers a
large fraction of what users would have asked drift for, without parsing prose.

## Contract / spec the dependent (T6) beads build against — for v2

When T6 is re-opened, it builds on the v1 map with this shape (recorded now so the
deferral is concrete, not vague):

```
assay drift <root...> --against <doc.md>
  1. derive the map (reuse v1 `assay map` pipeline, unchanged).
  2. parse <doc.md> into claims — REUSE the dk6.1 `ClaimSource` /
     tree-sitter-markdown machinery already in the tree (internal/docs).
  3. match each derived seam (resolved edge / artifact id) against the claims:
       - covered:    a claim names this seam/id.
       - missing:    a derived seam with no claim (doc is stale / incomplete).
       - phantom:    a claim naming a seam/id the map does not contain
                     (doc asserts something untrue).
  4. emit a drift report (reuse internal/report rendering).
```

v2 design must decide claim-match fidelity (literal id match vs signature vs
semantic) — and only then revisit whether any of the parked `dk6.4` semantic
matching is warranted. v1 leaves all of that untouched.

The v1 `map` output JSON MUST be a stable, documented schema so v2 drift can
consume it without re-deriving — i.e. `map` is the contract boundary, and T6
depends on the map schema, not on map internals.

## Residual risk

- **A user may expect grading on day one.** The current `CLAUDE.md` / `README.md`
  still describe the old "documentation coverage" framing, which *is* a grading
  story; shipping `map`-only changes the headline. Mitigation: the spec already
  marks itself as superseding that framing; v1 docs must lead with derive-first
  and point `dangling`/`external` at the "what's undocumented" question. **This
  is the product-scope judgement the human should confirm** — see below.
- **Re-opening cost.** Parking T6 risks the v2 drift contract drifting from the
  v1 map. Mitigation: the map JSON schema is frozen and golden-tested in v1, and
  this ADR records the v2 drift contract against that schema.
- **"Thin drift" temptation returns under pressure.** If a stakeholder pushes for
  *some* drift in v1, the smallest defensible scope is: literal artifact-id
  presence check against a doc (does `ARCHITECTURE.md` mention each *resolved
  edge's* producer and consumer id?), explicitly NOT prose claim extraction, and
  reported separately from the map. This is the only in-v1 scope this ADR would
  endorse, and only if derive-first is not thereby delayed.

## Human confirmation requested

This is a **product-scope call**, not a purely technical one. The recommendation
is **defer drift to v2**. The human should confirm specifically:

1. v1 = `assay map` only (derive + emit), no `assay drift`. ✅ recommended.
2. Accept that "is my ARCHITECTURE.md still true?" is answered in v2, with v1
   offering `dangling` / `external` as the interim "what's undocumented/unused"
   signal.
3. If (1) is rejected, adopt the thin literal-id-presence drift scoped in
   "Residual risk" above — nothing larger — to avoid resurrecting the parked
   semantic-matching work.
