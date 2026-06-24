package report_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/agentic-research/assay/internal/artifact"
	"github.com/agentic-research/assay/internal/extract"
	"github.com/agentic-research/assay/internal/report"
	"github.com/agentic-research/assay/internal/resolve"
)

// sampleResult builds a small Result exercising all three buckets and a
// cross-file edge, in deliberately non-sorted construction order.
func sampleResult() *resolve.Result {
	return &resolve.Result{
		Resolved: []resolve.ResolvedEdge{
			{
				Edge: artifact.Edge{
					Producer: artifact.Producer{
						Identity:   artifact.NewIdentity(artifact.KindGoModule, "github.com/example/lib"),
						Provenance: artifact.Provenance{File: "rootA/go.mod", Line: 1},
					},
					Consumer: artifact.Consumer{
						Identity:   artifact.NewIdentity(artifact.KindGoModule, "github.com/example/lib@v1.2.0"),
						Provenance: artifact.Provenance{File: "rootB/go.mod", Line: 7},
					},
				},
				VersionMatch: resolve.VersionMatchUnknown,
			},
		},
		External: []artifact.Consumer{
			{
				Identity:   artifact.NewIdentity(artifact.KindGoModule, "github.com/spf13/cobra@v1.8.0"),
				Provenance: artifact.Provenance{File: "rootA/go.mod", Line: 5},
			},
		},
		Dangling: []artifact.Producer{
			{
				Identity:   artifact.NewIdentity(artifact.KindContainerImage, "ghcr.io/example/app:latest"),
				Provenance: artifact.Provenance{File: "rootA/.github/workflows/ci.yml", Line: 12},
			},
		},
	}
}

func TestRenderDeterministic(t *testing.T) {
	skipped := []extract.Skipped{
		{Name: "zeta", Reason: "missing"},
		{Name: "alpha", Reason: "missing"},
	}

	renderers := map[string]func(*bytes.Buffer, *report.Graph) error{
		"json":     func(b *bytes.Buffer, g *report.Graph) error { return report.RenderJSON(b, g) },
		"mermaid":  func(b *bytes.Buffer, g *report.Graph) error { return report.RenderMermaid(b, g) },
		"markdown": func(b *bytes.Buffer, g *report.Graph) error { return report.RenderMarkdown(b, g) },
	}

	for name, render := range renderers {
		t.Run(name, func(t *testing.T) {
			// Two independent graphs from two independent Results: output must be
			// byte-identical across runs.
			var first, second bytes.Buffer
			if err := render(&first, report.FromResult(sampleResult(), skipped, nil)); err != nil {
				t.Fatalf("first render: %v", err)
			}
			if err := render(&second, report.FromResult(sampleResult(), skipped, nil)); err != nil {
				t.Fatalf("second render: %v", err)
			}
			if first.String() != second.String() {
				t.Errorf("render not deterministic:\n--- first ---\n%s\n--- second ---\n%s",
					first.String(), second.String())
			}
			if first.Len() == 0 {
				t.Error("render produced no output")
			}
		})
	}
}

func TestRenderMermaidHasEdgeAndBuckets(t *testing.T) {
	var b bytes.Buffer
	g := report.FromResult(sampleResult(), nil, nil)
	if err := report.RenderMermaid(&b, g); err != nil {
		t.Fatalf("render: %v", err)
	}
	out := b.String()

	if !strings.Contains(out, "graph LR") {
		t.Error("mermaid missing graph declaration")
	}
	// Exactly one resolved edge: one arrow.
	if got := strings.Count(out, "-->"); got != 1 {
		t.Errorf("expected 1 edge arrow, got %d:\n%s", got, out)
	}
	// Edge labeled by bucket:version-match.
	if !strings.Contains(out, "resolved:unknown") {
		t.Errorf("edge missing bucket label:\n%s", out)
	}
	// Grouped subgraphs by kind heading.
	if !strings.Contains(out, "Go modules") || !strings.Contains(out, "Container images") {
		t.Errorf("missing kind subgraphs:\n%s", out)
	}
}

func TestRenderJSONShape(t *testing.T) {
	var b bytes.Buffer
	g := report.FromResult(sampleResult(), []extract.Skipped{{Name: "x", Reason: "y"}}, nil)
	if err := report.RenderJSON(&b, g); err != nil {
		t.Fatalf("render: %v", err)
	}
	out := b.String()
	for _, want := range []string{
		`"artifacts"`, `"edges"`, `"skipped"`,
		`"bucket": "resolved"`, `"bucket": "external"`, `"bucket": "dangling"`,
		`"version_match": "unknown"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("json missing %q:\n%s", want, out)
		}
	}
}

func TestRenderSurfacesFailedInputs(t *testing.T) {
	failed := []extract.Failed{
		{Extractor: "gomod", Root: "/roots/template-go", Err: errors.New("go.mod:1: usage: module module/path")},
	}

	t.Run("json", func(t *testing.T) {
		var b bytes.Buffer
		g := report.FromResult(sampleResult(), nil, failed)
		if err := report.RenderJSON(&b, g); err != nil {
			t.Fatalf("render: %v", err)
		}
		out := b.String()
		for _, want := range []string{`"failed"`, `"extractor": "gomod"`, `"root": "/roots/template-go"`} {
			if !strings.Contains(out, want) {
				t.Errorf("json missing %q:\n%s", want, out)
			}
		}
	})

	t.Run("markdown", func(t *testing.T) {
		var b bytes.Buffer
		g := report.FromResult(sampleResult(), nil, failed)
		if err := report.RenderMarkdown(&b, g); err != nil {
			t.Fatalf("render: %v", err)
		}
		out := b.String()
		for _, want := range []string{"Skipped inputs", "gomod", "template-go"} {
			if !strings.Contains(out, want) {
				t.Errorf("markdown missing %q:\n%s", want, out)
			}
		}
	})
}
