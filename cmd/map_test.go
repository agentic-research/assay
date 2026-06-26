package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/agentic-research/assay/internal/report"
)

// TestMapGoldenDeterministic asserts the emitted JSON and mermaid are
// byte-identical across two independent pipeline runs over the same fixture tree.
// Roots are relative paths so provenance (which embeds the root) is
// machine-independent.
func TestMapGoldenDeterministic(t *testing.T) {
	roots := []string{"testdata/cross/roota", "testdata/cross/rootb"}

	for _, format := range []string{"json", "mermaid", "md"} {
		t.Run(format, func(t *testing.T) {
			first := renderRoots(t, roots, format)
			second := renderRoots(t, roots, format)
			if first != second {
				t.Errorf("%s output not deterministic:\n--- first ---\n%s\n--- second ---\n%s",
					format, first, second)
			}
			if strings.TrimSpace(first) == "" {
				t.Errorf("%s output empty", format)
			}
		})
	}
}

// TestMapCrossRootEdge asserts that two roots — A producing module example.com/lib,
// B requiring it — yield exactly ONE resolved cross-root edge, with that edge
// running between the two roots' files.
func TestMapCrossRootEdge(t *testing.T) {
	roots := []string{"testdata/cross/roota", "testdata/cross/rootb"}

	graph, err := buildGraph(roots)
	if err != nil {
		t.Fatalf("buildGraph: %v", err)
	}

	if got := len(graph.Resolved); got != 1 {
		t.Fatalf("expected exactly 1 resolved edge, got %d: %+v", got, graph.Resolved)
	}

	edge := graph.Resolved[0]
	if edge.Producer.Identity.Ref != "example.com/lib" {
		t.Errorf("producer ref = %q, want example.com/lib", edge.Producer.Identity.Ref)
	}
	if !strings.HasPrefix(edge.Producer.Provenance.File, "testdata/cross/roota") {
		t.Errorf("producer file = %q, want under roota", edge.Producer.Provenance.File)
	}
	if !strings.HasPrefix(edge.Consumer.Provenance.File, "testdata/cross/rootb") {
		t.Errorf("consumer file = %q, want under rootb", edge.Consumer.Provenance.File)
	}

	// And the mermaid renders exactly that one cross-root arrow.
	var b bytes.Buffer
	if err := report.RenderMermaid(&b, graph); err != nil {
		t.Fatalf("render: %v", err)
	}
	if got := strings.Count(b.String(), "-->"); got != 1 {
		t.Errorf("expected 1 mermaid edge, got %d:\n%s", got, b.String())
	}
}

func renderRoots(t *testing.T, roots []string, format string) string {
	t.Helper()
	graph, err := buildGraph(roots)
	if err != nil {
		t.Fatalf("buildGraph: %v", err)
	}
	var b bytes.Buffer
	if err := renderMap(&b, graph, format, "repo"); err != nil {
		t.Fatalf("render %s: %v", format, err)
	}
	return b.String()
}

// TestMapRepoMermaidIsRootToRoot asserts the default repo-grouped mermaid draws a
// single root→root edge between the two fixture roots (labeled by basename), the
// dependency-graph shape — not an artifact self-loop.
func TestMapRepoMermaidIsRootToRoot(t *testing.T) {
	roots := []string{"testdata/cross/roota", "testdata/cross/rootb"}
	graph, err := buildGraph(roots)
	if err != nil {
		t.Fatalf("buildGraph: %v", err)
	}
	var b bytes.Buffer
	if err := report.RenderMermaidRepo(&b, graph); err != nil {
		t.Fatalf("render: %v", err)
	}
	out := b.String()
	if got := strings.Count(out, "-->"); got != 1 {
		t.Errorf("expected 1 repo edge, got %d:\n%s", got, out)
	}
	for _, want := range []string{`"roota"`, `"rootb"`, "1 artifact"} {
		if !strings.Contains(out, want) {
			t.Errorf("repo mermaid missing %q:\n%s", want, out)
		}
	}
}

// TestMapMarkdownGroupArtifactIsNonEmpty asserts the md self-snapshot stays
// non-empty for a single root. A lone root collapses to one repo-level node (an
// empty repo diagram), so `--group artifact` must embed the per-artifact diagram
// instead — that is what keeps assay's own per-PR snapshot meaningful.
func TestMapMarkdownGroupArtifactIsNonEmpty(t *testing.T) {
	roots := []string{"testdata/cross/roota"}
	graph, err := buildGraph(roots)
	if err != nil {
		t.Fatalf("buildGraph: %v", err)
	}

	repoMD := renderTo(t, graph, "md", "repo")
	artifactMD := renderTo(t, graph, "md", "artifact")

	if repoMD == artifactMD {
		t.Fatalf("artifact-group md should differ from repo-group md for a single root")
	}
	// roota produces example.com/lib; the artifact diagram must carry that node
	// even though the lone-root repo diagram has no edges.
	if !strings.Contains(artifactMD, "example.com/lib") {
		t.Errorf("artifact-group md missing the root's produced artifact:\n%s", artifactMD)
	}
}

func renderTo(t *testing.T, graph *report.Graph, format, group string) string {
	t.Helper()
	var b bytes.Buffer
	if err := renderMap(&b, graph, format, group); err != nil {
		t.Fatalf("render %s/%s: %v", format, group, err)
	}
	return b.String()
}
