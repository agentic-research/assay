package ci

import (
	"path/filepath"
	"testing"

	"github.com/agentic-research/assay/internal/artifact"
	"github.com/agentic-research/assay/internal/extract"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The CI extractor satisfies the Extractor contract.
var _ extract.Extractor = (*Extractor)(nil)

func TestExtractor_NameAndAlwaysAvailable(t *testing.T) {
	ex := New()
	assert.Equal(t, "ci", ex.Name())

	ok, reason := ex.Available()
	assert.True(t, ok)
	assert.Empty(t, reason)
}

// findProducer returns the first producer whose canonical ref matches, and
// asserts exactly one exists.
func findProducer(t *testing.T, producers []artifact.Producer, kind artifact.Kind, ref string) artifact.Producer {
	t.Helper()
	var matches []artifact.Producer
	for _, p := range producers {
		if p.Identity.Kind == kind && p.Identity.Ref == ref {
			matches = append(matches, p)
		}
	}
	require.Lenf(t, matches, 1, "want exactly one %s producer %q, got %d in %#v", kind, ref, len(matches), producers)
	return matches[0]
}

func findConsumer(t *testing.T, consumers []artifact.Consumer, kind artifact.Kind, ref string) artifact.Consumer {
	t.Helper()
	var matches []artifact.Consumer
	for _, c := range consumers {
		if c.Identity.Kind == kind && c.Identity.Ref == ref {
			matches = append(matches, c)
		}
	}
	require.Lenf(t, matches, 1, "want exactly one %s consumer %q, got %d in %#v", kind, ref, len(matches), consumers)
	return matches[0]
}

// The rosary release.yml shape is the load-bearing case: an image declared in
// the workflow env: with ${{ github.repository_owner }} interpolation. The owner
// must be expanded to agentic-research and the producer must carry the line of
// the env declaration.
func TestExtractor_RosaryEnvImageWithOwnerInterpolation(t *testing.T) {
	ex := New()
	producers, consumers, err := ex.Extract("testdata/rosary-like")
	require.NoError(t, err)

	p := findProducer(t, producers, artifact.KindContainerImage, "ghcr.io/agentic-research/rosary")
	wantFile := filepath.Join(".github", "workflows", "release.yml")
	assert.Equal(t, wantFile, p.Provenance.File)
	// IMAGE_NAME is declared on line 20 of the fixture.
	assert.Equal(t, 20, p.Provenance.Line)

	// The cargo-built binary is run in a smoke-test step → CLI binary consumer.
	c := findConsumer(t, consumers, artifact.KindCLIBinary, "rsry")
	assert.Equal(t, wantFile, c.Provenance.File)
	assert.Positive(t, c.Provenance.Line)
}

// A docker push of an env-interpolated image is also a producer, and the
// strict tag (env.TAG → inputs.tag||github.ref_name, unresolved) must not
// pollute the identity ref, which is registry/repository only.
func TestExtractor_PushedImageResolvesToIdentityRef(t *testing.T) {
	ex := New()
	producers, _, err := ex.Extract("testdata/rosary-like")
	require.NoError(t, err)

	// Only one producer for the image identity even though it appears in env
	// AND in two docker commands.
	findProducer(t, producers, artifact.KindContainerImage, "ghcr.io/agentic-research/rosary")
}

// A docker pull of an external image is a container-image consumer; a plain
// command is a CLI-binary consumer.
func TestExtractor_SimpleConsumers(t *testing.T) {
	ex := New()
	_, consumers, err := ex.Extract("testdata/simple")
	require.NoError(t, err)

	pulled := findConsumer(t, consumers, artifact.KindContainerImage, "gcr.io/distroless/static-debian12")
	assert.Equal(t, filepath.Join(".github", "workflows", "ci.yml"), pulled.Provenance.File)
	assert.Positive(t, pulled.Provenance.Line)

	// `go test ./...` → go binary consumer.
	findConsumer(t, consumers, artifact.KindCLIBinary, "go")
}

// Extract over a root with no .github/workflows is a no-op, not an error.
func TestExtractor_NoWorkflowsIsEmpty(t *testing.T) {
	ex := New()
	producers, consumers, err := ex.Extract(t.TempDir())
	require.NoError(t, err)
	assert.Empty(t, producers)
	assert.Empty(t, consumers)
}

// Output is deterministic across repeated passes.
func TestExtractor_Deterministic(t *testing.T) {
	ex := New()
	p1, c1, err := ex.Extract("testdata/rosary-like")
	require.NoError(t, err)
	for range 5 {
		p2, c2, err := ex.Extract("testdata/rosary-like")
		require.NoError(t, err)
		assert.Equal(t, p1, p2)
		assert.Equal(t, c1, c2)
	}
}

// A `VAR=$(cmd …)` command-substitution assignment is not a binary invocation:
// its substituted command must not be mistaken for the leading program.
func TestExtractor_CommandSubstitutionAssignmentNotABinary(t *testing.T) {
	ex := New()
	_, consumers, err := ex.Extract("testdata/rosary-like")
	require.NoError(t, err)
	for _, c := range consumers {
		assert.NotEqual(t, "find", c.Identity.Ref, "command-substitution value leaked as a binary")
	}
}

// The owner used for github.repository_owner interpolation is configurable.
func TestExtractor_OwnerOverride(t *testing.T) {
	ex := New(WithRepositoryOwner("someone-else"))
	producers, _, err := ex.Extract("testdata/rosary-like")
	require.NoError(t, err)
	findProducer(t, producers, artifact.KindContainerImage, "ghcr.io/someone-else/rosary")
}
