package code

import "github.com/agentic-research/assay/internal/coverage"

// TreeSitterVerifier is the default coverage.StructuralVerifier: it derives
// the code's entity set by walking a source tree and extracting documentable
// constructs with tree-sitter (ExtractDir). This is the real, always-available
// backend — tree-sitter is compiled in, so it never reports unavailable.
type TreeSitterVerifier struct {
	Root         string
	ExportedOnly bool
}

// NewTreeSitterVerifier returns the default verifier rooted at the given
// source directory.
func NewTreeSitterVerifier(root string, exportedOnly bool) TreeSitterVerifier {
	return TreeSitterVerifier{Root: root, ExportedOnly: exportedOnly}
}

// Name identifies this backend in reports.
func (v TreeSitterVerifier) Name() string { return "tree-sitter" }

// Available always returns true: tree-sitter is linked into the binary, so
// this backend has no external dependency that could be missing.
func (v TreeSitterVerifier) Available() bool { return true }

// Entities walks the source root and extracts documentable entities,
// satisfying coverage.StructuralVerifier.
func (v TreeSitterVerifier) Entities() ([]coverage.Entity, error) {
	return ExtractDir(v.Root, v.ExportedOnly)
}

// Compile-time assertion that TreeSitterVerifier implements the interface.
var _ coverage.StructuralVerifier = TreeSitterVerifier{}
