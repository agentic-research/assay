package cmd

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runVerifyArgs drives `assay verify` end to end, discarding the report. Every
// flag the tests rely on is passed explicitly: cobra flag values are package
// globals and persist between executions. --fuzzy 0 skips the fuzzy coverage
// layers, which dominate runtime over the real tree and play no part in the
// dead-reference check (it resolves exactly).
func runVerifyArgs(t *testing.T, args ...string) error {
	t.Helper()
	rootCmd.SetOut(io.Discard)
	rootCmd.SetArgs(append([]string{"verify", "--format", "json", "--fuzzy", "0"}, args...))
	t.Cleanup(func() { rootCmd.SetArgs(nil); rootCmd.SetOut(nil) })
	return rootCmd.Execute()
}

// TestVerify_DeadRefs_RealDocsPass runs the dead-reference gate over assay's own
// committed docs. It must pass: a reference rule that flags the real tree is a
// rule nobody can keep green.
func TestVerify_DeadRefs_RealDocsPass(t *testing.T) {
	if err := runVerifyArgs(t, "--source", "..", "--docs", "../docs", "--fail-on-dead-refs"); err != nil {
		t.Fatalf("assay's own docs must have no dead references, got: %v", err)
	}
}

// TestVerify_DeadRefs_DeletedEntityFails adds one doc naming an entity that
// does not exist in the code, next to the real docs, and asserts the gate fails
// and names the doc and the entity.
func TestVerify_DeadRefs_DeletedEntityFails(t *testing.T) {
	docs := t.TempDir()
	stale := "# Gate\n\nThe decision is made by `coverage.GateThatWasDeleted`.\n"
	if err := os.WriteFile(filepath.Join(docs, "stale.md"), []byte(stale), 0o644); err != nil {
		t.Fatal(err)
	}

	err := runVerifyArgs(t, "--source", "..", "--docs", docs, "--fail-on-dead-refs")
	if err == nil {
		t.Fatal("a doc naming a deleted entity must fail verify, got nil")
	}
	for _, want := range []string{"coverage.GateThatWasDeleted", "stale.md:3"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("verify error must name %q, got: %v", want, err)
		}
	}
}

// TestIsWithin pins the docs-dir dedupe: a docs directory under the source root
// is already walked, and only one outside it is added as its own claim source.
func TestIsWithin(t *testing.T) {
	for _, c := range []struct {
		path, root string
		want       bool
	}{
		{"/repo/docs", "/repo", true},
		{"/repo", "/repo", true},
		{"/elsewhere/docs", "/repo", false},
		{"/repo-docs", "/repo", false},
		{"/repo/..docs", "/repo", true},
	} {
		if got := isWithin(c.path, c.root); got != c.want {
			t.Errorf("isWithin(%q, %q) = %v, want %v", c.path, c.root, got, c.want)
		}
	}
}
