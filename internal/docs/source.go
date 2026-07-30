package docs

import "github.com/agentic-research/assay/internal/coverage"

// MarkdownSource is a coverage.ClaimSource that extracts claim-references from
// the markdown files under a directory tree.
type MarkdownSource struct {
	Root string
}

// NewMarkdownSource returns a ClaimSource rooted at the given directory.
func NewMarkdownSource(root string) MarkdownSource {
	return MarkdownSource{Root: root}
}

// Claims walks the source root and extracts code references from every
// markdown file, satisfying coverage.ClaimSource.
func (s MarkdownSource) Claims() ([]coverage.DocRef, error) {
	return ExtractDir(s.Root)
}

// Compile-time assertion that MarkdownSource implements the interface.
var _ coverage.ClaimSource = MarkdownSource{}
