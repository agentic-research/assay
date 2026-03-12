package code

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/golang"

	"github.com/agentic-research/assay/internal/coverage"
)

// ExtractDir walks a directory and extracts all documentable entities from Go files.
func ExtractDir(root string, exportedOnly bool) ([]coverage.Entity, error) {
	var entities []coverage.Entity

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			base := d.Name()
			if base == "vendor" || base == "testdata" || base == ".git" || strings.HasPrefix(base, ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".go" {
			return nil
		}
		// Skip test files.
		if strings.HasSuffix(path, "_test.go") {
			return nil
		}

		rel, _ := filepath.Rel(root, path)
		found, err := ExtractFile(path, rel, exportedOnly)
		if err != nil {
			return err
		}
		entities = append(entities, found...)
		return nil
	})
	return entities, err
}

// ExtractFile extracts documentable entities from a single Go file.
func ExtractFile(path, relPath string, exportedOnly bool) ([]coverage.Entity, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ExtractSource(src, relPath, exportedOnly)
}

// ExtractSource extracts documentable entities from Go source bytes.
func ExtractSource(src []byte, relPath string, exportedOnly bool) ([]coverage.Entity, error) {
	parser := sitter.NewParser()
	parser.SetLanguage(golang.GetLanguage())

	tree, err := parser.ParseCtx(context.Background(), nil, src)
	if err != nil {
		return nil, err
	}
	root := tree.RootNode()

	// Extract package name.
	pkg := extractPackageName(root, src)

	var entities []coverage.Entity
	for _, q := range GoQueries {
		found, err := runQuery(q, root, src, pkg, relPath, exportedOnly)
		if err != nil {
			return nil, err
		}
		entities = append(entities, found...)
	}
	return entities, nil
}

// extractDocComment walks backward from a scope node to collect contiguous
// comment siblings. Uses the same adjacency check as mache's engine:
// gap ≤ 2 bytes between nodes (allows \n or \r\n, rejects double blank lines).
func extractDocComment(scopeNode *sitter.Node, src []byte) string {
	if scopeNode == nil {
		return ""
	}
	startByte := scopeNode.StartByte()
	n := scopeNode
	prev := n.PrevSibling()
	for prev != nil && prev.Type() == "comment" {
		gap := int(n.StartByte()) - int(prev.EndByte())
		if gap <= 2 {
			startByte = prev.StartByte()
			n = prev
			prev = prev.PrevSibling()
		} else {
			break
		}
	}
	if startByte < scopeNode.StartByte() {
		text := string(src[startByte:scopeNode.StartByte()])
		return strings.TrimRight(text, "\n\r\t ")
	}
	return ""
}

func extractPackageName(root *sitter.Node, src []byte) string {
	q, err := sitter.NewQuery([]byte(`(package_clause (package_identifier) @pkg)`), golang.GetLanguage())
	if err != nil {
		return ""
	}
	defer q.Close()

	qc := sitter.NewQueryCursor()
	defer qc.Close()
	qc.Exec(q, root)

	m, ok := qc.NextMatch()
	if !ok || len(m.Captures) == 0 {
		return ""
	}
	return m.Captures[0].Node.Content(src)
}

func runQuery(eq EntityQuery, root *sitter.Node, src []byte, pkg, relPath string, exportedOnly bool) ([]coverage.Entity, error) {
	q, err := sitter.NewQuery([]byte(eq.Query), golang.GetLanguage())
	if err != nil {
		return nil, err
	}
	defer q.Close()

	qc := sitter.NewQueryCursor()
	defer qc.Close()
	qc.Exec(q, root)

	// Map capture names to indices.
	nameIdx := -1
	receiverIdx := -1
	for i := uint32(0); i < q.CaptureCount(); i++ {
		cn := q.CaptureNameForId(i)
		switch cn {
		case "name":
			nameIdx = int(i)
		case "receiver":
			receiverIdx = int(i)
		}
	}
	if nameIdx < 0 {
		return nil, nil
	}

	// Find scope capture index for doc comment extraction.
	scopeIdx := -1
	for i := uint32(0); i < q.CaptureCount(); i++ {
		if q.CaptureNameForId(i) == "scope" {
			scopeIdx = int(i)
		}
	}

	var entities []coverage.Entity
	for {
		m, ok := qc.NextMatch()
		if !ok {
			break
		}

		var name, receiver string
		var scopeNode *sitter.Node
		for _, c := range m.Captures {
			switch int(c.Index) {
			case nameIdx:
				name = c.Node.Content(src)
			case receiverIdx:
				receiver = c.Node.Content(src)
			case scopeIdx:
				scopeNode = c.Node
			}
		}
		if name == "" {
			continue
		}

		exported := len(name) > 0 && unicode.IsUpper(rune(name[0]))
		if exportedOnly && !exported {
			continue
		}

		displayName := name
		if receiver != "" {
			displayName = receiver + "." + name
		}

		docComment := extractDocComment(scopeNode, src)

		entities = append(entities, coverage.Entity{
			Name:       displayName,
			Kind:       eq.Kind,
			Package:    pkg,
			File:       relPath,
			Exported:   exported,
			DocComment: docComment,
		})
	}
	return entities, nil
}
