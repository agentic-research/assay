package docs

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"

	"github.com/agentic-research/assay/internal/coverage"
)

// HTMLSource is a coverage.ClaimSource that extracts claim-references from the
// static HTML documentation under a directory tree. It is the real, CI-runnable
// HTML path: it parses files with golang.org/x/net/html (no browser required),
// so a docs site rendered to HTML is verified the same way markdown is.
//
// The live-DOM counterpart — capturing a *rendered* page's accessibility tree
// and HTML via Chrome DevTools Protocol — is the DOMSource seam (see dom.go).
type HTMLSource struct {
	Root string
}

// NewHTMLSource returns a ClaimSource rooted at the given directory.
func NewHTMLSource(root string) HTMLSource {
	return HTMLSource{Root: root}
}

// Claims walks the source root and extracts code references from every HTML
// file, satisfying coverage.ClaimSource.
func (s HTMLSource) Claims() ([]coverage.DocRef, error) {
	return ExtractHTMLDir(s.Root)
}

// Compile-time assertion that HTMLSource implements the interface.
var _ coverage.ClaimSource = HTMLSource{}

// ExtractHTMLDir walks a directory and extracts code references from all HTML
// files. Mirrors ExtractDir for markdown.
func ExtractHTMLDir(root string) ([]coverage.DocRef, error) {
	var refs []coverage.DocRef

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			base := d.Name()
			if base == ".git" || base == "node_modules" || strings.HasPrefix(base, ".") {
				return filepath.SkipDir
			}
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".html" && ext != ".htm" {
			return nil
		}

		rel, _ := filepath.Rel(root, path)
		found, err := ExtractHTMLFile(path, rel)
		if err != nil {
			return err
		}
		refs = append(refs, found...)
		return nil
	})
	return refs, err
}

// ExtractHTMLFile extracts code references from a single HTML file.
func ExtractHTMLFile(path, relPath string) ([]coverage.DocRef, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ExtractHTMLSource(src, relPath)
}

// ExtractHTMLSource extracts code references from HTML source bytes.
//
// It treats two element kinds as claim-bearing, mirroring the markdown
// extractor's "code_span" and "heading" refs:
//
//   - <code> (and inline <tt>) elements → "code_span", filtered through the
//     same isCommonNonIdentifier guard so CLI flags and paths drop out.
//   - <h1>..<h6> headings whose text is identifier-shaped → "heading".
//
// Line numbers are derived by counting newlines up to each node's source
// offset; x/net/html does not expose positions directly, so the tokenizer is
// used to map element start tags to lines.
func ExtractHTMLSource(src []byte, relPath string) ([]coverage.DocRef, error) {
	lines := newLineIndex(src)

	z := html.NewTokenizer(bytes.NewReader(src))
	var refs []coverage.DocRef

	// Track the active claim-bearing element we are collecting text inside.
	var (
		collecting bool
		kind       string // "code_span" or "heading"
		startLine  int
		buf        strings.Builder
		depth      int // nesting depth of the active element, to match its close
	)

	flush := func() {
		if !collecting {
			return
		}
		text := strings.TrimSpace(buf.String())
		switch kind {
		case "code_span":
			if text != "" && !isCommonNonIdentifier(text) {
				refs = append(refs, coverage.DocRef{
					Text: text, Kind: "code_span", File: relPath, Line: startLine,
				})
			}
		case "heading":
			if looksLikeIdentifier(text) {
				refs = append(refs, coverage.DocRef{
					Text: text, Kind: "heading", File: relPath, Line: startLine,
				})
			}
		}
		collecting = false
		kind = ""
		buf.Reset()
		depth = 0
	}

	for {
		tt := z.Next()
		if tt == html.ErrorToken {
			break // EOF or malformed tail; best-effort parse like markdown's.
		}

		switch tt {
		case html.StartTagToken, html.SelfClosingTagToken:
			name, _ := z.TagName()
			a := atom.Lookup(name)
			if collecting {
				if tt == html.StartTagToken {
					depth++
				}
				continue
			}
			if k := claimKind(a); k != "" {
				collecting = true
				kind = k
				startLine = lines.lineAt(z.Raw())
				buf.Reset()
				depth = 0
			}

		case html.EndTagToken:
			if !collecting {
				continue
			}
			if depth > 0 {
				depth--
				continue
			}
			flush()

		case html.TextToken:
			if collecting {
				buf.Write(z.Text())
			}
		}
	}

	// Close any element left open by a malformed document.
	flush()

	return refs, nil
}

// claimKind reports the DocRef kind a claim-bearing element maps to, or "".
func claimKind(a atom.Atom) string {
	switch a {
	case atom.Code, atom.Tt:
		return "code_span"
	case atom.H1, atom.H2, atom.H3, atom.H4, atom.H5, atom.H6:
		return "heading"
	default:
		return ""
	}
}

// lineIndex maps a byte slice from the source back to its 1-based line number.
type lineIndex struct {
	src     []byte
	offsets []int // byte offset of the start of each line
}

func newLineIndex(src []byte) lineIndex {
	offsets := []int{0}
	for i, b := range src {
		if b == '\n' {
			offsets = append(offsets, i+1)
		}
	}
	return lineIndex{src: src, offsets: offsets}
}

// lineAt returns the 1-based line of the first occurrence of raw in the source.
// raw is the tokenizer's verbatim bytes for the current token, which are a
// subslice of src, so a substring search recovers its offset reliably.
func (li lineIndex) lineAt(raw []byte) int {
	idx := bytes.Index(li.src, raw)
	if idx < 0 {
		return 1
	}
	// Binary-search-free walk: line counts are small for doc pages.
	line := 1
	for _, off := range li.offsets[1:] {
		if off > idx {
			break
		}
		line++
	}
	return line
}
