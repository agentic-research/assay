package coverage

// ClaimSource yields claim-references extracted from some artifact format.
//
// A claim-reference (DocRef) carries the claim text, its provenance (Kind),
// and its location (File, Line) — independent of the format it came from.
// Markdown is the first implementation; HTML/DOM, code comments, and ADRs
// are future sources that plug in behind the same interface.
type ClaimSource interface {
	Claims() ([]DocRef, error)
}

// ComputeFromSources gathers claim-references from one or more sources and
// computes coverage against the given entities. Claims from every source are
// merged (deduplicated by text+file+line) before matching.
//
// It consumes the ClaimSource interface rather than a concrete extractor, so
// any format that yields DocRefs participates in coverage uniformly.
func ComputeFromSources(entities []Entity, fuzzyThreshold float64, sources ...ClaimSource) (*CoverageResult, error) {
	var refs []DocRef
	for _, src := range sources {
		found, err := src.Claims()
		if err != nil {
			return nil, err
		}
		refs = MergeRefs(refs, found)
	}
	return ComputeWithThreshold(entities, refs, fuzzyThreshold), nil
}

// MergeRefs combines two ref slices, deduplicating by text+file+line.
func MergeRefs(a, b []DocRef) []DocRef {
	seen := make(map[refKey]bool, len(a)+len(b))
	merged := make([]DocRef, 0, len(a)+len(b))
	for _, refs := range [][]DocRef{a, b} {
		for _, r := range refs {
			key := refKey{r.Text, r.File, r.Line}
			if !seen[key] {
				seen[key] = true
				merged = append(merged, r)
			}
		}
	}
	return merged
}

// refKey identifies a DocRef for deduplication.
type refKey struct {
	text string
	file string
	line int
}
