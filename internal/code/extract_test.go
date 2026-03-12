package code

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/agentic-research/assay/internal/coverage"
)

func TestExtractSource_Functions(t *testing.T) {
	src := []byte(`package example

func PublicFunc() {}
func privateFunc() {}
`)
	entities, err := ExtractSource(src, "example.go", true)
	require.NoError(t, err)

	names := entityNames(entities)
	assert.Contains(t, names, "PublicFunc")
	assert.NotContains(t, names, "privateFunc")
}

func TestExtractSource_AllEntities(t *testing.T) {
	src := []byte(`package example

func PublicFunc() {}
func privateFunc() {}
`)
	entities, err := ExtractSource(src, "example.go", false)
	require.NoError(t, err)

	names := entityNames(entities)
	assert.Contains(t, names, "PublicFunc")
	assert.Contains(t, names, "privateFunc")
}

func TestExtractSource_Types(t *testing.T) {
	src := []byte(`package example

type MyStruct struct {
	Field string
}

type MyInterface interface {
	Method()
}

type myPrivate struct{}
`)
	entities, err := ExtractSource(src, "example.go", true)
	require.NoError(t, err)

	names := entityNames(entities)
	assert.Contains(t, names, "MyStruct")
	assert.Contains(t, names, "MyInterface")
	assert.NotContains(t, names, "myPrivate")
}

func TestExtractSource_Methods(t *testing.T) {
	src := []byte(`package example

type Server struct{}

func (s *Server) Start() {}
func (s Server) Stop() {}
func (s *Server) internal() {}
`)
	entities, err := ExtractSource(src, "example.go", true)
	require.NoError(t, err)

	names := entityNames(entities)
	assert.Contains(t, names, "Server.Start")
	assert.Contains(t, names, "Server.Stop")
	assert.NotContains(t, names, "Server.internal")
	assert.Contains(t, names, "Server") // type itself
}

func TestExtractSource_Constants(t *testing.T) {
	src := []byte(`package example

const MaxSize = 1024
const minSize = 64
`)
	entities, err := ExtractSource(src, "example.go", true)
	require.NoError(t, err)

	names := entityNames(entities)
	assert.Contains(t, names, "MaxSize")
	assert.NotContains(t, names, "minSize")
}

func TestExtractSource_Package(t *testing.T) {
	src := []byte(`package graph

func NewStore() {}
`)
	entities, err := ExtractSource(src, "graph/store.go", true)
	require.NoError(t, err)

	require.Len(t, entities, 1)
	assert.Equal(t, "graph", entities[0].Package)
	assert.Equal(t, "NewStore", entities[0].Name)
	assert.Equal(t, "function", entities[0].Kind)
}

func TestExtractSource_DocComment(t *testing.T) {
	src := []byte(`package example

// NewStore creates a new store instance.
// It initializes the connection pool.
func NewStore() {}

// standalone comment far away


// another unrelated block


func Bare() {}
`)
	entities, err := ExtractSource(src, "example.go", true)
	require.NoError(t, err)

	var newStore, bare coverage.Entity
	for _, e := range entities {
		switch e.Name {
		case "NewStore":
			newStore = e
		case "Bare":
			bare = e
		}
	}
	assert.Contains(t, newStore.DocComment, "creates a new store instance")
	assert.Contains(t, newStore.DocComment, "initializes the connection pool")
	assert.Empty(t, bare.DocComment) // standalone comment too far away
}

func entityNames(entities []coverage.Entity) []string {
	var names []string
	for _, e := range entities {
		names = append(names, e.Name)
	}
	return names
}
