package resolve

import (
	"strings"

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
type identityKey struct {
	Kind     artifact.Kind
	Identity string
}

// identityRefOf splits an artifact reference into its version-stripped identity
// reference and its version component, applying the per-kind rules from decision
// 0002. The facts the resolver consumes carry only Identity{Kind, Ref}, so the
// version is recovered from the ref here rather than read from a dedicated field.
//
// Kinds with no version concept (CLI binaries, Go package symbols) return the
// whole ref as identity and an empty version.
func identityRefOf(kind artifact.Kind, ref string) (identity, version string) {
	switch kind {
	case artifact.KindGoModule:
		return splitGoModule(ref)
	case artifact.KindContainerImage:
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
