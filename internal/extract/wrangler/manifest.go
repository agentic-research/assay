package wrangler

import (
	"fmt"
	"strings"

	"github.com/pelletier/go-toml/v2/unstable"

	"github.com/agentic-research/assay/internal/artifact"
)

// parseManifest walks a wrangler.toml's expressions once, emitting the worker it
// declares (from the top-level name) and every service it binds to (from service
// bindings, in both the [[services]] array-of-tables and inline
// `services = [ { ... } ]` array forms). It reads declarations only and never
// rewrites the file.
//
// The top-level name is the producer only when it sits at the document root: a
// `name = "..."` inside a binding table (e.g. a durable_objects binding) is not
// the worker's name. Service consumers are emitted by the bound service's name
// (the `service = "..."` value), never the local binding alias, and are
// de-duplicated within a single manifest so a service bound in both the base
// config and an [env.<name>] override yields one consumer fact.
//
// Provenance lines come from the go-toml AST: a key/value reports its own line,
// so a binding's `service` value is pinned exactly. A [[services]] header carries
// no usable position, so a binding declared that way is pinned to its first key
// line — the closest stable anchor.
func parseManifest(path string, data []byte) ([]artifact.Producer, []artifact.Consumer, error) {
	p := &unstable.Parser{}
	p.Reset(data)

	var producers []artifact.Producer
	var consumers []artifact.Consumer
	seen := make(map[string]struct{})

	addConsumer := func(service string, line int) {
		service = strings.TrimSpace(service)
		if service == "" {
			return
		}
		if _, dup := seen[service]; dup {
			return
		}
		seen[service] = struct{}{}
		consumers = append(consumers, artifact.Consumer{
			Identity:   artifact.NewIdentity(artifact.KindService, service),
			Provenance: artifact.Provenance{File: path, Line: line},
		})
	}

	// table is the dotted key path of the active [table] / [[array-table]] header.
	// A [[services]] header makes the active table a service-binding table whose
	// `service` key names the bound service.
	var table []string

	for p.NextExpression() {
		e := p.Expression()
		switch e.Kind {
		case unstable.Table, unstable.ArrayTable:
			table = keyPath(e)

		case unstable.KeyValue:
			line := startLine(p, e)
			key := keyPath(e)
			switch {
			case isServicesTable(table):
				// Inside a [[services]] table: the `service` key names the bound
				// service.
				if len(key) == 1 && key[0] == "service" {
					addConsumer(stringValue(e.Value()), line)
				}
			case len(key) == 1 && key[0] == "services":
				// Inline form: `services = [ { binding = .., service = .. }, .. ]`.
				// Accepted at the document root or under an [env.<name>] override.
				for _, svc := range inlineServices(e.Value()) {
					addConsumer(svc, line)
				}
			case len(table) == 0 && len(key) == 1 && key[0] == "name":
				// Top-level worker name → producer.
				if name := stringValue(e.Value()); name != "" {
					producers = append(producers, artifact.Producer{
						Identity:   artifact.NewIdentity(artifact.KindService, name),
						Provenance: artifact.Provenance{File: path, Line: line},
					})
				}
			}
		}
	}

	if err := p.Error(); err != nil {
		return nil, nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return producers, consumers, nil
}

// isServicesTable reports whether table is a service-binding array table: a
// [[services]] header at the document root or under an [env.<name>] override.
// The deepest segment is "services".
func isServicesTable(table []string) bool {
	return len(table) > 0 && table[len(table)-1] == "services"
}

// inlineServices returns the `service` value of every inline-table element in a
// `services = [ { ... } ]` array value, in source order. A non-array value, or
// an element without a `service` string, contributes nothing.
func inlineServices(v *unstable.Node) []string {
	if v == nil || v.Kind != unstable.Array {
		return nil
	}
	var out []string
	it := v.Children()
	for it.Next() {
		elem := it.Node()
		if elem.Kind != unstable.InlineTable {
			continue
		}
		if svc := inlineField(elem, "service"); svc != "" {
			out = append(out, svc)
		}
	}
	return out
}

// inlineField returns the string value of a named key inside an inline table, or
// "" when absent or non-string.
func inlineField(v *unstable.Node, name string) string {
	it := v.Children()
	for it.Next() {
		c := it.Node()
		k := keyPath(c)
		if len(k) == 1 && k[0] == name {
			return stringValue(c.Value())
		}
	}
	return ""
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

// startLine returns the 1-based source line where a node begins, or 0 if unknown.
func startLine(p *unstable.Parser, n *unstable.Node) int {
	return p.Shape(n.Raw).Start.Line
}
