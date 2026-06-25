package cargo

import (
	"fmt"
	"strings"

	"github.com/pelletier/go-toml/v2/unstable"

	"github.com/agentic-research/assay/internal/artifact"
)

// depTables are the manifest tables whose keys name dependency crates. The same
// three names appear bare ([dependencies]), under [workspace] ([workspace.
// dependencies]), and under a platform predicate ([target.'cfg(...)'.
// dependencies]); the walk matches on the table name wherever it sits.
var depTables = map[string]struct{}{
	"dependencies":       {},
	"dev-dependencies":   {},
	"build-dependencies": {},
}

// parseManifest walks a Cargo.toml's expressions once, emitting the crate it
// declares (from [package].name) and the crates it depends on (from every
// dependency table, in every form Cargo allows). It reads declarations only and
// never rewrites the file.
//
// Provenance lines come from the go-toml AST: a key/value reports its own line,
// so an inline dependency (`foo = { ... }`) is pinned exactly. A [deps.<crate>]
// subtable header carries no usable position, so such a dependency is pinned to
// its first key line — the closest stable anchor.
//
// Consumers are de-duplicated by crate name within a single manifest: a crate
// pulled in by both [dependencies] and [dev-dependencies], or named across
// several dotted keys (`foo.version`, `foo.features`), yields one consumer fact
// at its first occurrence. The resolver matches on identity, so one fact per
// crate per manifest is sufficient and keeps the output stable.
func parseManifest(path string, data []byte) ([]artifact.Producer, []artifact.Consumer, error) {
	p := &unstable.Parser{}
	p.Reset(data)

	var producers []artifact.Producer
	var consumers []artifact.Consumer
	seen := make(map[string]struct{})

	addConsumer := func(crate string, line int) {
		crate = strings.TrimSpace(crate)
		if crate == "" {
			return
		}
		if _, dup := seen[crate]; dup {
			return
		}
		seen[crate] = struct{}{}
		consumers = append(consumers, artifact.Consumer{
			Identity:   artifact.NewIdentity(artifact.KindCargoCrate, crate),
			Provenance: artifact.Provenance{File: path, Line: line},
		})
	}

	var (
		table []string // dotted key path of the active [table] header
		sub   *subdep  // pending [deps.<crate>] subtable being accumulated
	)
	flush := func() {
		if sub != nil {
			addConsumer(sub.crate(), sub.line)
			sub = nil
		}
	}

	for p.NextExpression() {
		e := p.Expression()
		switch e.Kind {
		case unstable.Table, unstable.ArrayTable:
			flush()
			table = keyPath(e)
			if crate, ok := subtableDep(table); ok {
				sub = &subdep{alias: crate}
			}

		case unstable.KeyValue:
			line := startLine(p, e)
			key := keyPath(e)
			switch {
			case sub != nil:
				// Inside [deps.<crate>]: anchor the line and watch for a rename.
				if sub.line == 0 {
					sub.line = line
				}
				if len(key) == 1 && key[0] == "package" {
					sub.pkg = stringValue(e.Value())
				}
			case isDepTable(table):
				if len(key) == 0 {
					continue
				}
				crate := key[0]
				if v := e.Value(); v.Kind == unstable.InlineTable {
					if pkg := inlinePackage(v); pkg != "" {
						crate = pkg
					}
				}
				addConsumer(crate, line)
			case len(table) == 1 && table[0] == "package":
				if len(key) == 1 && key[0] == "name" {
					if name := stringValue(e.Value()); name != "" {
						producers = append(producers, artifact.Producer{
							Identity:   artifact.NewIdentity(artifact.KindCargoCrate, name),
							Provenance: artifact.Provenance{File: path, Line: line},
						})
					}
				}
			}
		}
	}
	flush()

	if err := p.Error(); err != nil {
		return nil, nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return producers, consumers, nil
}

// subdep accumulates a dependency declared as a [deps.<crate>] subtable, where
// the crate alias is the table name and an optional `package` rename appears as a
// child key on a later line.
type subdep struct {
	alias string
	pkg   string
	line  int
}

// crate is the dependency's resolved crate name: the package rename when present,
// otherwise the table alias.
func (s *subdep) crate() string {
	if s.pkg != "" {
		return s.pkg
	}
	return s.alias
}

// isDepTable reports whether table is a dependency table whose direct keys are
// crate names (e.g. [dependencies], [workspace.dependencies],
// [target.'cfg(unix)'.dependencies]).
func isDepTable(table []string) bool {
	i := depTableIndex(table)
	return i >= 0 && i == len(table)-1
}

// subtableDep reports the crate alias when table is a per-dependency subtable
// (e.g. [dependencies.serde] or [target.'cfg(unix)'.dependencies.serde]): a
// dependency table name followed by exactly one trailing crate segment.
func subtableDep(table []string) (string, bool) {
	i := depTableIndex(table)
	if i >= 0 && i == len(table)-2 {
		return table[len(table)-1], true
	}
	return "", false
}

// depTableIndex returns the index of the first dependency-table segment in a
// table path, or -1 if none. The segment may be the whole table ([dependencies])
// or nested under [workspace] or [target.<cfg>].
func depTableIndex(table []string) int {
	for i, seg := range table {
		if _, ok := depTables[seg]; ok {
			return i
		}
	}
	return -1
}

// keyPath returns the dotted key segments of a Table, ArrayTable, or KeyValue
// node as decoded strings.
func keyPath(n *unstable.Node) []string {
	var segs []string
	it := n.Key()
	for it.Next() {
		segs = append(segs, string(it.Node().Data))
	}
	return segs
}

// stringValue returns the decoded content of a String value node, or "" for any
// other kind.
func stringValue(v *unstable.Node) string {
	if v != nil && v.Kind == unstable.String {
		return string(v.Data)
	}
	return ""
}

// inlinePackage returns the `package` rename declared inside an inline-table
// dependency value (`foo = { package = "real-name" }`), or "" when absent.
func inlinePackage(v *unstable.Node) string {
	it := v.Children()
	for it.Next() {
		c := it.Node()
		k := keyPath(c)
		if len(k) == 1 && k[0] == "package" {
			return stringValue(c.Value())
		}
	}
	return ""
}

// startLine returns the 1-based source line where a node begins, or 0 if unknown.
func startLine(p *unstable.Parser, n *unstable.Node) int {
	return p.Shape(n.Raw).Start.Line
}
