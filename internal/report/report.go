// Package report renders a resolved artifact/usage graph deterministically, as
// machine-readable JSON or as human Markdown with a mermaid diagram.
//
// It is the terminal stage of the map pipeline: extractors emit facts, the
// resolver partitions them into resolved / external / dangling buckets, and this
// package turns that Result into stable, byte-deterministic output. Every
// collection is sorted by canonical identity key (and provenance) before
// emission, so two runs over the same Result produce identical bytes — the
// property the golden tests assert.
//
// The package depends only on internal/artifact, internal/resolve, and
// internal/extract (for the skipped-extractor record). It does no file I/O of its
// own beyond writing to the io.Writer the caller provides.
package report

import (
	"sort"

	"github.com/agentic-research/assay/internal/artifact"
	"github.com/agentic-research/assay/internal/extract"
	"github.com/agentic-research/assay/internal/resolve"
)

// Bucket names the partition an artifact or edge falls into. These strings are
// part of the JSON contract and the mermaid edge labels, so they are stable.
type Bucket string

const (
	// BucketResolved is an edge whose consumer reference matched a producer in
	// some scanned root.
	BucketResolved Bucket = "resolved"
	// BucketExternal is a consumer whose reference matched no scanned producer:
	// a real outside-world dependency.
	BucketExternal Bucket = "external"
	// BucketDangling is a producer no consumer references: a dead-surface
	// candidate.
	BucketDangling Bucket = "dangling"
)

// Graph is the report-facing view of a resolve.Result: the three buckets plus
// the extractors that were skipped during gathering. A Graph is built from a
// Result with FromResult and rendered by Render.
type Graph struct {
	// Resolved are the producer→consumer edges, sorted by identity then
	// provenance.
	Resolved []resolve.ResolvedEdge
	// External are outside-world consumer dependencies, sorted.
	External []artifact.Consumer
	// Dangling are unreferenced producers, sorted.
	Dangling []artifact.Producer
	// Skipped records extractors that did not run and why, sorted by name. It is
	// provenance for the report: it makes visible what evidence was deliberately
	// left ungathered.
	Skipped []extract.Skipped
	// Failed records per-(extractor, root) extraction failures (e.g. a malformed
	// go.mod), sorted by extractor then root. Like Skipped, it is non-fatal
	// provenance: it makes visible which inputs were skipped because they could
	// not be parsed, rather than silently dropping them or aborting the scan.
	Failed []extract.Failed
	// Roots are the scan roots the facts were gathered from, in scan order. They
	// let the repo-grouped mermaid map each fact's provenance file back to its
	// owning root and draw root→root edges (the repo-dependency-graph shape).
	// Empty when roots are unknown, in which case repo grouping falls back to
	// per-artifact nodes.
	Roots []string
}

// FromResult builds a Graph from a resolve.Result and the skip/failure records
// from the Gather pass that produced it. The Result's buckets are already
// deterministically ordered by Resolve; this copies them so the report owns its
// own slices and additionally imposes a stable order on Skipped and Failed
// (which Gather appends in extractor × root order — stable, but sorted here so
// the report is self-contained).
func FromResult(result *resolve.Result, skipped []extract.Skipped, failed []extract.Failed) *Graph {
	g := &Graph{}
	if result != nil {
		g.Resolved = append(g.Resolved, result.Resolved...)
		g.External = append(g.External, result.External...)
		g.Dangling = append(g.Dangling, result.Dangling...)
	}
	g.Skipped = append(g.Skipped, skipped...)
	sortSkipped(g.Skipped)
	g.Failed = append(g.Failed, failed...)
	sortFailed(g.Failed)
	return g
}

// FromResultWithRoots is FromResult plus the scan roots, recorded so the
// repo-grouped mermaid can attribute provenance files to roots and draw root→root
// edges. Roots are stored longest-first so a nested root matches before a parent.
func FromResultWithRoots(result *resolve.Result, skipped []extract.Skipped, failed []extract.Failed, roots []string) *Graph {
	g := FromResult(result, skipped, failed)
	g.Roots = append(g.Roots, roots...)
	sort.SliceStable(g.Roots, func(i, j int) bool { return len(g.Roots[i]) > len(g.Roots[j]) })
	return g
}

// sortSkipped orders skip records by extractor name, then reason, so the report
// is independent of registration order.
func sortSkipped(s []extract.Skipped) {
	sort.Slice(s, func(i, j int) bool {
		if s[i].Name != s[j].Name {
			return s[i].Name < s[j].Name
		}
		return s[i].Reason < s[j].Reason
	})
}

// sortFailed orders failure records by extractor name, then root, so the report
// is independent of registration × argument order.
func sortFailed(f []extract.Failed) {
	sort.Slice(f, func(i, j int) bool {
		if f[i].Extractor != f[j].Extractor {
			return f[i].Extractor < f[j].Extractor
		}
		return f[i].Root < f[j].Root
	})
}
