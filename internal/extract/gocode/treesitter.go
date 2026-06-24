package gocode

import (
	"bufio"
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/golang"

	"github.com/agentic-research/assay/internal/artifact"
	"github.com/agentic-research/assay/internal/code"
)

// treeSitterBackend is the always-available backend. It walks the scan root's
// Go source with tree-sitter (reusing internal/code for entity extraction) and
// emits package-symbol producers and import consumers with provenance.
//
// It is the floor the Extractor falls back to whenever no mache .db is
// available, so the pipeline is never blocked on mache.
type treeSitterBackend struct {
	exportedOnly bool
}

func newTreeSitterBackend(exportedOnly bool) treeSitterBackend {
	return treeSitterBackend{exportedOnly: exportedOnly}
}

// extract walks root and returns producers (package symbols) and consumers
// (imports) in deterministic order.
func (b treeSitterBackend) extract(root string) ([]artifact.Producer, []artifact.Consumer, error) {
	modPath := readModulePath(root)

	entities, err := code.ExtractDir(root, b.exportedOnly)
	if err != nil {
		return nil, nil, err
	}

	producers := make([]artifact.Producer, 0, len(entities))
	for _, e := range entities {
		importPath := importPathFor(modPath, e.File)
		ref := e.Name
		if importPath != "" {
			ref = importPath + "." + e.Name
		}
		producers = append(producers, artifact.Producer{
			Identity:   artifact.NewIdentity(artifact.KindGoPackageSymbol, ref),
			Provenance: artifact.Provenance{File: e.File, Line: 0},
		})
	}

	consumers, err := extractImports(root)
	if err != nil {
		return nil, nil, err
	}

	sortProducers(producers)
	sortConsumers(consumers)
	return producers, consumers, nil
}

// extractImports walks root and emits one consumer per import spec found in each
// non-test Go file, keyed on the imported package path as a go_module ref.
func extractImports(root string) ([]artifact.Consumer, error) {
	var consumers []artifact.Consumer
	walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			base := d.Name()
			if base == "vendor" || base == "testdata" || base == ".git" || (base != "." && strings.HasPrefix(base, ".")) {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		found, err := importsInSource(src, rel)
		if err != nil {
			return err
		}
		consumers = append(consumers, found...)
		return nil
	})
	return consumers, walkErr
}

// importsInSource parses a single Go source and returns a consumer for each
// imported package path, with the import spec's 1-based line as provenance.
func importsInSource(src []byte, relPath string) ([]artifact.Consumer, error) {
	parser := sitter.NewParser()
	parser.SetLanguage(golang.GetLanguage())
	tree, err := parser.ParseCtx(context.Background(), nil, src)
	if err != nil {
		return nil, err
	}
	defer tree.Close()

	q, err := sitter.NewQuery(
		[]byte(`(import_spec path: (interpreted_string_literal) @path)`),
		golang.GetLanguage(),
	)
	if err != nil {
		return nil, err
	}
	defer q.Close()

	qc := sitter.NewQueryCursor()
	defer qc.Close()
	qc.Exec(q, tree.RootNode())

	var consumers []artifact.Consumer
	for {
		m, ok := qc.NextMatch()
		if !ok {
			break
		}
		for _, c := range m.Captures {
			raw := c.Node.Content(src)
			path := strings.Trim(raw, "`\"")
			if path == "" {
				continue
			}
			consumers = append(consumers, artifact.Consumer{
				Identity:   artifact.NewIdentity(artifact.KindGoModule, path),
				Provenance: artifact.Provenance{File: relPath, Line: int(c.Node.StartPoint().Row) + 1},
			})
		}
	}
	return consumers, nil
}

// readModulePath returns the module path from root/go.mod, or "" if there is no
// go.mod (the producer ref then falls back to the bare symbol name).
func readModulePath(root string) string {
	f, err := os.Open(filepath.Join(root, "go.mod"))
	if err != nil {
		return ""
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if rest, ok := strings.CutPrefix(line, "module "); ok {
			return strings.TrimSpace(rest)
		}
	}
	return ""
}

// importPathFor derives the import path of the package containing relFile from
// the module path and the file's directory relative to the module root. Returns
// "" when the module path is unknown.
func importPathFor(modPath, relFile string) string {
	if modPath == "" {
		return ""
	}
	dir := filepath.ToSlash(filepath.Dir(relFile))
	if dir == "." || dir == "" {
		return modPath
	}
	return modPath + "/" + dir
}

func sortProducers(ps []artifact.Producer) {
	sort.Slice(ps, func(i, j int) bool {
		if ps[i].Identity.Ref != ps[j].Identity.Ref {
			return ps[i].Identity.Ref < ps[j].Identity.Ref
		}
		return ps[i].Provenance.File < ps[j].Provenance.File
	})
}

func sortConsumers(cs []artifact.Consumer) {
	sort.Slice(cs, func(i, j int) bool {
		if cs[i].Identity.Ref != cs[j].Identity.Ref {
			return cs[i].Identity.Ref < cs[j].Identity.Ref
		}
		if cs[i].Provenance.File != cs[j].Provenance.File {
			return cs[i].Provenance.File < cs[j].Provenance.File
		}
		return cs[i].Provenance.Line < cs[j].Provenance.Line
	})
}
