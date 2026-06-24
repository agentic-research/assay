package artifact

import "strings"

// keySep separates the kind from the ref in a canonical key. NUL cannot appear
// in any realistic artifact reference, so a Ref cannot forge another kind's key
// by embedding the separator: the kind is keyed independently of the ref bytes.
const keySep = "\x00"

// Identity is the stable global identity of an artifact: a canonical reference
// string keyed by Kind. It is repo-agnostic — resolution matches Consumers to
// Producers on Identity alone, so two equivalent references must produce equal
// Identities, and references under differing kinds must stay distinct even when
// the string matches.
//
// Identity is comparable, so equal Identities compare equal with == and can be
// used directly as map keys.
type Identity struct {
	Kind Kind
	Ref  string
}

// NewIdentity constructs an Identity, applying only minimal normalization
// (surrounding-whitespace trimming) so equivalent references converge.
//
// The formal canonicalization policy — how aggressively to normalize image
// tags/digests/registries, module paths, etc. — is being decided in T0-3
// (bead assay-926eb8) and deliberately does not live here. Keep this trivial
// and pluggable; do not hardcode normalization rules in this package.
func NewIdentity(kind Kind, ref string) Identity {
	return Identity{Kind: kind, Ref: strings.TrimSpace(ref)}
}

// Key returns the canonical key Identity resolves on. It embeds the Kind so it
// is collision-safe across kinds: identities with the same Ref but different
// Kinds yield different keys.
func (id Identity) Key() string {
	return id.Kind.String() + keySep + id.Ref
}

// CanonicalID is the resolver-facing decomposition of an Identity per decision
// 0002: the version-stripped IdentityKey the resolver matches on, paired with
// the Version it factored out. It is the single source of truth for how a raw
// Ref splits into identity-vs-version — the kind-specific @-splitting (Go
// modules) and tag/digest rules (container images) live ONLY here, not in the
// resolver.
type CanonicalID struct {
	// Kind is the artifact kind, carried so a CanonicalID is self-describing.
	Kind Kind
	// IdentityKey is the kind-specific, version-stripped reference the resolver
	// groups on: the module path (major-version suffix kept), the image
	// registry/repository (tag and digest removed), or the whole ref for kinds
	// with no version concept.
	IdentityKey string
	// Version is the component factored out of the ref: a module version, an
	// image tag, an image digest, or "" when the ref carries none.
	Version string
}

// Canonical decomposes the Identity into its version-stripped IdentityKey and
// Version, applying the per-kind rules from decision 0002. This is the one place
// that knows go_module @-splitting (keeping the /vN major-version path suffix)
// and image registry/tag/digest rules; everything that needs the match key or
// the version reads it from here.
//
// Kinds with no version concept (CLI binaries, Go package symbols) return the
// whole ref as the IdentityKey and an empty Version.
func (id Identity) Canonical() CanonicalID {
	identity, version := splitRef(id.Kind, id.Ref)
	return CanonicalID{Kind: id.Kind, IdentityKey: identity, Version: version}
}

// splitRef splits a raw artifact reference into its version-stripped identity
// reference and its version component, dispatching on kind.
func splitRef(kind Kind, ref string) (identity, version string) {
	switch kind {
	case KindGoModule:
		return splitGoModule(ref)
	case KindContainerImage:
		return splitContainerImage(ref)
	default:
		return ref, ""
	}
}

// splitGoModule strips a Go module's version, returning the bare module path and
// the version. A module ref is `path` or `path@version` (e.g.
// `github.com/agentic-research/mache@v0.5.5`); the major-version path suffix
// (`.../v2`) is part of identity and is NOT stripped, since `@` separates the
// version while `/v2` is a path segment.
func splitGoModule(ref string) (identity, version string) {
	path, ver, found := strings.Cut(ref, "@")
	if !found {
		return ref, ""
	}
	return path, ver
}

// splitContainerImage strips a container image's tag or digest, returning the
// `registry/repository` identity and the version (digest beats tag). A ref is
// `registry/repository[:tag][@digest]`; an `@sha256:...` digest pins content and
// is the strict version, otherwise a trailing `:tag` is the version. A `:port`
// in the registry host is not a tag, so only a colon after the final `/` counts.
func splitContainerImage(ref string) (identity, version string) {
	if repo, digest, found := strings.Cut(ref, "@"); found {
		return repo, digest
	}
	slash := strings.LastIndex(ref, "/")
	colon := strings.LastIndex(ref, ":")
	if colon > slash {
		return ref[:colon], ref[colon+1:]
	}
	return ref, ""
}

// Equal reports whether two identities are the same artifact identity.
func (id Identity) Equal(other Identity) bool {
	return id == other
}

// IsZero reports whether id is the zero Identity (no kind, no ref).
func (id Identity) IsZero() bool {
	return id == Identity{}
}
