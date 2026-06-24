package report

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"
)

// RenderMermaidRepo writes a repo-level mermaid diagram: one node per scan root,
// with a consumer→producer edge for every resolved edge whose producer and
// consumer live in different roots. This is the repo-dependency-graph shape — the
// same "A depends on B" view a hand-maintained ecosystem diagram draws (the arrow
// points from the depending repo to the one it depends on) — collapsing the
// thousands of artifact-level facts into the cross-repo structure they imply.
//
// It needs g.Roots to attribute provenance files to roots; with no roots it
// degrades to the artifact-level diagram so the command always emits something.
// Output is byte-deterministic: roots and edges are sorted by label.
func RenderMermaidRepo(w io.Writer, g *Graph) error {
	if len(g.Roots) == 0 {
		return RenderMermaid(w, g)
	}
	_, err := io.WriteString(w, repoDiagram(g))
	return err
}

// repoEdge is one directed root→root dependency with how many artifact-level
// edges grounded it — the count makes a thick seam (many shared artifacts)
// distinguishable from an incidental one.
type repoEdge struct {
	from  string
	to    string
	count int
}

func repoDiagram(g *Graph) string {
	labels := repoLabels(g.Roots)

	// Aggregate resolved edges to the repo level. An edge counts only when its
	// producer and consumer resolve to DIFFERENT roots: same-root edges are
	// intra-repo and would render as self-loops, which carry no cross-repo
	// structure.
	edges := make(map[[2]string]int)
	seenRoots := make(map[string]struct{})
	for _, e := range g.Resolved {
		pr := rootOf(e.Producer.Provenance.File, g.Roots, labels)
		co := rootOf(e.Consumer.Provenance.File, g.Roots, labels)
		if pr == "" || co == "" {
			continue
		}
		seenRoots[pr] = struct{}{}
		seenRoots[co] = struct{}{}
		if pr == co {
			continue
		}
		// Arrow points consumer→producer: "the consumer repo depends on the
		// producer repo", matching the hand-maintained graph's convention.
		edges[[2]string{co, pr}]++
	}

	var b strings.Builder
	b.WriteString("graph LR\n")

	// Declare every root node that participated in any resolved edge, sorted.
	roots := make([]string, 0, len(seenRoots))
	for r := range seenRoots {
		roots = append(roots, r)
	}
	sort.Strings(roots)
	for _, r := range roots {
		fmt.Fprintf(&b, "  %s[%q]\n", repoNodeID(r), r)
	}

	// Emit edges sorted by (from, to) so output is deterministic.
	ordered := make([]repoEdge, 0, len(edges))
	for k, n := range edges {
		ordered = append(ordered, repoEdge{from: k[0], to: k[1], count: n})
	}
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].from != ordered[j].from {
			return ordered[i].from < ordered[j].from
		}
		return ordered[i].to < ordered[j].to
	})
	for _, e := range ordered {
		fmt.Fprintf(&b, "  %s -->|%q| %s\n",
			repoNodeID(e.from), edgeCountLabel(e.count), repoNodeID(e.to))
	}

	return b.String()
}

// repoLabels assigns each root a short, unique label: its basename, suffixed with
// a disambiguator when two roots share a basename. The mapping is keyed by the
// root string so rootOf can look a file's root up by prefix.
func repoLabels(roots []string) map[string]string {
	labels := make(map[string]string, len(roots))
	used := make(map[string]int)
	// Stable assignment order: by root string, so labels are reproducible.
	sorted := append([]string(nil), roots...)
	sort.Strings(sorted)
	for _, r := range sorted {
		base := filepath.Base(strings.TrimRight(r, string(filepath.Separator)))
		if base == "" || base == "." {
			base = "root"
		}
		if n := used[base]; n > 0 {
			labels[r] = fmt.Sprintf("%s~%d", base, n)
		} else {
			labels[r] = base
		}
		used[base]++
	}
	return labels
}

// rootOf returns the label of the root that owns file, matching the
// longest root prefix (g.Roots is stored longest-first so a nested root wins over
// its parent). Returns "" when no root claims the file.
func rootOf(file string, roots []string, labels map[string]string) string {
	for _, r := range roots {
		if file == r || strings.HasPrefix(file, r+string(filepath.Separator)) {
			return labels[r]
		}
	}
	return ""
}

func repoNodeID(label string) string {
	sum := sha1.Sum([]byte("repo:" + label))
	return "r" + hex.EncodeToString(sum[:6])
}

// edgeCountLabel labels a repo→repo edge by how many artifact-level resolved
// edges back it.
func edgeCountLabel(count int) string {
	if count == 1 {
		return "1 artifact"
	}
	return fmt.Sprintf("%d artifacts", count)
}
