# 0002 — identity normalization rules

- **Status:** Accepted (pending human review)
- **Date:** 2026-06-23
- **Bead:** `assay-926eb8` (T0-3)
- **Resolves spec open question:** #4 — "Identity normalization: how aggressively
  to canonicalize image refs (tags/digests/registries) so A's published tag
  matches B's `FROM` reliably."
- **Spec:** `docs/superpowers/specs/2026-06-22-assay-artifact-usage-graph-design.md`,
  §"Concepts" (Artifact identity, Edge, Resolver).

## Decision

The resolver matches a consumer reference to a producer id by comparing their
**canonical identity strings**. Each artifact kind has a deterministic
`Canonicalize(raw, ctx) → CanonicalID` function. An edge resolves iff
`Canonicalize(producer) == Canonicalize(consumer)` for the same kind.

Canonicalization is split into two passes so the rules are testable in isolation:

1. **Interpolation pass** — resolve CI/template variables to literals using a
   per-scan-root **context** (env, repo owner, repo name). Runs only on strings
   that contain `${{ … }}` / `$VAR` syntax.
2. **Normalization pass** — fold each kind's optional/implicit components to a
   single canonical form.

A match key has two layers (see §"Matching policy"): a **strict key** including
version/tag, and a **identity key** ignoring version. v1 resolves on the
identity key and annotates whether the version also matched, so a tag/version
mismatch surfaces as a *weak* edge rather than a missed one.

## Canonical forms by kind

### Container image refs

Canonical form: `registry/repository:tag` **or** `registry/repository@digest`.

Normalization rules:

- **Registry host default.** A ref with no registry host (first path segment
  contains no `.` and no `:` and is not `localhost`) defaults to `docker.io`,
  and a bare `name` (no `/`) defaults to `docker.io/library/name` (Docker Hub
  official-image namespace). `ghcr.io/...`, `gcr.io/...`, `registry.k8s.io/...`
  keep their explicit host.
- **Implicit tag.** No `:tag` and no `@digest` ⇒ append `:latest`.
- **Tag vs digest.** If a `@sha256:…` digest is present it is the *strict*
  identity (a digest pins content); any `:tag` before it is advisory. If only a
  tag is present, the tag is the strict version component.
- **Case.** Registry host and repository path are case-sensitive per the OCI
  spec **except** the host, which is lowercased (DNS). Do not lowercase the
  repository path.
- **Identity key** = `registry/repository` (tag and digest stripped). This is
  what the resolver keys on so "image published as `:0.3`" matches "image pulled
  as `:latest`" as the *same artifact*, with the version delta recorded.

### Go module ids

Canonical form: the module path, e.g. `github.com/agentic-research/mache`.

Normalization rules:

- **Strip the version.** `require github.com/agentic-research/mache v0.5.5` and a
  pseudo-version `v0.0.0-20230505084239-6fb4a4f75381` both canonicalize to the
  bare module path `github.com/agentic-research/mache`. Version is kept as the
  strict component, not part of identity.
- **Strip `+incompatible` and major-version suffix handling.** `…/v2`,`…/v3` in
  the path are a *real* part of module identity (different major = different
  module) and are KEPT. `+incompatible` build metadata is stripped from the
  version, not the path.
- **`replace` directives.** A `replace X => ../local` means the consumer's import
  of `X` is satisfied by a local tree. The resolver keys on the *original*
  module path `X` (left side), so the edge still resolves to the producing root;
  the replace is recorded as provenance, not a new identity. (x-ray ships exactly
  this, commented out: `// replace github.com/agentic-research/mache => ../mache`,
  `go.mod:51`.)
- **Subdir module → owning repo.** A module path may be deeper than the repo:
  `github.com/agentic-research/ley-line-open/clients/go/leyline-schema` is a Go
  module whose **owning repo** is `github.com/agentic-research/ley-line-open`.
  The module *identity* is the full module path (that's what `require` matches);
  the **owning-repo identity** (`host/owner/repo`, first 3 path segments for
  `github.com`/`gitlab.com`-style hosts) is computed alongside it so the resolver
  can also answer "which repo/root produces this?" and link a module edge to a
  container-image edge from the same repo. Both keys are emitted.

### CI env interpolation

Canonical form: the literal string after `${{ … }}` substitution.

Normalization rules:

- **`github.repository_owner`** → the owner segment of the scan root's
  `origin` remote (or an explicitly supplied context). For these repos it is
  `agentic-research`.
- **`github.repository`** → `owner/repo`.
- **`github.ref_name` / a release `inputs.tag`** → the tag literal (e.g.
  `v0.1.0`); when unknown at scan time, treat as the wildcard version component
  (identity-key match still succeeds; strict-key match is "unknown").
- **`env.*` references** → resolved against the workflow's own `env:` block first
  (e.g. `${{ env.IMAGE_NAME }}` → its `env:` definition), then the scan context.
- Unsubstitutable variables (secrets, matrix values with no fixture) ⇒ the ref is
  marked **unresolvable** and excluded from strict matching, but its literal
  prefix/suffix may still seed an identity-key match.

## Matching policy

- **Resolve on the identity key** (kind-specific, version-stripped): image
  `registry/repository`, module path, owning repo. This is what creates the
  edge — the spec's value is the *seam*, which exists regardless of version.
- **Annotate the strict delta.** Record whether the version/tag/digest also
  matched (`version_match ∈ {exact, mismatch, unknown}`). A resolved edge with
  `version_match=mismatch` is a real, reportable signal (consumer pins an old
  tag) but is still a resolved edge, not a miss.
- **Buckets** (spec §Resolver) are computed on the identity key: *resolved*
  (producer in some scanned root), *external* (no scanned root produces it),
  *dangling* (producer nothing consumes).

## Worked examples

### `ghcr.io/agentic-research/rosary`

Producer side (rosary `release.yml`):
```
env.IMAGE_NAME = ghcr.io/${{ github.repository_owner }}/rosary
                            └── github.repository_owner = agentic-research
push tag       = ${{ env.TAG }} = ${{ inputs.tag || github.ref_name }} = v0.1.0
```
1. Interpolation: `ghcr.io/agentic-research/rosary` ; tag `v0.1.0`.
2. Normalization: host `ghcr.io` (kept, lowercased), repo
   `agentic-research/rosary`, explicit tag `v0.1.0`.
   - strict key: `ghcr.io/agentic-research/rosary:v0.1.0`
   - **identity key: `ghcr.io/agentic-research/rosary`**
   - owning-repo identity: `github.com/agentic-research/rosary` (links this
     image to rosary's Go-module / repo node).

Consumer side, a hypothetical `FROM ghcr.io/agentic-research/rosary` in another
root:
1. No interpolation needed.
2. Normalization: implicit tag ⇒ `:latest`.
   - strict key: `ghcr.io/agentic-research/rosary:latest`
   - **identity key: `ghcr.io/agentic-research/rosary`** ← matches producer.

Result: **one resolved cross-root edge**, `version_match=mismatch`
(`v0.1.0` vs `latest`) — exactly the spec's motivating "image built in A,
consumed in B" case, surfaced even though the tags differ.

### `github.com/agentic-research/mache`

Producer side (mache `go.mod`): `module github.com/agentic-research/mache`.
- identity key: `github.com/agentic-research/mache`
- owning-repo identity: `github.com/agentic-research/mache`

Consumer side (x-ray `go.mod:6`): `require github.com/agentic-research/mache v0.5.5`.
1. Strip version `v0.5.5`.
   - strict key: `github.com/agentic-research/mache@v0.5.5`
   - **identity key: `github.com/agentic-research/mache`** ← matches producer.
2. Note the commented `replace … => ../mache` (`go.mod:51`): were it active, the
   left side `github.com/agentic-research/mache` is still the key — same edge,
   plus a `replace=../mache` provenance note.

Result: **one resolved module edge**, `version_match` recorded against whatever
version mache's repo currently tags.

### Subdir module: `…/ley-line-open/clients/go/leyline-schema`

Producer (`ley-line-open/clients/go/leyline-schema/go.mod`):
`module github.com/agentic-research/ley-line-open/clients/go/leyline-schema`.
- module identity key: the full path (matches any `require` of it).
- owning-repo identity: `github.com/agentic-research/ley-line-open` (first three
  segments). ley-line-open's root is a Rust repo with no root `go.mod`; the Go
  identity lives entirely in the subdir, but the *repo seam* still attributes to
  ley-line-open via the owning-repo key.

### Image with explicit registry + implicit tag (x-ray Dockerfile)

`FROM gcr.io/distroless/static-debian12` (`x-ray/Dockerfile:10`):
- host `gcr.io` kept; implicit tag ⇒ `:latest`.
- identity key: `gcr.io/distroless/static-debian12` → no scanned root produces
  it ⇒ **external** edge (a surfaced dependency on the outside world, which is
  the correct, useful classification).

`FROM golang:1.26` (`x-ray/Dockerfile:1`):
- bare name ⇒ `docker.io/library/golang`; tag `1.26`.
- identity key: `docker.io/library/golang` ⇒ **external**.

## Contract / spec the dependent beads build against

Each extractor emits identities already shaped for the resolver:

```go
type CanonicalID struct {
    Kind        ArtifactKind // image | gomodule | gobinary | ...
    IdentityKey string       // version-stripped match key (resolver keys on this)
    StrictKey   string       // includes tag/digest/module-version
    OwningRepo  string       // host/owner/repo, when derivable ("" otherwise)
    Version     string       // tag | digest | module version | "" | "unknown"
    Raw         string       // original text, for provenance
    Resolvable  bool         // false if interpolation left unresolved vars
}
```

Resolver rule:
- group producers and consumers by `(Kind, IdentityKey)`;
- an edge resolves when both sides share `(Kind, IdentityKey)`;
- set `version_match` by comparing `Version` (digest beats tag; `"unknown"` ⇒
  `unknown`);
- unresolved consumer ⇒ `external`; unconsumed producer ⇒ `dangling`.

Interpolation context (per scan root):
```go
type CIContext struct {
    RepositoryOwner string // from origin remote owner; e.g. "agentic-research"
    Repository      string // "owner/repo"
    RefName         string // tag, when scanning a tagged state; else ""
    WorkflowEnv     map[string]string // the workflow's own env: block
}
```

## Residual risk

- **Identity-key vs strict-key default.** Resolving on the identity key
  maximises seam discovery (the spec's stated value) but will link a producer to
  a consumer pinning a wildly old version. v1 surfaces this as
  `version_match=mismatch` rather than hiding it; if that proves noisy, a future
  flag can require strict match. Flagged as the main tunable.
- **`docker.io/library` heuristic.** The "bare name ⇒ library namespace" rule is
  Docker Hub-specific; an air-gapped registry serving bare names would
  mis-normalize. Acceptable for v1 (the corpus uses explicit registries for
  first-party images); revisit if a private default-registry config appears.
- **Owning-repo derivation is host-shaped.** "first three path segments" works
  for `github.com`/`gitlab.com`; vanity import paths (`example.org/x/y`) and
  `gopkg.in` need per-host rules. v1 handles the GitHub-style hosts the corpus
  uses; others fall back to "owning repo unknown" (module identity still works).
- **Unresolvable CI vars.** Secrets and matrix expansions can't be resolved
  statically; those refs are marked unresolvable and excluded from strict
  matching. A workflow that builds an image whose whole name is a secret is
  invisible — an accepted blind spot, noted so it isn't mistaken for a bug.
