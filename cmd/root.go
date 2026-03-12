package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "assay",
	Short: "Documentation coverage verifier",
	Long:  "Assay proves your documentation matches your code by treating docs as claims and verifying them against tree-sitter-extracted code entities.",
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}
