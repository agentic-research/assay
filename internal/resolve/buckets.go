package resolve

import (
	"github.com/agentic-research/assay/internal/artifact"
)

// VersionMatch records how a resolved edge's producer and consumer versions
// compare. The resolver matches on the version-stripped identity key, so an
// edge can resolve even when versions differ; this annotation surfaces that
// delta rather than hiding it (see decision 0002, "Matching policy").
type VersionMatch string

const (
	// VersionMatchExact means producer and consumer carry the same version
	// component (or both carry none).
	VersionMatchExact VersionMatch = "exact"
	// VersionMatchMismatch means both sides carry a version but they differ —
	// a real, reportable signal (e.g. a consumer pins an old tag).
	VersionMatchMismatch VersionMatch = "mismatch"
	// VersionMatchUnknown means at least one side has no version to compare,
	// so the strict relationship cannot be determined.
	VersionMatchUnknown VersionMatch = "unknown"
)

// identityKey is the version-stripped match key the resolver groups on. It pairs
// the artifact Kind with the kind-specific identity reference (the ref with any
// tag/digest/module-version removed), so two references to the same artifact at
// different versions land in the same group while references under different
// kinds stay distinct.
//
// It is derived from artifact.Identity.Canonical(), which owns the per-kind
// splitting rules (decision 0002); the resolver does not re-parse refs.
type identityKey struct {
	Kind     artifact.Kind
	Identity string
}

// keyAndVersion decomposes an artifact Identity into the resolver's grouping key
// and the version component to annotate edges with. The split rules live in
// artifact.Identity.Canonical(); this is a thin adapter to the resolver's
// identityKey shape.
func keyAndVersion(id artifact.Identity) (identityKey, string) {
	c := id.Canonical()
	return identityKey{Kind: c.Kind, Identity: c.IdentityKey}, c.Version
}

// classifyVersionMatch reports how a producer version and consumer version
// compare for an already-resolved edge.
func classifyVersionMatch(producerVersion, consumerVersion string) VersionMatch {
	if producerVersion == "" || consumerVersion == "" {
		if producerVersion == "" && consumerVersion == "" {
			return VersionMatchExact
		}
		return VersionMatchUnknown
	}
	if producerVersion == consumerVersion {
		return VersionMatchExact
	}
	return VersionMatchMismatch
}
