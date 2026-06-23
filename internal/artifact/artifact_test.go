package artifact

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestKind_String(t *testing.T) {
	assert.Equal(t, "go_module", KindGoModule.String())
	assert.Equal(t, "go_package_symbol", KindGoPackageSymbol.String())
	assert.Equal(t, "container_image", KindContainerImage.String())
	assert.Equal(t, "cli_binary", KindCLIBinary.String())
}

func TestKind_Valid(t *testing.T) {
	assert.True(t, KindGoModule.Valid())
	assert.True(t, KindContainerImage.Valid())
	assert.False(t, Kind("").Valid())
	assert.False(t, Kind("not_a_kind").Valid())
}

func TestNewIdentity_TrimsRef(t *testing.T) {
	id := NewIdentity(KindGoModule, "  github.com/agentic-research/assay  ")
	assert.Equal(t, "github.com/agentic-research/assay", id.Ref)
	assert.Equal(t, KindGoModule, id.Kind)
}

func TestIdentity_Equal_EquivalentRefs(t *testing.T) {
	// Two refs that should produce equal identities once constructed (here:
	// equivalent up to surrounding whitespace, the only normalization this
	// package applies — formal canonicalization policy is decided in T0-3).
	a := NewIdentity(KindGoModule, "github.com/agentic-research/assay")
	b := NewIdentity(KindGoModule, "  github.com/agentic-research/assay  ")

	assert.True(t, a.Equal(b))
	assert.Equal(t, a, b) // value equality via ==
	assert.Equal(t, a.Key(), b.Key())
}

func TestIdentity_DistinctAcrossKinds_SameRef(t *testing.T) {
	// Same ref string under different kinds must never be equal or collide.
	mod := NewIdentity(KindGoModule, "assay")
	bin := NewIdentity(KindCLIBinary, "assay")

	assert.False(t, mod.Equal(bin))
	assert.NotEqual(t, mod, bin)
	assert.NotEqual(t, mod.Key(), bin.Key())
}

func TestIdentity_Key_Stable(t *testing.T) {
	id := NewIdentity(KindContainerImage, "ghcr.io/agentic-research/mache:0.3")
	// Key is deterministic and embeds the kind so it is collision-safe across kinds.
	assert.Equal(t, id.Key(), id.Key())
	assert.Contains(t, id.Key(), KindContainerImage.String())
	assert.Contains(t, id.Key(), "ghcr.io/agentic-research/mache:0.3")
}

func TestIdentity_Key_NoCrossKindForgery(t *testing.T) {
	// A ref must not be able to forge another kind's key by embedding the
	// separator/kind text: the kind is keyed independently of the ref bytes.
	forge := NewIdentity(KindGoModule, "container_image\x00ghcr.io/x:1")
	real := NewIdentity(KindContainerImage, "ghcr.io/x:1")
	assert.NotEqual(t, real.Key(), forge.Key())
}

func TestIdentity_Zero(t *testing.T) {
	var id Identity
	assert.True(t, id.IsZero())
	assert.False(t, NewIdentity(KindGoModule, "x").IsZero())
}

func TestArtifact_Identity(t *testing.T) {
	a := Artifact{
		Identity: NewIdentity(KindGoPackageSymbol, "github.com/agentic-research/assay/internal/coverage.Compute"),
		Name:     "Compute",
	}
	assert.Equal(t, KindGoPackageSymbol, a.Identity.Kind)
	assert.Equal(t, "Compute", a.Name)
}

func TestProducerConsumer_CarryProvenance(t *testing.T) {
	id := NewIdentity(KindContainerImage, "ghcr.io/agentic-research/mache:0.3")
	prov := Provenance{File: "Dockerfile", Line: 1}

	p := Producer{Identity: id, Provenance: prov}
	c := Consumer{Identity: id, Provenance: Provenance{File: "other/Dockerfile", Line: 1}}

	assert.Equal(t, KindContainerImage, p.Kind())
	assert.Equal(t, KindContainerImage, c.Kind())
	assert.Equal(t, "Dockerfile", p.Provenance.File)
	assert.True(t, p.Identity.Equal(c.Identity)) // resolution keys on identity
}

func TestEdge_Direction(t *testing.T) {
	id := NewIdentity(KindContainerImage, "ghcr.io/agentic-research/mache:0.3")
	e := Edge{
		Producer: Producer{Identity: id, Provenance: Provenance{File: "a/Dockerfile", Line: 1}},
		Consumer: Consumer{Identity: id, Provenance: Provenance{File: "b/Dockerfile", Line: 4}},
	}
	// An edge is producer → consumer over a shared identity.
	assert.True(t, e.Producer.Identity.Equal(e.Consumer.Identity))
	assert.Equal(t, "a/Dockerfile", e.Producer.Provenance.File)
	assert.Equal(t, "b/Dockerfile", e.Consumer.Provenance.File)
}
