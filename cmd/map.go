package cmd

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/agentic-research/assay/internal/extract"
	"github.com/agentic-research/assay/internal/extract/ci"
	"github.com/agentic-research/assay/internal/extract/dockerfile"
	"github.com/agentic-research/assay/internal/extract/gocode"
	"github.com/agentic-research/assay/internal/extract/gomod"
	"github.com/agentic-research/assay/internal/report"
	"github.com/agentic-research/assay/internal/resolve"
)

var mapCmd = &cobra.Command{
	Use:   "map <root> [root...]",
	Short: "Map the artifact/usage graph across one or more roots",
	Long: "Gather producer/consumer facts from every root (go.mod, Dockerfiles, CI " +
		"workflows, Go source), resolve them into a single artifact/usage graph, and " +
		"emit it as JSON or a mermaid diagram.\n\n" +
		"One root is a mono-repo; many roots are a multi-repo ecosystem — the same " +
		"resolver makes repo boundaries invisible, so a producer in one root and a " +
		"consumer in another resolve to a single cross-root edge.",
	Args: cobra.MinimumNArgs(1),
	RunE: runMap,
}

var (
	flagMapFormat string
	flagMapGroup  string
)

func init() {
	rootCmd.AddCommand(mapCmd)
	mapCmd.Flags().StringVar(&flagMapFormat, "format", "mermaid", "Output format: json, mermaid, md")
	mapCmd.Flags().StringVar(&flagMapGroup, "group", "repo",
		"Mermaid node grouping: repo (root→root edges, the dependency-graph shape) or artifact (one node per artifact)")
}

func runMap(cmd *cobra.Command, args []string) error {
	if err := validateMapFormat(flagMapFormat); err != nil {
		return err
	}
	if err := validateMapGroup(flagMapGroup); err != nil {
		return err
	}
	graph, err := buildGraph(args)
	if err != nil {
		return err
	}
	return renderMap(os.Stdout, graph, flagMapFormat, flagMapGroup)
}

// buildGraph runs the full map pipeline over roots: gather facts with all four
// extractors, resolve them, and fold the result (plus skip records and the roots
// themselves) into a report Graph. It is the seam the cmd tests drive directly.
func buildGraph(roots []string) (*report.Graph, error) {
	// All four extractors, in a stable registration order. gocode's tree-sitter
	// floor is always available, so the Go layer is never silently absent.
	registry := extract.NewRegistry(
		gomod.New(),
		dockerfile.Extractor{},
		ci.New(),
		gocode.New(),
	)

	facts, err := registry.Gather(roots...)
	if err != nil {
		return nil, fmt.Errorf("gather facts: %w", err)
	}

	result := resolve.Resolve(facts)
	return report.FromResultWithRoots(result, facts.Skipped, facts.Failed, roots), nil
}

// validateMapFormat rejects unknown formats up front so a typo fails fast rather
// than silently defaulting.
func validateMapFormat(format string) error {
	switch format {
	case "json", "mermaid", "md":
		return nil
	default:
		return fmt.Errorf("unknown format %q (want json, mermaid, or md)", format)
	}
}

// validateMapGroup rejects unknown grouping modes.
func validateMapGroup(group string) error {
	switch group {
	case "repo", "artifact":
		return nil
	default:
		return fmt.Errorf("unknown group %q (want repo or artifact)", group)
	}
}

// renderMap dispatches to the report renderer for the chosen format. For mermaid,
// the group mode selects the repo-level (root→root) or artifact-level diagram;
// JSON is always the full artifact-level fact set.
func renderMap(w io.Writer, graph *report.Graph, format, group string) error {
	switch format {
	case "json":
		return report.RenderJSON(w, graph)
	case "mermaid":
		return renderMermaid(w, graph, group)
	case "md":
		return report.RenderMarkdown(w, graph)
	default:
		return fmt.Errorf("unknown format %q", format)
	}
}

func renderMermaid(w io.Writer, graph *report.Graph, group string) error {
	if group == "repo" {
		return report.RenderMermaidRepo(w, graph)
	}
	return report.RenderMermaid(w, graph)
}
