package dockerfile

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/agentic-research/assay/internal/artifact"
	"github.com/agentic-research/assay/internal/extract"
)

// compile-time assertion: Extractor satisfies the extract contract.
var _ extract.Extractor = Extractor{}

func TestExtractor_AvailableAlwaysTrue(t *testing.T) {
	ok, reason := Extractor{}.Available()
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestExtractor_Name(t *testing.T) {
	assert.Equal(t, "dockerfile", Extractor{}.Name())
}

// img builds the container-image identity for a normalized reference, matching
// what the extractor emits.
func img(normalizedRef string) artifact.Identity {
	return artifact.NewIdentity(artifact.KindContainerImage, normalizedRef)
}

// consumerIdentities collects the identities of consumer facts for assertions
// that care about *what* was consumed, not provenance.
func consumerIdentities(cs []artifact.Consumer) []artifact.Identity {
	out := make([]artifact.Identity, len(cs))
	for i, c := range cs {
		out[i] = c.Identity
	}
	return out
}

func producerIdentities(ps []artifact.Producer) []artifact.Identity {
	out := make([]artifact.Identity, len(ps))
	for i, p := range ps {
		out[i] = p.Identity
	}
	return out
}

// findConsumer returns the consumer with the given normalized ref, failing the
// test if it is absent.
func findConsumer(t *testing.T, cs []artifact.Consumer, normalizedRef string) artifact.Consumer {
	t.Helper()
	want := img(normalizedRef)
	for _, c := range cs {
		if c.Identity == want {
			return c
		}
	}
	t.Fatalf("no consumer for %q; got %v", normalizedRef, consumerIdentities(cs))
	return artifact.Consumer{}
}

// TestExtractor_MultiStage models x-ray's Dockerfile: a named `builder` stage on
// an external golang base, a final stage on an external distroless base, and a
// `COPY --from=builder`. The COPY must resolve to the *internal* stage (not an
// external image), the two FROM bases must be external consumers, and the build
// target must be a producer — all with line provenance.
func TestExtractor_MultiStage(t *testing.T) {
	root := filepath.Join("testdata", "multistage")
	producers, consumers, err := Extractor{}.Extract(root)
	require.NoError(t, err)

	// Exactly two external image consumers: the two FROM bases. The
	// `COPY --from=builder` names the local stage, so it is NOT a consumer.
	assert.ElementsMatch(t, []artifact.Identity{
		img("docker.io/library/golang"),
		img("gcr.io/distroless/static-debian12"),
	}, consumerIdentities(consumers),
		"COPY --from=builder must resolve to the internal stage, not an external image")

	// The golang base FROM is on line 1; the distroless base FROM on line 7.
	golang := findConsumer(t, consumers, "docker.io/library/golang")
	assert.Equal(t, "Dockerfile", golang.Provenance.File)
	assert.Equal(t, 1, golang.Provenance.Line)

	distroless := findConsumer(t, consumers, "gcr.io/distroless/static-debian12")
	assert.Equal(t, 7, distroless.Provenance.Line)

	// Producers: the named `builder` stage and the final build target.
	assert.ElementsMatch(t, []artifact.Identity{
		img("Dockerfile#builder"),
		img("Dockerfile"),
	}, producerIdentities(producers))

	for _, p := range producers {
		assert.Equal(t, "Dockerfile", p.Provenance.File)
		assert.NotZero(t, p.Provenance.Line, "producer %v missing line provenance", p.Identity)
	}
}

// TestExtractor_ArgSubstitution covers ARG-interpolated FROM bases, an automatic
// platform arg ($BUILDPLATFORM) that must not leak a literal, and a
// `COPY --from=<external image>` that IS a real consumer (it names no local
// stage).
func TestExtractor_ArgSubstitution(t *testing.T) {
	root := filepath.Join("testdata", "argsubst")
	_, consumers, err := Extractor{}.Extract(root)
	require.NoError(t, err)

	got := consumerIdentities(consumers)
	assert.Contains(t, got, img("docker.io/library/golang"),
		"FROM golang:${GO_VERSION} should interpolate the ARG and normalize")
	assert.Contains(t, got, img("gcr.io/distroless/static-debian12"),
		"FROM ${BASE} should interpolate the meta-arg base image")
	assert.Contains(t, got, img("docker.io/library/busybox"),
		"COPY --from of an external image is a real consumer")

	// The builder stage is local, so `COPY --from=builder` is NOT a consumer.
	assert.NotContains(t, got, img("docker.io/library/builder"))
}

// TestExtractor_SimpleBareImage checks the bare-name normalization path:
// `FROM alpine` becomes docker.io/library/alpine.
func TestExtractor_SimpleBareImage(t *testing.T) {
	root := filepath.Join("testdata", "simple")
	producers, consumers, err := Extractor{}.Extract(root)
	require.NoError(t, err)

	assert.Equal(t, []artifact.Identity{img("docker.io/library/alpine")}, consumerIdentities(consumers))
	// A single-stage Dockerfile yields exactly one producer: its build target.
	assert.Equal(t, []artifact.Identity{img("Dockerfile")}, producerIdentities(producers))
}

func TestExtractor_NoDockerfilesYieldsNoFacts(t *testing.T) {
	dir := t.TempDir()
	producers, consumers, err := Extractor{}.Extract(dir)
	require.NoError(t, err)
	assert.Empty(t, producers)
	assert.Empty(t, consumers)
}

func TestExtractor_DeterministicAcrossRuns(t *testing.T) {
	root := filepath.Join("testdata", "multistage")
	first, firstC, err := Extractor{}.Extract(root)
	require.NoError(t, err)
	for i := 0; i < 5; i++ {
		again, againC, err := Extractor{}.Extract(root)
		require.NoError(t, err)
		assert.Equal(t, first, again)
		assert.Equal(t, firstC, againC)
	}
}

func TestIsDockerfile(t *testing.T) {
	tests := map[string]bool{
		"Dockerfile":        true,
		"Containerfile":     true,
		"Dockerfile.dev":    true,
		"app.Dockerfile":    true,
		"dockerfile":        false, // lower-case is not the convention
		"Dockerfile.bak.go": true,  // still a Dockerfile.* variant
		"README.md":         false,
		"main.go":           false,
	}
	for name, want := range tests {
		assert.Equalf(t, want, isDockerfile(name), "isDockerfile(%q)", name)
	}
}
