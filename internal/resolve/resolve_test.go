package resolve

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/agentic-research/assay/internal/artifact"
	"github.com/agentic-research/assay/internal/extract"
)

// producer is a test helper: a producer fact for the given kind/ref observed in
// rootFile (the file path stands in for which scan root it came from).
func producer(kind artifact.Kind, ref, rootFile string) artifact.Producer {
	return artifact.Producer{
		Identity:   artifact.NewIdentity(kind, ref),
		Provenance: artifact.Provenance{File: rootFile, Line: 1},
	}
}

func consumer(kind artifact.Kind, ref, rootFile string) artifact.Consumer {
	return artifact.Consumer{
		Identity:   artifact.NewIdentity(kind, ref),
		Provenance: artifact.Provenance{File: rootFile, Line: 1},
	}
}

// TestResolve_CrossRootEdge: a producer in root A and a consumer in root B with
// the same normalized identity resolve to exactly one cross-root edge. The
// versions differ, so version_match must be recorded as mismatch.
func TestResolve_CrossRootEdge(t *testing.T) {
	facts := &extract.Facts{
		Producers: []artifact.Producer{
			// Root A publishes the image at tag v0.1.0.
			producer(artifact.KindContainerImage, "ghcr.io/agentic-research/rosary:v0.1.0", "rootA/release.yml"),
		},
		Consumers: []artifact.Consumer{
			// Root B consumes it with an implicit/different tag.
			consumer(artifact.KindContainerImage, "ghcr.io/agentic-research/rosary:latest", "rootB/Dockerfile"),
		},
	}

	got := Resolve(facts)

	require.Len(t, got.Resolved, 1, "exactly one resolved cross-root edge")
	assert.Empty(t, got.External)
	assert.Empty(t, got.Dangling)

	edge := got.Resolved[0]
	assert.Equal(t, "rootA/release.yml", edge.Producer.Provenance.File, "producer from root A")
	assert.Equal(t, "rootB/Dockerfile", edge.Consumer.Provenance.File, "consumer from root B")
	assert.Equal(t, VersionMatchMismatch, edge.VersionMatch, "tags differ → mismatch, still resolved")
}

// TestResolve_External: a consumer referencing an id no scanned root produces is
// an external dependency, not an edge.
func TestResolve_External(t *testing.T) {
	facts := &extract.Facts{
		Consumers: []artifact.Consumer{
			consumer(artifact.KindContainerImage, "gcr.io/distroless/static-debian12:latest", "x-ray/Dockerfile"),
		},
	}

	got := Resolve(facts)

	assert.Empty(t, got.Resolved)
	assert.Empty(t, got.Dangling)
	require.Len(t, got.External, 1)
	assert.Equal(t, "gcr.io/distroless/static-debian12:latest", got.External[0].Identity.Ref)
}

// TestResolve_Dangling: a producer no consumer references is a dead-surface
// candidate.
func TestResolve_Dangling(t *testing.T) {
	facts := &extract.Facts{
		Producers: []artifact.Producer{
			producer(artifact.KindGoModule, "github.com/agentic-research/orphan", "orphan/go.mod"),
		},
	}

	got := Resolve(facts)

	assert.Empty(t, got.Resolved)
	assert.Empty(t, got.External)
	require.Len(t, got.Dangling, 1)
	assert.Equal(t, "github.com/agentic-research/orphan", got.Dangling[0].Identity.Ref)
}

// TestResolve_GoModuleVersionNormalization: a `require …@v0.5.5` resolves to a
// module producer regardless of version, since matching is on the version-
// stripped identity key.
func TestResolve_GoModuleVersionNormalization(t *testing.T) {
	facts := &extract.Facts{
		Producers: []artifact.Producer{
			producer(artifact.KindGoModule, "github.com/agentic-research/mache", "mache/go.mod"),
		},
		Consumers: []artifact.Consumer{
			consumer(artifact.KindGoModule, "github.com/agentic-research/mache@v0.5.5", "x-ray/go.mod"),
		},
	}

	got := Resolve(facts)

	require.Len(t, got.Resolved, 1, "require@version resolves to the bare module producer")
	assert.Empty(t, got.External)
	assert.Empty(t, got.Dangling)

	edge := got.Resolved[0]
	assert.Equal(t, "mache/go.mod", edge.Producer.Provenance.File)
	assert.Equal(t, "x-ray/go.mod", edge.Consumer.Provenance.File)
	// Producer has no version, consumer pins v0.5.5 → unknown (cannot compare).
	assert.Equal(t, VersionMatchUnknown, edge.VersionMatch)
}

// TestResolve_GoModuleMajorVersionSuffixKept: the `/v2` major-version path
// suffix is part of module identity and must NOT be stripped, so v2 and v1 of a
// module do not resolve to each other.
func TestResolve_GoModuleMajorVersionSuffixKept(t *testing.T) {
	facts := &extract.Facts{
		Producers: []artifact.Producer{
			producer(artifact.KindGoModule, "github.com/agentic-research/mache/v2", "mache/go.mod"),
		},
		Consumers: []artifact.Consumer{
			consumer(artifact.KindGoModule, "github.com/agentic-research/mache@v0.5.5", "x-ray/go.mod"),
		},
	}

	got := Resolve(facts)

	assert.Empty(t, got.Resolved, "/v2 is a different module identity than the v1 path")
	require.Len(t, got.External, 1)
	require.Len(t, got.Dangling, 1)
}

// TestResolve_VersionMatchExact: identical versions on both sides → exact.
func TestResolve_VersionMatchExact(t *testing.T) {
	facts := &extract.Facts{
		Producers: []artifact.Producer{
			producer(artifact.KindContainerImage, "ghcr.io/agentic-research/rosary:v0.1.0", "rootA/release.yml"),
		},
		Consumers: []artifact.Consumer{
			consumer(artifact.KindContainerImage, "ghcr.io/agentic-research/rosary:v0.1.0", "rootB/Dockerfile"),
		},
	}

	got := Resolve(facts)
	require.Len(t, got.Resolved, 1)
	assert.Equal(t, VersionMatchExact, got.Resolved[0].VersionMatch)
}

// TestResolve_KindIsolation: the same ref string under different kinds must not
// resolve to each other.
func TestResolve_KindIsolation(t *testing.T) {
	facts := &extract.Facts{
		Producers: []artifact.Producer{
			producer(artifact.KindGoModule, "assay", "go.mod"),
		},
		Consumers: []artifact.Consumer{
			consumer(artifact.KindCLIBinary, "assay", "ci.yml"),
		},
	}

	got := Resolve(facts)
	assert.Empty(t, got.Resolved, "same ref, different kind → no edge")
	assert.Len(t, got.External, 1)
	assert.Len(t, got.Dangling, 1)
}

// TestResolve_OrderDeterminism: repeated runs over the same facts — including
// facts supplied in different input orders — produce identical bucket orderings.
func TestResolve_OrderDeterminism(t *testing.T) {
	producers := []artifact.Producer{
		producer(artifact.KindGoModule, "github.com/agentic-research/mache", "mache/go.mod"),
		producer(artifact.KindGoModule, "github.com/agentic-research/orphan", "orphan/go.mod"),
		producer(artifact.KindContainerImage, "ghcr.io/agentic-research/rosary:v0.1.0", "rosary/release.yml"),
	}
	consumers := []artifact.Consumer{
		consumer(artifact.KindGoModule, "github.com/agentic-research/mache@v0.5.5", "x-ray/go.mod"),
		consumer(artifact.KindContainerImage, "ghcr.io/agentic-research/rosary:latest", "deploy/Dockerfile"),
		consumer(artifact.KindContainerImage, "gcr.io/distroless/static-debian12:latest", "x-ray/Dockerfile"),
	}

	forward := Resolve(&extract.Facts{Producers: producers, Consumers: consumers})

	// Same facts, reversed input order, run repeatedly: outputs must be identical.
	revP := reverseProducers(producers)
	revC := reverseConsumers(consumers)
	for i := 0; i < 50; i++ {
		got := Resolve(&extract.Facts{Producers: revP, Consumers: revC})
		assert.Equal(t, forward, got, "run %d must match the canonical ordering", i)
	}
}

func TestResolve_NilFacts(t *testing.T) {
	got := Resolve(nil)
	assert.Empty(t, got.Resolved)
	assert.Empty(t, got.External)
	assert.Empty(t, got.Dangling)
}

func reverseProducers(in []artifact.Producer) []artifact.Producer {
	out := make([]artifact.Producer, len(in))
	for i, p := range in {
		out[len(in)-1-i] = p
	}
	return out
}

func reverseConsumers(in []artifact.Consumer) []artifact.Consumer {
	out := make([]artifact.Consumer, len(in))
	for i, c := range in {
		out[len(in)-1-i] = c
	}
	return out
}
