package report

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/agentic-research/assay/internal/artifact"
	"github.com/agentic-research/assay/internal/resolve"
)

// kindOrder fixes the subgraph order in the mermaid diagram so node grouping is
// deterministic and reads top-down from coarse (modules, images) to fine
// (symbols, binaries). Kinds not listed here sort after, alphabetically.
var kindOrder = map[artifact.Kind]int{
	artifact.KindGoModule:        0,
	artifact.KindContainerImage:  1,
	artifact.KindCLIBinary:       2,
	artifact.KindGoPackageSymbol: 3,
}

// kindLabel is the human heading for a kind's subgraph.
var kindLabel = map[artifact.Kind]string{
	artifact.KindGoModule:        "Go modules",
	artifact.KindContainerImage:  "Container images",
	artifact.KindCLIBinary:       "CLI binaries",
	artifact.KindGoPackageSymbol: "Go symbols",
}

// RenderMermaid writes just the fenced mermaid diagram for the graph to w. It is
// the diagram body of the Markdown report, exposed separately for the
// --format mermaid output. Output is byte-deterministic.
func RenderMermaid(w io.Writer, g *Graph) error {
	_, err := io.WriteString(w, mermaidDiagram(g))
	return err
}

// RenderMarkdown writes the full human report with the repo-level diagram. It is
// the default cross-repo dependency-graph view (one node per root); use
// RenderMarkdownGroup with "artifact" for the per-artifact diagram.
func RenderMarkdown(w io.Writer, g *Graph) error {
	return RenderMarkdownGroup(w, g, "repo")
}

// RenderMarkdownGroup writes the full human report: a heading, summary counts,
// the mermaid diagram, and bucket listings (external dependencies, dangling
// producers, skipped extractors). The group mode selects which diagram is
// embedded — "repo" for the root→root dependency-graph shape (the meaningful
// view across an ecosystem), or "artifact" for one node per artifact (the
// non-empty view for a single self-contained repo). Output is byte-deterministic.
func RenderMarkdownGroup(w io.Writer, g *Graph, group string) error {
	var b strings.Builder

	b.WriteString("# assay map\n\n")
	fmt.Fprintf(&b, "- Resolved edges: %d\n", len(g.Resolved))
	fmt.Fprintf(&b, "- External dependencies: %d\n", len(g.External))
	fmt.Fprintf(&b, "- Dangling producers: %d\n", len(g.Dangling))
	if len(g.Skipped) > 0 {
		fmt.Fprintf(&b, "- Skipped extractors: %d\n", len(g.Skipped))
	}
	if len(g.Failed) > 0 {
		fmt.Fprintf(&b, "- Skipped inputs (parse failures): %d\n", len(g.Failed))
	}
	// The repo-level diagram is the dependency-graph view worth reading across an
	// ecosystem; for a single repo it collapses to one node (empty), so the
	// artifact group gives a non-empty per-artifact view instead.
	b.WriteString("\n## Graph\n\n")
	b.WriteString("```mermaid\n")
	switch {
	case group == "artifact":
		b.WriteString(mermaidDiagram(g))
	case len(g.Roots) > 0:
		b.WriteString(repoDiagram(g))
	default:
		b.WriteString(mermaidDiagram(g))
	}
	b.WriteString("```\n")

	if len(g.External) > 0 {
		b.WriteString("\n## External dependencies\n\n")
		for _, c := range g.External {
			fmt.Fprintf(&b, "- `%s` (%s) — %s\n",
				c.Identity.Ref, c.Identity.Kind, provenanceString(c.Provenance))
		}
	}
	if len(g.Dangling) > 0 {
		b.WriteString("\n## Dangling producers\n\n")
		for _, p := range g.Dangling {
			fmt.Fprintf(&b, "- `%s` (%s) — %s\n",
				p.Identity.Ref, p.Identity.Kind, provenanceString(p.Provenance))
		}
	}
	if len(g.Skipped) > 0 {
		b.WriteString("\n## Skipped extractors\n\n")
		for _, s := range g.Skipped {
			fmt.Fprintf(&b, "- `%s` — %s\n", s.Name, s.Reason)
		}
	}
	if len(g.Failed) > 0 {
		b.WriteString("\n## Skipped inputs\n\n")
		for _, f := range g.Failed {
			fmt.Fprintf(&b, "- `%s` in `%s` — %s\n", f.Extractor, f.Root, f.Err)
		}
	}

	_, err := io.WriteString(w, b.String())
	return err
}

// mermaidDiagram renders the graph body (without the ```mermaid fence). Nodes are
// grouped into one subgraph per kind, declared in kindOrder; edges are emitted in
// resolved order (already identity-sorted) and labeled by bucket/version-match.
func mermaidDiagram(g *Graph) string {
	var b strings.Builder
	b.WriteString("graph LR\n")

	// Collect distinct nodes per kind with their bucket, so subgraphs and node
	// styling are deterministic. A node's bucket is resolved if it participates
	// in any edge; otherwise external/dangling as gathered.
	nodes := collectNodes(g)

	kinds := make([]artifact.Kind, 0, len(nodes))
	for k := range nodes {
		kinds = append(kinds, k)
	}
	sort.Slice(kinds, func(i, j int) bool { return kindLess(kinds[i], kinds[j]) })

	for _, k := range kinds {
		fmt.Fprintf(&b, "  subgraph %s[%q]\n", subgraphID(k), kindLabelOf(k))
		entries := nodes[k]
		sort.Slice(entries, func(i, j int) bool { return entries[i].ref < entries[j].ref })
		for _, n := range entries {
			fmt.Fprintf(&b, "    %s[%q]\n", n.id, n.ref)
		}
		b.WriteString("  end\n")
	}

	for _, e := range g.Resolved {
		from := nodeID(e.Producer.Identity)
		to := nodeID(e.Consumer.Identity)
		fmt.Fprintf(&b, "  %s -->|%s| %s\n", from, edgeLabel(e), to)
	}

	return b.String()
}

// node is a distinct diagram node: its stable id, its display ref, and the
// bucket it was first seen in.
type node struct {
	id     string
	ref    string
	bucket Bucket
}

// collectNodes groups distinct artifacts by kind. Resolved endpoints win the
// bucket assignment over external/dangling, since an artifact that is both
// produced-and-consumed is resolved.
func collectNodes(g *Graph) map[artifact.Kind][]node {
	byID := make(map[string]node)
	order := make(map[artifact.Kind][]string)
	add := func(id artifact.Identity, bucket Bucket) {
		nid := nodeID(id)
		existing, ok := byID[nid]
		if ok {
			if existing.bucket != BucketResolved && bucket == BucketResolved {
				existing.bucket = BucketResolved
				byID[nid] = existing
			}
			return
		}
		byID[nid] = node{id: nid, ref: id.Ref, bucket: bucket}
		order[id.Kind] = append(order[id.Kind], nid)
	}

	for _, e := range g.Resolved {
		add(e.Producer.Identity, BucketResolved)
		add(e.Consumer.Identity, BucketResolved)
	}
	for _, c := range g.External {
		add(c.Identity, BucketExternal)
	}
	for _, p := range g.Dangling {
		add(p.Identity, BucketDangling)
	}

	out := make(map[artifact.Kind][]node, len(order))
	for kind, ids := range order {
		for _, nid := range ids {
			out[kind] = append(out[kind], byID[nid])
		}
	}
	return out
}

// nodeID derives a stable, mermaid-safe node identifier from an identity. The
// identity key embeds the kind, so distinct kinds with the same ref get distinct
// nodes; the SHA-1 prefix keeps the id free of characters mermaid cannot use in
// an id (/, :, @, .) while staying deterministic.
func nodeID(id artifact.Identity) string {
	sum := sha1.Sum([]byte(id.Key()))
	return "n" + hex.EncodeToString(sum[:6])
}

// subgraphID is the deterministic id of a kind's subgraph container.
func subgraphID(k artifact.Kind) string {
	sum := sha1.Sum([]byte("kind:" + k.String()))
	return "k" + hex.EncodeToString(sum[:4])
}

func kindLabelOf(k artifact.Kind) string {
	if l, ok := kindLabel[k]; ok {
		return l
	}
	return k.String()
}

func kindLess(a, b artifact.Kind) bool {
	ai, aok := kindOrder[a]
	bi, bok := kindOrder[b]
	if aok && bok {
		return ai < bi
	}
	if aok != bok {
		return aok // listed kinds sort before unlisted
	}
	return a < b
}

// edgeLabel labels a resolved edge: the bucket is always "resolved", refined by
// the version-match annotation, so a diagram reader sees not just that two sides
// resolved but whether their versions agreed.
func edgeLabel(e resolve.ResolvedEdge) string {
	return string(BucketResolved) + ":" + string(e.VersionMatch)
}

func provenanceString(p artifact.Provenance) string {
	if p.Line > 0 {
		return fmt.Sprintf("%s:%d", p.File, p.Line)
	}
	return p.File
}
