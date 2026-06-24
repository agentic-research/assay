package docs

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/markdown"

	"github.com/agentic-research/assay/internal/coverage"
)

// ExtractDir walks a directory and extracts code references from all markdown files.
func ExtractDir(root string) ([]coverage.DocRef, error) {
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
		if ext != ".md" && ext != ".markdown" {
			return nil
		}

		rel, _ := filepath.Rel(root, path)
		found, err := ExtractFile(path, rel)
		if err != nil {
			return err
		}
		refs = append(refs, found...)
		return nil
	})
	return refs, err
}

// ExtractFile extracts code references from a single markdown file.
func ExtractFile(path, relPath string) ([]coverage.DocRef, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ExtractSource(src, relPath)
}

// ExtractSource extracts code references from markdown source bytes.
func ExtractSource(src []byte, relPath string) ([]coverage.DocRef, error) {
	tree, err := markdown.ParseCtx(context.Background(), nil, src)
	if err != nil {
		return nil, err
	}

	var refs []coverage.DocRef
	var inHeading bool

	tree.Iter(func(node *markdown.Node) bool {
		typ := node.Type()

		// Track when we enter/exit heading blocks.
		switch typ {
		case "atx_heading":
			inHeading = true
		case "section", "paragraph", "document":
			inHeading = false
		}

		if node.Inline == nil {
			return true
		}

		if inHeading {
			// Heading inline content — check if it looks like an identifier.
			text := strings.TrimSpace(node.Inline.Content(src))
			if looksLikeIdentifier(text) {
				refs = append(refs, coverage.DocRef{
					Text: text,
					Kind: "heading",
					File: relPath,
					Line: int(node.StartPoint().Row) + 1,
				})
			}
			inHeading = false
		} else {
			// Non-heading inline: extract code_span nodes.
			inlineRefs := extractInlineRefs(node.Inline, src, relPath)
			refs = append(refs, inlineRefs...)
		}

		return true
	})

	return refs, nil
}

func extractInlineRefs(inlineRoot *sitter.Node, src []byte, relPath string) []coverage.DocRef {
	var refs []coverage.DocRef
	walkNode(inlineRoot, func(n *sitter.Node) {
		if n.Type() == "code_span" {
			text := n.Content(src)
			// Strip surrounding backticks if present in content.
			text = strings.TrimPrefix(text, "`")
			text = strings.TrimSuffix(text, "`")
			text = strings.TrimSpace(text)
			if text != "" && !isCommonNonIdentifier(text) {
				refs = append(refs, coverage.DocRef{
					Text: text,
					Kind: "code_span",
					File: relPath,
					Line: int(n.StartPoint().Row) + 1,
				})
			}
		}
	})
	return refs
}

func walkNode(node *sitter.Node, fn func(*sitter.Node)) {
	if node == nil {
		return
	}
	fn(node)
	for i := 0; i < int(node.ChildCount()); i++ {
		walkNode(node.Child(i), fn)
	}
}

// looksLikeIdentifier checks if text looks like a code identifier.
func looksLikeIdentifier(text string) bool {
	if text == "" {
		return false
	}
	// Must start with a letter and contain only letters, digits, dots, underscores.
	for i, r := range text {
		if i == 0 {
			if r < 'A' || (r > 'Z' && r < 'a') || r > 'z' {
				return false
			}
			continue
		}
		if r == '.' || r == '_' || (r >= '0' && r <= '9') || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') {
			continue
		}
		return false
	}
	return true
}

// isCommonNonIdentifier filters out common backtick content that isn't a code identifier.
func isCommonNonIdentifier(text string) bool {
	// Filter CLI flags, file paths, shell commands.
	if strings.HasPrefix(text, "-") || strings.HasPrefix(text, "/") || strings.HasPrefix(text, "~") {
		return true
	}
	// Identifiers don't contain spaces.
	if strings.Contains(text, " ") {
		return true
	}
	return false
}
