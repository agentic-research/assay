package extract

import (
	"errors"
	"testing"

	"github.com/agentic-research/assay/internal/artifact"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeExtractor is a test double that emits a fixed set of facts per root and
// reports a configurable availability. It satisfies the Extractor contract.
type fakeExtractor struct {
	name      string
	avail     bool
	reason    string
	err       error
	producers map[string][]artifact.Producer // keyed by root
	consumers map[string][]artifact.Consumer // keyed by root
}

func (f fakeExtractor) Name() string { return f.name }

func (f fakeExtractor) Available() (bool, string) { return f.avail, f.reason }

func (f fakeExtractor) Extract(root string) ([]artifact.Producer, []artifact.Consumer, error) {
	if f.err != nil {
		return nil, nil, f.err
	}
	return f.producers[root], f.consumers[root], nil
}

// compile-time assertion: fakeExtractor (and thus the interface) is honored.
var _ Extractor = fakeExtractor{}

func prod(kind artifact.Kind, ref, file string, line int) artifact.Producer {
	return artifact.Producer{
		Identity:   artifact.NewIdentity(kind, ref),
		Provenance: artifact.Provenance{File: file, Line: line},
	}
}

func cons(kind artifact.Kind, ref, file string, line int) artifact.Consumer {
	return artifact.Consumer{
		Identity:   artifact.NewIdentity(kind, ref),
		Provenance: artifact.Provenance{File: file, Line: line},
	}
}

func TestRegistry_GatherMergesFactsAcrossRootsWithProvenance(t *testing.T) {
	fx := fakeExtractor{
		name:  "fake",
		avail: true,
		producers: map[string][]artifact.Producer{
			"rootA": {prod(artifact.KindGoModule, "example.com/a", "rootA/go.mod", 1)},
			"rootB": {prod(artifact.KindGoModule, "example.com/b", "rootB/go.mod", 1)},
		},
		consumers: map[string][]artifact.Consumer{
			"rootA": {cons(artifact.KindGoModule, "example.com/x", "rootA/go.mod", 5)},
			"rootB": {cons(artifact.KindCLIBinary, "docker", "rootB/Makefile", 9)},
		},
	}

	reg := NewRegistry(fx)
	result, err := reg.Gather("rootA", "rootB")
	require.NoError(t, err)

	// Every fact from every (extractor, root) pass is present, with provenance
	// carried through untouched.
	assert.ElementsMatch(t, []artifact.Producer{
		prod(artifact.KindGoModule, "example.com/a", "rootA/go.mod", 1),
		prod(artifact.KindGoModule, "example.com/b", "rootB/go.mod", 1),
	}, result.Producers)
	assert.ElementsMatch(t, []artifact.Consumer{
		cons(artifact.KindGoModule, "example.com/x", "rootA/go.mod", 5),
		cons(artifact.KindCLIBinary, "docker", "rootB/Makefile", 9),
	}, result.Consumers)
	assert.Empty(t, result.Skipped)
}

func TestRegistry_RunsEveryExtractorOverEveryRoot(t *testing.T) {
	a := fakeExtractor{
		name:      "a",
		avail:     true,
		producers: map[string][]artifact.Producer{"r1": {prod(artifact.KindGoModule, "a/r1", "r1", 1)}},
	}
	b := fakeExtractor{
		name:      "b",
		avail:     true,
		producers: map[string][]artifact.Producer{"r1": {prod(artifact.KindGoModule, "b/r1", "r1", 1)}},
	}

	reg := NewRegistry(a, b)
	result, err := reg.Gather("r1")
	require.NoError(t, err)
	assert.ElementsMatch(t, []artifact.Producer{
		prod(artifact.KindGoModule, "a/r1", "r1", 1),
		prod(artifact.KindGoModule, "b/r1", "r1", 1),
	}, result.Producers)
}

func TestRegistry_SkipsUnavailableExtractorRecordingReason(t *testing.T) {
	good := fakeExtractor{
		name:      "good",
		avail:     true,
		producers: map[string][]artifact.Producer{"r1": {prod(artifact.KindGoModule, "good", "r1", 1)}},
	}
	bad := fakeExtractor{
		name:   "bad",
		avail:  false,
		reason: "binary not on PATH",
		// Would have emitted facts, but must never be invoked.
		producers: map[string][]artifact.Producer{"r1": {prod(artifact.KindGoModule, "must-not-appear", "r1", 1)}},
	}

	reg := NewRegistry(good, bad)
	result, err := reg.Gather("r1")
	require.NoError(t, err)

	// Unavailable extractor contributes no facts...
	assert.ElementsMatch(t, []artifact.Producer{
		prod(artifact.KindGoModule, "good", "r1", 1),
	}, result.Producers)

	// ...but its reason is recorded, not silently dropped, and not an error.
	require.Len(t, result.Skipped, 1)
	assert.Equal(t, "bad", result.Skipped[0].Name)
	assert.Equal(t, "binary not on PATH", result.Skipped[0].Reason)
}

func TestRegistry_PropagatesFirstError(t *testing.T) {
	sentinel := errors.New("extract boom")
	boom := fakeExtractor{name: "boom", avail: true, err: sentinel}
	ok := fakeExtractor{
		name:      "ok",
		avail:     true,
		producers: map[string][]artifact.Producer{"r1": {prod(artifact.KindGoModule, "ok", "r1", 1)}},
	}

	reg := NewRegistry(boom, ok)
	result, err := reg.Gather("r1")
	require.Error(t, err)
	assert.ErrorIs(t, err, sentinel)
	assert.Nil(t, result)
}

func TestRegistry_DeterministicOrder(t *testing.T) {
	fx := fakeExtractor{
		name:  "fake",
		avail: true,
		producers: map[string][]artifact.Producer{
			"r1": {
				prod(artifact.KindGoModule, "first", "r1", 1),
				prod(artifact.KindGoModule, "second", "r1", 2),
			},
			"r2": {prod(artifact.KindGoModule, "third", "r2", 1)},
		},
	}
	reg := NewRegistry(fx)

	first, err := reg.Gather("r1", "r2")
	require.NoError(t, err)
	for i := 0; i < 5; i++ {
		again, err := reg.Gather("r1", "r2")
		require.NoError(t, err)
		assert.Equal(t, first.Producers, again.Producers)
	}
}

func TestRegistry_NoRootsYieldsEmpty(t *testing.T) {
	fx := fakeExtractor{name: "fake", avail: true}
	reg := NewRegistry(fx)
	result, err := reg.Gather()
	require.NoError(t, err)
	assert.Empty(t, result.Producers)
	assert.Empty(t, result.Consumers)
	assert.Empty(t, result.Skipped)
}
