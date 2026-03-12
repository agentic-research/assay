package coverage

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTokenize_CamelCase(t *testing.T) {
	assert.Equal(t, []string{"new", "graph", "cache"}, Tokenize("NewGraphCache"))
	assert.Equal(t, []string{"memory", "store"}, Tokenize("MemoryStore"))
	assert.Equal(t, []string{"get", "node"}, Tokenize("getNode"))
}

func TestTokenize_SnakeCase(t *testing.T) {
	assert.Equal(t, []string{"memory", "store"}, Tokenize("memory_store"))
	assert.Equal(t, []string{"max", "open", "conns"}, Tokenize("max_open_conns"))
}

func TestTokenize_Qualified(t *testing.T) {
	assert.Equal(t, []string{"graph", "new", "store"}, Tokenize("graph.NewStore"))
}

func TestTokenize_Acronyms(t *testing.T) {
	assert.Equal(t, []string{"html", "parser"}, Tokenize("HTMLParser"))
	assert.Equal(t, []string{"nfs", "mount"}, Tokenize("NFSMount"))
}

func TestTokenize_PlainText(t *testing.T) {
	assert.Equal(t, []string{"the", "graph", "cache"}, Tokenize("the graph cache"))
	assert.Equal(t, []string{"in", "memory", "store"}, Tokenize("in-memory store"))
}

func TestTokenize_Empty(t *testing.T) {
	assert.Empty(t, Tokenize(""))
}

func TestJaccard_Identical(t *testing.T) {
	a := Tokenize("NewGraphCache")
	b := Tokenize("NewGraphCache")
	assert.Equal(t, 1.0, Jaccard(a, b))
}

func TestJaccard_Overlap(t *testing.T) {
	a := Tokenize("NewGraphCache")             // [new, graph, cache]
	b := Tokenize("create a new graph cache")  // [create, new, graph, cache] (a filtered)
	sim := Jaccard(a, b)
	assert.InDelta(t, 0.75, sim, 0.01) // {new,graph,cache} ∩ {create,new,graph,cache} = 3, union = 4
}

func TestJaccard_NoOverlap(t *testing.T) {
	a := Tokenize("MemoryStore")
	b := Tokenize("network handler")
	assert.Equal(t, 0.0, Jaccard(a, b))
}

func TestJaccard_SubwordMatch(t *testing.T) {
	// "MemoryStore" shares tokens with "the in-memory store"
	a := Tokenize("MemoryStore")         // [memory, store] → set {memory, store}
	b := Tokenize("the in-memory store") // [the, in, memory, store] → set {the, in, memory, store}
	sim := Jaccard(a, b)
	assert.InDelta(t, 0.5, sim, 0.01) // 2 shared / 4 union = 0.5 (above 0.4 threshold)
}

func TestJaccard_Empty(t *testing.T) {
	assert.Equal(t, 0.0, Jaccard(nil, nil))
	assert.Equal(t, 0.0, Jaccard(Tokenize("Foo"), nil))
}
