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

// Equal reports whether two identities are the same artifact identity.
func (id Identity) Equal(other Identity) bool {
	return id == other
}

// IsZero reports whether id is the zero Identity (no kind, no ref).
func (id Identity) IsZero() bool {
	return id == Identity{}
}
