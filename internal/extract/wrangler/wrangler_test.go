package wrangler

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/agentic-research/assay/internal/artifact"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// findConsumer returns the service consumer matching ref, failing if absent.
func findConsumer(t *testing.T, cons []artifact.Consumer, ref string) artifact.Consumer {
	t.Helper()
	want := artifact.NewIdentity(artifact.KindService, ref)
	for _, c := range cons {
		if c.Identity == want {
			return c
		}
	}
	t.Fatalf("no consumer found for service %q in %v", ref, cons)
	return artifact.Consumer{}
}

func findProducer(t *testing.T, prods []artifact.Producer, ref string) artifact.Producer {
	t.Helper()
	want := artifact.NewIdentity(artifact.KindService, ref)
	for _, p := range prods {
		if p.Identity == want {
			return p
		}
	}
	t.Fatalf("no producer found for service %q in %v", ref, prods)
	return artifact.Producer{}
}

func hasConsumer(cons []artifact.Consumer, ref string) bool {
	want := artifact.NewIdentity(artifact.KindService, ref)
	for _, c := range cons {
		if c.Identity == want {
			return true
		}
	}
	return false
}

func hasProducer(prods []artifact.Producer, ref string) bool {
	want := artifact.NewIdentity(artifact.KindService, ref)
	for _, p := range prods {
		if p.Identity == want {
			return true
		}
	}
	return false
}

func TestExtractor_AvailableAndName(t *testing.T) {
	ex := New()
	assert.Equal(t, "wrangler", ex.Name())
	ok, reason := ex.Available()
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestExtractor_NoWranglerYieldsNothing(t *testing.T) {
	ex := New()
	prods, cons, err := ex.Extract(t.TempDir())
	require.NoError(t, err)
	assert.Empty(t, prods)
	assert.Empty(t, cons)
}

// The producer root is the notme shape: a wrangler.toml declaring the
// `notme-bot` worker. Its top-level name is the service producer.
func TestExtractor_Producer(t *testing.T) {
	root := filepath.Join("testdata", "producer")
	ex := New()
	prods, _, err := ex.Extract(root)
	require.NoError(t, err)

	notme := findProducer(t, prods, "notme-bot")
	assert.Equal(t, filepath.Join(root, "wrangler.toml"), notme.Provenance.File)
	assert.Equal(t, 1, notme.Provenance.Line)
}

// The consumer root is the cloister shape: a worker named `cloister` that binds
// to the `notme-bot` service. The worker is a producer; the bound service is a
// consumer keyed by the bound service's name (not the binding alias).
func TestExtractor_Consumer(t *testing.T) {
	root := filepath.Join("testdata", "consumer")
	manifest := filepath.Join(root, "wrangler.toml")
	ex := New()
	prods, cons, err := ex.Extract(root)
	require.NoError(t, err)

	// The worker this config defines is a producer.
	cloister := findProducer(t, prods, "cloister")
	assert.Equal(t, manifest, cloister.Provenance.File)
	assert.Equal(t, 1, cloister.Provenance.Line)

	// The service binding is a consumer keyed by the bound service's name.
	notme := findConsumer(t, cons, "notme-bot")
	assert.Equal(t, manifest, notme.Provenance.File)

	// Durable Object bindings are same-worker classes, not cross-worker service
	// bindings: their class names must never become service consumers.
	assert.False(t, hasConsumer(cons, "BeadStore"),
		"a durable_objects class binding must not be a service consumer")
	// The binding alias is not an identity either.
	assert.False(t, hasConsumer(cons, "NOTME"),
		"the binding alias must not be a consumer; the bound service name is")
}

// The producer and consumer roots are scanned independently; the service the
// consumer binds to and the service the producer declares must share an
// identity, which is exactly what lets the resolver draw the cross-root edge.
func TestExtractor_ProducerConsumerIdentitiesMatch(t *testing.T) {
	ex := New()
	prods, _, err := ex.Extract(filepath.Join("testdata", "producer"))
	require.NoError(t, err)
	_, cons, err := ex.Extract(filepath.Join("testdata", "consumer"))
	require.NoError(t, err)

	producer := findProducer(t, prods, "notme-bot")
	consumer := findConsumer(t, cons, "notme-bot")
	assert.Equal(t, producer.Identity, consumer.Identity,
		"cross-root edge needs the producer and consumer identities to match")
}

// Both binding forms Cloudflare allows must be observed: the canonical
// [[services]] array-of-tables and the inline `services = [ { ... } ]` array,
// including under an [env.<name>] override.
func TestExtractor_BindingForms(t *testing.T) {
	root := filepath.Join("testdata", "forms")
	ex := New()
	prods, cons, err := ex.Extract(root)
	require.NoError(t, err)

	assert.True(t, hasProducer(prods, "forms-worker"))

	// Array-of-tables bindings.
	assert.True(t, hasConsumer(cons, "auth-service"))
	assert.True(t, hasConsumer(cons, "billing-service"),
		"an entrypoint on a service binding must not change its identity")

	// Inline-array binding under an environment override.
	assert.True(t, hasConsumer(cons, "queue-service"))
}

// A wrangler.toml under a skipped build/dependency directory must NOT contribute
// facts.
func TestExtractor_SkipsBuildDirs(t *testing.T) {
	dir := t.TempDir()
	skip := filepath.Join(dir, "node_modules", "pkg")
	require.NoError(t, os.MkdirAll(skip, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(skip, "wrangler.toml"),
		[]byte("name = \"vendored\"\n"), 0o644))

	ex := New()
	prods, _, err := ex.Extract(dir)
	require.NoError(t, err)
	assert.False(t, hasProducer(prods, "vendored"),
		"wrangler.toml under node_modules/ must be skipped")
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
