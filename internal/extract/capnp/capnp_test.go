package capnp

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/agentic-research/assay/internal/artifact"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func hasConsumer(cons []artifact.Consumer, kind artifact.Kind, ref string) bool {
	want := artifact.NewIdentity(kind, ref)
	for _, c := range cons {
		if c.Identity == want {
			return true
		}
	}
	return false
}

func findConsumer(t *testing.T, cons []artifact.Consumer, kind artifact.Kind, ref string) artifact.Consumer {
	t.Helper()
	want := artifact.NewIdentity(kind, ref)
	for _, c := range cons {
		if c.Identity == want {
			return c
		}
	}
	t.Fatalf("no %s consumer found for %q in %v", kind, ref, cons)
	return artifact.Consumer{}
}

func hasProducer(prods []artifact.Producer, kind artifact.Kind, ref string) bool {
	want := artifact.NewIdentity(kind, ref)
	for _, p := range prods {
		if p.Identity == want {
			return true
		}
	}
	return false
}

func TestExtractor_AvailableAndName(t *testing.T) {
	ex := New()
	assert.Equal(t, "capnp", ex.Name())
	ok, reason := ex.Available()
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestExtractor_NoCapnpYieldsNothing(t *testing.T) {
	ex := New()
	prods, cons, err := ex.Extract(t.TempDir())
	require.NoError(t, err)
	assert.Empty(t, prods)
	assert.Empty(t, cons)
}

// A pure-schema capnp file (struct/enum/annotation/interface defs, no `const`)
// declares no runtime data, so it must contribute zero facts: a struct's `name`
// or `image` field declaration must never be mistaken for a const-data value.
func TestExtractor_SchemaOnlyYieldsNothing(t *testing.T) {
	ex := New()
	prods, cons, err := ex.Extract(filepath.Join("testdata", "schema_only"))
	require.NoError(t, err)
	assert.Empty(t, prods, "schema-only capnp must produce no producers")
	assert.Empty(t, cons, "schema-only capnp must produce no consumers")
}

// The kind of each consumer is DERIVED from its declared form: an external
// bundle's `image = "..."` is a container image; a Worker's `service = "..."`
// binding is a service. cluster.capnp declares the bundle images; config.capnp
// declares the service bindings.
func TestExtractor_DerivesKindFromDeclaredForm(t *testing.T) {
	ex := New()
	_, cons, err := ex.Extract(filepath.Join("testdata", "cluster"))
	require.NoError(t, err)

	// Bundle images → container_image consumers, carrying the declared ref.
	assert.True(t, hasConsumer(cons, artifact.KindContainerImage, "mache:0.8.0"),
		"mache's external bundle image must be a container_image consumer")
	assert.True(t, hasConsumer(cons, artifact.KindContainerImage, "rosary:0.2.0"))
	assert.True(t, hasConsumer(cons, artifact.KindContainerImage, "notme:0.1.0"))

	// Service bindings → service consumers, keyed by the bound service name.
	assert.True(t, hasConsumer(cons, artifact.KindService, "notme-bot"))
	assert.True(t, hasConsumer(cons, artifact.KindService, "mache-mcp"))
	assert.True(t, hasConsumer(cons, artifact.KindService, "rosary-mcp"))

	// An image ref is NEVER emitted as a service, nor a service as an image:
	// cross-kind matching is banned, so the kinds must stay distinct.
	assert.False(t, hasConsumer(cons, artifact.KindService, "mache:0.8.0"))
	assert.False(t, hasConsumer(cons, artifact.KindContainerImage, "notme-bot"))
}

// A config service entry that fronts a worker is the owner's own service
// identity — a producer (config.capnp's `name = "cloister"` with `worker = ...`).
// A durable-object binding's name is NOT a service producer.
func TestExtractor_WorkerBackedServiceIsProducer(t *testing.T) {
	ex := New()
	prods, _, err := ex.Extract(filepath.Join("testdata", "cluster"))
	require.NoError(t, err)

	assert.True(t, hasProducer(prods, artifact.KindService, "cloister"),
		"the worker-fronting config service entry is a service producer")
	// A binding alias / DO namespace name must not be a service producer.
	assert.False(t, hasProducer(prods, artifact.KindService, "BEAD_STORE"))
	assert.False(t, hasProducer(prods, artifact.KindService, "NOTME"))
}

// Provenance pins each fact to the exact source line of its declaring field, so
// a report can point a reader at the precise capnp declaration.
func TestExtractor_Provenance(t *testing.T) {
	ex := New()
	_, cons, err := ex.Extract(filepath.Join("testdata", "cluster"))
	require.NoError(t, err)

	notme := findConsumer(t, cons, artifact.KindService, "notme-bot")
	assert.Equal(t, filepath.Join("testdata", "cluster", "config.capnp"), notme.Provenance.File)
	assert.Positive(t, notme.Provenance.Line, "a fact must carry a real source line")
}

// A capnp file under a skipped build/dependency directory must NOT contribute
// facts.
func TestExtractor_SkipsBuildDirs(t *testing.T) {
	dir := t.TempDir()
	skip := filepath.Join(dir, "node_modules", "pkg")
	require.NoError(t, os.MkdirAll(skip, 0o755))
	const body = "const c :Foo = ( bindings = [ ( service = \"vendored-svc\" ) ] );\n"
	require.NoError(t, os.WriteFile(filepath.Join(skip, "x.capnp"), []byte(body), 0o644))

	ex := New()
	_, cons, err := ex.Extract(dir)
	require.NoError(t, err)
	assert.False(t, hasConsumer(cons, artifact.KindService, "vendored-svc"),
		"a capnp file under node_modules/ must be skipped")
}

func TestExtractor_Deterministic(t *testing.T) {
	root := filepath.Join("testdata", "cluster")
	ex := New()
	firstProd, firstCons, err := ex.Extract(root)
	require.NoError(t, err)
	for i := 0; i < 5; i++ {
		againProd, againCons, err := ex.Extract(root)
		require.NoError(t, err)
		assert.Equal(t, firstProd, againProd)
		assert.Equal(t, firstCons, againCons)
	}
}

// A `#` inside a quoted value must not be treated as a comment, and a comment
// must not yield a field. Guards the parser's quote-aware comment stripping.
func TestParse_CommentAndQuoteHandling(t *testing.T) {
	body := []byte(`
const c :Foo = (
  # service = "commented-out"   <- this is a comment, not a field
  bindings = [
    ( service = "real#hash" ),   # trailing comment ignored
  ],
);
`)
	_, cons, err := parseConst("x.capnp", body)
	require.NoError(t, err)
	assert.True(t, hasConsumer(cons, artifact.KindService, "real#hash"),
		"a # inside a quoted value is part of the value")
	assert.False(t, hasConsumer(cons, artifact.KindService, "commented-out"),
		"a commented-out field must not be emitted")
}
