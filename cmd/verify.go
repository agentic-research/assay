package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/agentic-research/assay/internal/code"
	"github.com/agentic-research/assay/internal/coverage"
	"github.com/agentic-research/assay/internal/docs"
	"github.com/agentic-research/assay/internal/structural"
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
	flagVerifier     string
	flagMacheDB      string
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
	verifyCmd.Flags().StringVar(&flagVerifier, "verifier", "tree-sitter", "Structural verifier backend: tree-sitter, mache")
	verifyCmd.Flags().StringVar(&flagMacheDB, "mache-db", "", "Leyline-parsed .db with canonical AST views (required for --verifier=mache)")
}

// selectVerifier resolves the configured structural-verifier backend. The
// tree-sitter backend is the always-available default; the mache backend is
// selectable but guarded — it must be Available() (mache on PATH + a .db),
// otherwise selection fails loudly rather than silently faking a result.
func selectVerifier(source string) (coverage.StructuralVerifier, error) {
	switch flagVerifier {
	case "", "tree-sitter":
		return code.NewTreeSitterVerifier(source, flagExportedOnly), nil
	case "mache":
		v := structural.NewMacheVerifier(flagMacheDB)
		if !v.Available() {
			return nil, fmt.Errorf(
				"mache verifier unavailable: needs `mache` on PATH and --mache-db pointing at a leyline-parsed .db",
			)
		}
		return v, nil
	default:
		return nil, fmt.Errorf("unknown verifier %q; must be one of: tree-sitter, mache", flagVerifier)
	}
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

	// Select the structural verifier — the "what the code has" side of the
	// coverage join. tree-sitter is the default real backend; mache is a
	// selectable, guarded backend behind the same interface.
	verifier, err := selectVerifier(source)
	if err != nil {
		return err
	}

	// Claim sources: the docs directory plus root-level markdown
	// (README.md, ARCHITECTURE.md, etc.). Both are markdown today; other
	// formats plug in behind coverage.ClaimSource.
	sources := []coverage.ClaimSource{
		docs.NewMarkdownSource(docsDir),
		docs.NewMarkdownSource(source),
	}

	// Gather claim-references, then compute coverage of the verifier's
	// entity set against them.
	var refs []coverage.DocRef
	for _, src := range sources {
		found, err := src.Claims()
		if err != nil {
			return fmt.Errorf("gather claims: %w", err)
		}
		refs = coverage.MergeRefs(refs, found)
	}
	result, err := coverage.ComputeFromVerifier(verifier, refs, flagFuzzy)
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
