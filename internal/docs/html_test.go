package docs

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/agentic-research/assay/internal/coverage"
)

// The static-HTML source is the real, CI-runnable path. It must extract
// claim-references from <code> elements and headings exactly the way the
// markdown source extracts code spans and headings, so the two compose
// behind coverage.ClaimSource identically.
func TestExtractHTMLSource_CodeAndHeadings(t *testing.T) {
	html := []byte(`<!DOCTYPE html>
<html>
<head><title>API</title></head>
<body>
  <h1>ComputeFromSources</h1>
  <p>Call <code>MergeRefs</code> to combine references.</p>
  <h2>Helpers and notes</h2>
  <pre><code>NewMarkdownSource</code></pre>
  <p>Run <code>--verbose</code> for detail, see <code>/etc/hosts</code>.</p>
</body>
</html>`)

	refs, err := ExtractHTMLSource(html, "api.html")
	if err != nil {
		t.Fatalf("ExtractHTMLSource: %v", err)
	}

	texts := map[string]string{} // text -> kind
	for _, r := range refs {
		if r.File != "api.html" {
			t.Errorf("DocRef.File = %q, want %q", r.File, "api.html")
		}
		if r.Line < 1 {
			t.Errorf("DocRef.Line = %d, want >= 1 for %q", r.Line, r.Text)
		}
		texts[r.Text] = r.Kind
	}

	// Identifier-shaped headings become "heading" refs; multi-word ones don't.
	if texts["ComputeFromSources"] != "heading" {
		t.Errorf("want ComputeFromSources as heading, got kind %q", texts["ComputeFromSources"])
	}
	if _, ok := texts["Helpers and notes"]; ok {
		t.Error("multi-word heading should not be a claim-reference")
	}

	// Identifier-shaped <code> spans become "code_span" refs.
	if texts["MergeRefs"] != "code_span" {
		t.Errorf("want MergeRefs as code_span, got kind %q", texts["MergeRefs"])
	}
	if texts["NewMarkdownSource"] != "code_span" {
		t.Errorf("want NewMarkdownSource as code_span, got kind %q", texts["NewMarkdownSource"])
	}

	// CLI flags and paths are filtered out, mirroring the markdown extractor.
	if _, ok := texts["--verbose"]; ok {
		t.Error("CLI flag --verbose should be filtered")
	}
	if _, ok := texts["/etc/hosts"]; ok {
		t.Error("path /etc/hosts should be filtered")
	}
}

// HTMLSource satisfies coverage.ClaimSource and walks a directory tree,
// mirroring MarkdownSource.
func TestHTMLSource_ClaimsOverDir(t *testing.T) {
	dir := t.TempDir()
	page := filepath.Join(dir, "index.html")
	if err := os.WriteFile(page, []byte(
		`<html><body><h1>VerifyCommand</h1><code>RunVerify</code></body></html>`,
	), 0o644); err != nil {
		t.Fatal(err)
	}
	// A non-HTML sibling must be ignored.
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("ignore me"), 0o644); err != nil {
		t.Fatal(err)
	}

	var src coverage.ClaimSource = NewHTMLSource(dir)
	refs, err := src.Claims()
	if err != nil {
		t.Fatalf("Claims: %v", err)
	}

	got := map[string]bool{}
	for _, r := range refs {
		got[r.Text] = true
		if r.File != "index.html" {
			t.Errorf("DocRef.File = %q, want relative %q", r.File, "index.html")
		}
	}
	if !got["VerifyCommand"] || !got["RunVerify"] {
		t.Errorf("missing expected refs, got %v", got)
	}
}
