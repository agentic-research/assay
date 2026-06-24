package dockerfile

import "strings"

// defaultRegistry is the registry assumed for a reference with no host, per the
// Docker convention and ADR-0002.
const defaultRegistry = "docker.io"

// officialNamespace is the Docker Hub namespace for official ("library") images,
// so a bare name like `golang` resolves to `docker.io/library/golang`.
const officialNamespace = "library"

// NormalizeImageRef folds a raw image reference to its ADR-0002 *identity key*:
// `registry/repository`, with the tag and digest stripped. The identity key is
// what the resolver matches on, so an image pulled as `golang:1.26` here matches
// the same image published as `docker.io/library/golang:latest` elsewhere — the
// version delta is recorded separately, not used to split the artifact.
//
// Normalization, per ADR-0002:
//   - no registry host (first segment has no `.`/`:` and is not `localhost`) ⇒
//     default to docker.io; a bare name additionally gets the `library/`
//     namespace.
//   - the registry host is lowercased (it is a DNS name); the repository path is
//     left case-sensitive per the OCI spec.
//   - the `:tag` and `@digest` are dropped from the identity key.
//
// An empty or whitespace-only ref normalizes to "".
func NormalizeImageRef(raw string) string {
	ref := strings.TrimSpace(raw)
	if ref == "" {
		return ""
	}
	ref = stripDigest(ref)
	ref = stripTag(ref)

	registry, repository := splitRegistry(ref)
	registry = strings.ToLower(registry)
	if registry == defaultRegistry && !strings.Contains(repository, "/") {
		repository = officialNamespace + "/" + repository
	}
	return registry + "/" + repository
}

// stripDigest removes an `@sha256:...` (or any `@algo:hex`) digest suffix.
func stripDigest(ref string) string {
	if at := strings.IndexByte(ref, '@'); at >= 0 {
		return ref[:at]
	}
	return ref
}

// stripTag removes a trailing `:tag`. It looks only after the final `/` so a
// registry port (`localhost:5000/img`) is never mistaken for a tag.
func stripTag(ref string) string {
	lastSlash := strings.LastIndexByte(ref, '/')
	if colon := strings.LastIndexByte(ref, ':'); colon > lastSlash {
		return ref[:colon]
	}
	return ref
}

// splitRegistry separates the registry host from the repository path, supplying
// the default registry when the reference omits a host. A first segment is a
// registry host iff it contains a `.` or `:` or is exactly `localhost`.
func splitRegistry(ref string) (registry, repository string) {
	if i := strings.IndexByte(ref, '/'); i >= 0 {
		first := ref[:i]
		if strings.ContainsAny(first, ".:") || first == "localhost" {
			return first, ref[i+1:]
		}
	}
	return defaultRegistry, ref
}
