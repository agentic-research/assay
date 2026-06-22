package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/agentic-research/assay/internal/code"
	"github.com/agentic-research/assay/internal/coverage"
	"github.com/agentic-research/assay/internal/docs"
)

var verifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "Verify documentation coverage against source code",
	Long: "Extract documentable entities from source code and code references from markdown, then compute coverage.\n\n" +
		"With --max-uncovered or --threshold, verify becomes a gate: it exits non-zero when documentation\n" +
		"has drifted behind the code, naming the entities responsible. Wire it into CI via `task docs:check`.",
	// A gate failure is a result, not a usage error — printing the full usage
	// block after it would bury the entity names the caller needs.
	SilenceUsage: true,
	RunE:         runVerify,
}

var (
	flagSource       string
	flagDocs         string
	flagThreshold    float64
	flagFuzzy        float64
	flagFormat       string
	flagExportedOnly bool
	flagVerbose      bool
	flagMaxUncovered int
)

func init() {
	rootCmd.AddCommand(verifyCmd)

	verifyCmd.Flags().StringVar(&flagSource, "source", ".", "Source code root directory")
	verifyCmd.Flags().StringVar(&flagDocs, "docs", "", "Documentation directory (default: auto-detect)")
	verifyCmd.Flags().Float64Var(&flagThreshold, "threshold", 0.0, "Minimum coverage ratio (0.0-1.0)")
	verifyCmd.Flags().Float64Var(&flagFuzzy, "fuzzy", coverage.DefaultFuzzyThreshold, "Jaccard similarity threshold for fuzzy matching (0=exact only)")
	verifyCmd.Flags().StringVar(&flagFormat, "format", "text", "Output format: text, json")
	verifyCmd.Flags().BoolVar(&flagExportedOnly, "exported-only", true, "Only count exported/public entities")
	verifyCmd.Flags().BoolVarP(&flagVerbose, "verbose", "v", false, "Show all matched entities")
	verifyCmd.Flags().IntVar(&flagMaxUncovered, "max-uncovered", -1,
		"Fail when more than N exported entities are undocumented (0 = none may be; -1 = disabled). Ratchet a repo by setting its current count.")
}

func runVerify(cmd *cobra.Command, args []string) error {
	source, err := filepath.Abs(flagSource)
	if err != nil {
		return fmt.Errorf("resolve source path: %w", err)
	}

	docsDir := flagDocs
	if docsDir == "" {
		docsDir = detectDocsDir(source)
	}
	if docsDir == "" {
		return fmt.Errorf("no docs directory found (tried docs/, doc/); use --docs to specify")
	}
	docsDir, err = filepath.Abs(docsDir)
	if err != nil {
		return fmt.Errorf("resolve docs path: %w", err)
	}

	// Extract code entities.
	entities, err := code.ExtractDir(source, flagExportedOnly)
	if err != nil {
		return fmt.Errorf("extract code entities: %w", err)
	}

	// Claim sources: the docs directory plus root-level markdown
	// (README.md, ARCHITECTURE.md, etc.). Both are markdown today; other
	// formats plug in behind coverage.ClaimSource.
	sources := []coverage.ClaimSource{
		docs.NewMarkdownSource(docsDir),
		docs.NewMarkdownSource(source),
	}

	// Compute coverage from the claim sources.
	result, err := coverage.ComputeFromSources(entities, flagFuzzy, sources...)
	if err != nil {
		return fmt.Errorf("compute coverage: %w", err)
	}

	// Output report.
	switch flagFormat {
	case "json":
		if err := coverage.FormatJSON(os.Stdout, result); err != nil {
			return err
		}
	default:
		if err := coverage.FormatText(os.Stdout, result, flagVerbose); err != nil {
			return err
		}
	}

	// Apply the gate. Returning an error rather than calling os.Exit keeps the
	// decision unit-testable and lets the reason reach the CI log — a bare
	// exit(1) tells a reader the build failed but not which entity caused it.
	return coverage.Gate(result, coverage.GateOptions{
		MaxUncovered:    flagMaxUncovered,
		MaxUncoveredSet: cmd.Flags().Changed("max-uncovered"),
		MinCoverage:     flagThreshold,
	})
}

func detectDocsDir(source string) string {
	for _, candidate := range []string{"docs", "doc"} {
		path := filepath.Join(source, candidate)
		info, err := os.Stat(path)
		if err == nil && info.IsDir() {
			return path
		}
	}
	return ""
}
