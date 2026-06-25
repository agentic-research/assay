package cargo

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/agentic-research/assay/internal/artifact"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// findConsumer returns the consumer whose identity matches (kind, ref), failing
// the test if none is present. Provenance is asserted by the caller.
func findConsumer(t *testing.T, cons []artifact.Consumer, ref string) artifact.Consumer {
	t.Helper()
	want := artifact.NewIdentity(artifact.KindCargoCrate, ref)
	for _, c := range cons {
		if c.Identity == want {
			return c
		}
	}
	t.Fatalf("no consumer found for crate %q in %v", ref, cons)
	return artifact.Consumer{}
}

func findProducer(t *testing.T, prods []artifact.Producer, ref string) artifact.Producer {
	t.Helper()
	want := artifact.NewIdentity(artifact.KindCargoCrate, ref)
	for _, p := range prods {
		if p.Identity == want {
			return p
		}
	}
	t.Fatalf("no producer found for crate %q in %v", ref, prods)
	return artifact.Producer{}
}

func hasConsumer(cons []artifact.Consumer, ref string) bool {
	want := artifact.NewIdentity(artifact.KindCargoCrate, ref)
	for _, c := range cons {
		if c.Identity == want {
			return true
		}
	}
	return false
}

func hasProducer(prods []artifact.Producer, ref string) bool {
	want := artifact.NewIdentity(artifact.KindCargoCrate, ref)
	for _, p := range prods {
		if p.Identity == want {
			return true
		}
	}
	return false
}

func TestExtractor_AvailableAndName(t *testing.T) {
	ex := New()
	assert.Equal(t, "cargo", ex.Name())
	ok, reason := ex.Available()
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestExtractor_NoCargoTomlYieldsNothing(t *testing.T) {
	ex := New()
	prods, cons, err := ex.Extract(t.TempDir())
	require.NoError(t, err)
	assert.Empty(t, prods)
	assert.Empty(t, cons)
}

// The producer root is the ley-line-open shape: a virtual workspace whose member
// crates carry the [package].name producers. Walking the whole root discovers
// every member, so leyline-cas-ffi (the crate cloister depends on) is produced.
func TestExtractor_ProducerWorkspaceMembers(t *testing.T) {
	root := filepath.Join("testdata", "producer")
	ex := New()
	prods, cons, err := ex.Extract(root)
	require.NoError(t, err)

	ffi := findProducer(t, prods, "leyline-cas-ffi")
	assert.Equal(t, filepath.Join(root, "rs", "cas-ffi", "Cargo.toml"), ffi.Provenance.File)
	assert.Equal(t, 2, ffi.Provenance.Line)

	// The sibling workspace member is produced too, from its own manifest.
	core := findProducer(t, prods, "leyline-core")
	assert.Equal(t, filepath.Join(root, "rs", "core", "Cargo.toml"), core.Provenance.File)

	// The in-workspace path dependency is a consumer keyed by crate name.
	leylineCore := findConsumer(t, cons, "leyline-core")
	assert.Equal(t, filepath.Join(root, "rs", "cas-ffi", "Cargo.toml"), leylineCore.Provenance.File)
	assert.Equal(t, 9, leylineCore.Provenance.Line)
}

// The consumer root is the cloister shape: cloister-cas depends on leyline-cas-ffi
// via a cross-repo git dependency (with a `package` restatement), on a sibling via
// a path dependency, and on blake3 as a dev-dependency.
func TestExtractor_ConsumerDependencies(t *testing.T) {
	root := filepath.Join("testdata", "consumer")
	casManifest := filepath.Join(root, "rs", "crates", "cas", "Cargo.toml")
	ex := New()
	prods, cons, err := ex.Extract(root)
	require.NoError(t, err)

	// The crate itself is a producer.
	cas := findProducer(t, prods, "cloister-cas")
	assert.Equal(t, casManifest, cas.Provenance.File)
	assert.Equal(t, 2, cas.Provenance.Line)

	// The cross-repo git dependency, emitted by its crate name so it matches the
	// producer in the producer root.
	ffi := findConsumer(t, cons, "leyline-cas-ffi")
	assert.Equal(t, casManifest, ffi.Provenance.File)
	assert.Equal(t, 11, ffi.Provenance.Line)

	// The local path dependency is a consumer too (it resolves to the sibling
	// member's producer within this same root).
	helper := findConsumer(t, cons, "local-helper")
	assert.Equal(t, casManifest, helper.Provenance.File)
	assert.Equal(t, 13, helper.Provenance.Line)

	// dev-dependencies count as consumers.
	blake := findConsumer(t, cons, "blake3")
	assert.Equal(t, 16, blake.Provenance.Line)

	// The sibling member is produced, so the path dep above resolves in-root.
	assert.True(t, hasProducer(prods, "local-helper"))
}

// The producer and consumer roots are scanned independently; the crate the
// consumer depends on and the crate the producer declares must share an identity,
// which is exactly what lets the resolver draw the cross-root edge.
func TestExtractor_ProducerConsumerIdentitiesMatch(t *testing.T) {
	ex := New()
	prods, _, err := ex.Extract(filepath.Join("testdata", "producer"))
	require.NoError(t, err)
	_, cons, err := ex.Extract(filepath.Join("testdata", "consumer"))
	require.NoError(t, err)

	producer := findProducer(t, prods, "leyline-cas-ffi")
	consumer := findConsumer(t, cons, "leyline-cas-ffi")
	assert.Equal(t, producer.Identity, consumer.Identity,
		"cross-root edge needs the producer and consumer identities to match")
}

// Every dependency form Cargo allows must be observed: a registry string dep, an
// inline-table dep, a [deps.<crate>] subtable with a `package` rename (the rename
// is the identity, not the alias), a [target.<cfg>.dependencies] platform dep, and
// a build-dependency.
func TestExtractor_DependencyForms(t *testing.T) {
	root := filepath.Join("testdata", "forms")
	ex := New()
	prods, cons, err := ex.Extract(root)
	require.NoError(t, err)

	assert.True(t, hasProducer(prods, "forms-crate"))

	serde := findConsumer(t, cons, "serde")
	assert.Equal(t, 6, serde.Provenance.Line)

	// The subtable's `package` rename is the crate identity; the alias is not
	// emitted.
	renamed := findConsumer(t, cons, "real-crate")
	assert.Equal(t, 11, renamed.Provenance.Line, "subtable dep anchors to its first key line")
	assert.False(t, hasConsumer(cons, "aliased"), "the local alias must not be a consumer")

	// Platform-specific and build dependencies are consumers.
	nix := findConsumer(t, cons, "nix-only")
	assert.Equal(t, 17, nix.Provenance.Line)
	assert.True(t, hasConsumer(cons, "cc"))
}

// A Cargo.toml under a skipped build/scratch directory must NOT contribute facts:
// Rust's target/ in particular can hold vendored manifests.
func TestExtractor_SkipsBuildDirs(t *testing.T) {
	dir := t.TempDir()
	buildDir := filepath.Join(dir, "target", "package", "vendored-1.0")
	require.NoError(t, os.MkdirAll(buildDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(buildDir, "Cargo.toml"),
		[]byte("[package]\nname = \"vendored\"\n"), 0o644))

	ex := New()
	prods, _, err := ex.Extract(dir)
	require.NoError(t, err)
	assert.False(t, hasProducer(prods, "vendored"),
		"manifest under target/ must be skipped")
}

func TestExtractor_Deterministic(t *testing.T) {
	root := filepath.Join("testdata", "consumer")
	ex := New()
	first, firstCons, err := ex.Extract(root)
	require.NoError(t, err)
	for i := 0; i < 5; i++ {
		again, againCons, err := ex.Extract(root)
		require.NoError(t, err)
		assert.Equal(t, first, again)
		assert.Equal(t, firstCons, againCons)
	}
}
