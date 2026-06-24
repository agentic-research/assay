// Package resolve performs the global join that turns gathered Producer/Consumer
// facts — from every scanned root — into the artifact/usage graph. It is where
// repo boundaries become invisible: a consumer in one root and a producer in
// another that share a normalized identity resolve to a single cross-root edge.
//
// Matching keys on the version-stripped identity (decision 0002): two references
// to the same artifact at different versions still resolve, and the version
// delta is recorded on the edge as a VersionMatch rather than dropped. Whether
// an artifact is "external" is COMPUTED — purely a function of whether some
// scanned root produces it — never configured and never a module-path-prefix
// heuristic.
package resolve

import (
	"sort"

	"github.com/agentic-research/assay/internal/artifact"
	"github.com/agentic-research/assay/internal/extract"
)

// ResolvedEdge is a producer→consumer edge created when a consumer reference
// matches a producer present in some scanned root. The endpoints may live in
// different roots — that cross-root case is the whole point. VersionMatch records
// whether the two sides also agreed on version, since matching is on the
// version-stripped identity key.
type ResolvedEdge struct {
	artifact.Edge
	VersionMatch VersionMatch
}

// Result holds the three computed buckets of a resolve pass over all gathered
// facts. The partition is exhaustive over the inputs: every consumer is either
// resolved or external, and every producer is either resolved (referenced by at
// least one consumer) or dangling.
type Result struct {
	// Resolved are edges whose consumer reference matched a producer in some
	// scanned root. A consumer that matches several producers (or a producer
	// matched by several consumers) yields one edge per matching pair.
	Resolved []ResolvedEdge
	// External are consumers whose reference matches NO scanned producer: real
	// dependencies on the outside world.
	External []artifact.Consumer
	// Dangling are producers that no consumer references: dead-surface
	// candidates.
	Dangling []artifact.Producer
}

// Resolve joins consumer references to producer ids across all gathered facts,
// keying on the version-stripped identity so repo boundaries are invisible, and
// partitions the result into the resolved / external / dangling buckets.
//
// All outputs are deterministically ordered (by identity key, then by
// provenance), so repeated runs over the same facts produce byte-identical
// results.
func Resolve(facts *extract.Facts) *Result {
	result := &Result{}
	if facts == nil {
		return result
	}

	// Group producers by version-stripped identity key. Producers under the same
	// key are the candidate sources for any consumer that shares the key.
	producersByKey := make(map[identityKey][]producerEntry)
	for _, p := range facts.Producers {
		id, version := identityRefOf(p.Kind(), p.Identity.Ref)
		key := identityKey{Kind: p.Kind(), Identity: id}
		producersByKey[key] = append(producersByKey[key], producerEntry{
			producer: p,
			version:  version,
		})
	}

	// Track which producer keys actually got consumed; the remainder dangle.
	consumed := make(map[identityKey]bool, len(producersByKey))

	for _, c := range facts.Consumers {
		id, consumerVersion := identityRefOf(c.Kind(), c.Identity.Ref)
		key := identityKey{Kind: c.Kind(), Identity: id}

		producers, ok := producersByKey[key]
		if !ok {
			// No scanned root produces this id: an outside-world dependency.
			result.External = append(result.External, c)
			continue
		}

		consumed[key] = true
		for _, pe := range producers {
			result.Resolved = append(result.Resolved, ResolvedEdge{
				Edge:         artifact.Edge{Producer: pe.producer, Consumer: c},
				VersionMatch: classifyVersionMatch(pe.version, consumerVersion),
			})
		}
	}

	for key, producers := range producersByKey {
		if consumed[key] {
			continue
		}
		for _, pe := range producers {
			result.Dangling = append(result.Dangling, pe.producer)
		}
	}

	result.sort()
	return result
}

// producerEntry pairs a producer with its recovered version so an edge can be
// annotated without re-splitting the ref.
type producerEntry struct {
	producer artifact.Producer
	version  string
}

// sort imposes the deterministic total order on every bucket. Map iteration
// (over producersByKey) is randomized, so dangling in particular must be sorted
// for runs to be reproducible; resolved and external are sorted too so the whole
// Result is order-independent of the input fact order.
func (r *Result) sort() {
	sort.SliceStable(r.Resolved, func(i, j int) bool {
		return resolvedLess(r.Resolved[i], r.Resolved[j])
	})
	sort.SliceStable(r.External, func(i, j int) bool {
		return consumerLess(r.External[i], r.External[j])
	})
	sort.SliceStable(r.Dangling, func(i, j int) bool {
		return producerLess(r.Dangling[i], r.Dangling[j])
	})
}

// resolvedLess orders edges by producer identity, then consumer identity, then
// the provenance of each side — enough to disambiguate every distinct edge.
func resolvedLess(a, b ResolvedEdge) bool {
	if k := compareIdentity(a.Producer.Identity, b.Producer.Identity); k != 0 {
		return k < 0
	}
	if k := compareIdentity(a.Consumer.Identity, b.Consumer.Identity); k != 0 {
		return k < 0
	}
	if k := compareProvenance(a.Producer.Provenance, b.Producer.Provenance); k != 0 {
		return k < 0
	}
	return compareProvenance(a.Consumer.Provenance, b.Consumer.Provenance) < 0
}

func consumerLess(a, b artifact.Consumer) bool {
	if k := compareIdentity(a.Identity, b.Identity); k != 0 {
		return k < 0
	}
	return compareProvenance(a.Provenance, b.Provenance) < 0
}

func producerLess(a, b artifact.Producer) bool {
	if k := compareIdentity(a.Identity, b.Identity); k != 0 {
		return k < 0
	}
	return compareProvenance(a.Provenance, b.Provenance) < 0
}

// compareIdentity orders identities by their canonical key, which embeds the
// kind, so the order is total across kinds.
func compareIdentity(a, b artifact.Identity) int {
	switch ak, bk := a.Key(), b.Key(); {
	case ak < bk:
		return -1
	case ak > bk:
		return 1
	default:
		return 0
	}
}

// compareProvenance orders by file then line, so two facts of the same identity
// from different source locations have a stable relative order.
func compareProvenance(a, b artifact.Provenance) int {
	switch {
	case a.File < b.File:
		return -1
	case a.File > b.File:
		return 1
	case a.Line < b.Line:
		return -1
	case a.Line > b.Line:
		return 1
	default:
		return 0
	}
}
