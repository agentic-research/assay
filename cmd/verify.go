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
	Long:  "Extract documentable entities from source code and code references from markdown, then compute coverage.",
	RunE:  runVerify,
}

var (
	flagSource       string
	flagDocs         string
	flagThreshold    float64
	flagFormat       string
	flagExportedOnly bool
	flagVerbose      bool
)

func init() {
	rootCmd.AddCommand(verifyCmd)

	verifyCmd.Flags().StringVar(&flagSource, "source", ".", "Source code root directory")
	verifyCmd.Flags().StringVar(&flagDocs, "docs", "", "Documentation directory (default: auto-detect)")
	verifyCmd.Flags().Float64Var(&flagThreshold, "threshold", 0.0, "Minimum coverage ratio (0.0-1.0)")
	verifyCmd.Flags().StringVar(&flagFormat, "format", "text", "Output format: text, json")
	verifyCmd.Flags().BoolVar(&flagExportedOnly, "exported-only", true, "Only count exported/public entities")
	verifyCmd.Flags().BoolVarP(&flagVerbose, "verbose", "v", false, "Show all matched entities")
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

	// Also scan root-level markdown files (README.md, ARCHITECTURE.md, etc.)
	rootMarkdown, err := docs.ExtractDir(source)
	if err != nil {
		return fmt.Errorf("extract root markdown refs: %w", err)
	}

	// Extract code entities.
	entities, err := code.ExtractDir(source, flagExportedOnly)
	if err != nil {
		return fmt.Errorf("extract code entities: %w", err)
	}

	// Extract doc references from the docs directory.
	docRefs, err := docs.ExtractDir(docsDir)
	if err != nil {
		return fmt.Errorf("extract doc refs: %w", err)
	}

	// Merge root-level markdown refs (deduplicated).
	docRefs = mergeRefs(docRefs, rootMarkdown)

	// Compute coverage.
	result := coverage.Compute(entities, docRefs)

	// Output report.
	switch flagFormat {
	case "json":
		if err := coverage.FormatJSON(os.Stdout, result); err != nil {
			return err
		}
	default:
		coverage.FormatText(os.Stdout, result, flagVerbose)
	}

	// Exit with error if below threshold.
	if flagThreshold > 0 && result.Coverage < flagThreshold {
		os.Exit(1)
	}
	return nil
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

// mergeRefs combines two ref slices, deduplicating by text+file+line.
func mergeRefs(a, b []coverage.DocRef) []coverage.DocRef {
	seen := make(map[string]bool)
	var merged []coverage.DocRef
	for _, refs := range [][]coverage.DocRef{a, b} {
		for _, r := range refs {
			key := fmt.Sprintf("%s:%s:%d", r.Text, r.File, r.Line)
			if !seen[key] {
				seen[key] = true
				merged = append(merged, r)
			}
		}
	}
	return merged
}
