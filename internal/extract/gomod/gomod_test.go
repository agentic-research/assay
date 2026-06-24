package gomod

import (
	"path/filepath"
	"testing"

	"github.com/agentic-research/assay/internal/artifact"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// findConsumer returns the consumer whose identity matches (kind, ref), failing
// the test if none is present. Provenance is asserted by the caller.
func findConsumer(t *testing.T, cons []artifact.Consumer, kind artifact.Kind, ref string) artifact.Consumer {
	t.Helper()
	for _, c := range cons {
		if c.Identity == artifact.NewIdentity(kind, ref) {
			return c
		}
	}
	t.Fatalf("no consumer found for %s %q in %v", kind, ref, cons)
	return artifact.Consumer{}
}

func findProducer(t *testing.T, prods []artifact.Producer, kind artifact.Kind, ref string) artifact.Producer {
	t.Helper()
	for _, p := range prods {
		if p.Identity == artifact.NewIdentity(kind, ref) {
			return p
		}
	}
	t.Fatalf("no producer found for %s %q in %v", kind, ref, prods)
	return artifact.Producer{}
}

func hasConsumer(cons []artifact.Consumer, kind artifact.Kind, ref string) bool {
	want := artifact.NewIdentity(kind, ref)
	for _, c := range cons {
		if c.Identity == want {
			return true
		}
	}
	return false
}

func TestExtractor_AvailableAndName(t *testing.T) {
	ex := New()
	assert.Equal(t, "gomod", ex.Name())
	ok, reason := ex.Available()
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestExtractor_NoGoModYieldsNothing(t *testing.T) {
	ex := New()
	prods, cons, err := ex.Extract(t.TempDir())
	require.NoError(t, err)
	assert.Empty(t, prods)
	assert.Empty(t, cons)
}

// x-ray's go.mod is the motivating cross-repo case: a module that requires
// github.com/agentic-research/mache and carries a COMMENTED replace pointing at
// ../mache (local-dev cross-root link).
func TestExtractor_XRay(t *testing.T) {
	root := filepath.Join("testdata", "xray")
	ex := New()
	prods, cons, err := ex.Extract(root)
	require.NoError(t, err)

	// Producer: the module declaration, with its owning-repo identity. x-ray's
	// module path is already host/owner/repo, so both identities coincide.
	mod := findProducer(t, prods, artifact.KindGoModule, "github.com/agentic-research/x-ray")
	assert.Equal(t, filepath.Join(root, "go.mod"), mod.Provenance.File)
	assert.Equal(t, 1, mod.Provenance.Line)

	// Consumer: require of mache, version stripped to the bare module path.
	mache := findConsumer(t, cons, artifact.KindGoModule, "github.com/agentic-research/mache")
	assert.Equal(t, filepath.Join(root, "go.mod"), mache.Provenance.File)
	assert.Equal(t, 6, mache.Provenance.Line)

	// Pseudo-version require is stripped to the bare path too.
	assert.True(t, hasConsumer(cons, artifact.KindGoModule, "github.com/tmc/it2"))

	// Indirect requires are still consumers (the file declares the dependency).
	assert.True(t, hasConsumer(cons, artifact.KindGoModule, "golang.org/x/sys"))

	// The COMMENTED replace encodes a local-dev cross-root link and MUST be
	// emitted as a consumer of the left-hand module path, with the comment's
	// line as provenance.
	commented := false
	for _, c := range cons {
		if c.Identity == artifact.NewIdentity(artifact.KindGoModule, "github.com/agentic-research/mache") &&
			c.Provenance.Line == 17 {
			commented = true
		}
	}
	assert.True(t, commented, "commented replace of mache => ../mache must be emitted at go.mod:17")
}

// A subdir module path is deeper than its repo: it must emit BOTH the
// full-module-path identity AND the owning-repo identity (per decision 0002).
func TestExtractor_SubdirModuleEmitsBothIdentities(t *testing.T) {
	root := filepath.Join("testdata", "subdir")
	ex := New()
	prods, _, err := ex.Extract(root)
	require.NoError(t, err)

	full := findProducer(t, prods, artifact.KindGoModule,
		"github.com/agentic-research/ley-line-open/clients/go/leyline-schema")
	assert.Equal(t, 1, full.Provenance.Line)

	owning := findProducer(t, prods, artifact.KindGoModule,
		"github.com/agentic-research/ley-line-open")
	// The owning-repo identity is derived from the same module declaration.
	assert.Equal(t, 1, owning.Provenance.Line)
	assert.Equal(t, filepath.Join(root, "go.mod"), owning.Provenance.File)
}

// An ACTIVE replace is a cross-root link too: the left-hand module path becomes
// a consumer. +incompatible and /vN are handled per decision 0002.
func TestExtractor_ActiveReplaceAndVersionForms(t *testing.T) {
	root := filepath.Join("testdata", "replace")
	ex := New()
	_, cons, err := ex.Extract(root)
	require.NoError(t, err)

	// /vN major-version suffix is part of identity and is KEPT.
	billy := findConsumer(t, cons, artifact.KindGoModule, "github.com/go-git/go-billy/v5")
	assert.Equal(t, 6, billy.Provenance.Line)

	// +incompatible is stripped from the version, not the path; the path is
	// unaffected by version stripping.
	assert.True(t, hasConsumer(cons, artifact.KindGoModule, "github.com/incompat/lib"))

	// The active replace's left side is emitted as a consumer at its own line.
	replaced := false
	for _, c := range cons {
		if c.Identity == artifact.NewIdentity(artifact.KindGoModule, "github.com/go-git/go-billy/v5") &&
			c.Provenance.Line == 10 {
			replaced = true
		}
	}
	assert.True(t, replaced, "active replace of go-billy must be emitted at go.mod:10")
}

func TestExtractor_Deterministic(t *testing.T) {
	root := filepath.Join("testdata", "xray")
	ex := New()
	first, _, err := ex.Extract(root)
	require.NoError(t, err)
	for i := 0; i < 5; i++ {
		again, _, err := ex.Extract(root)
		require.NoError(t, err)
		assert.Equal(t, first, again)
	}
}
