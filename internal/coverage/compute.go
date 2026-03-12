package coverage

import "strings"

// Compute performs set operations between code entities and doc references.
// It first tries exact matching, then falls back to fuzzy Jaccard similarity
// on camelCase-split tokens when fuzzyThreshold > 0.
func Compute(entities []Entity, refs []DocRef) *CoverageResult {
	return ComputeWithThreshold(entities, refs, DefaultFuzzyThreshold)
}

// ComputeWithThreshold is like Compute but with an explicit fuzzy threshold.
// Set threshold to 0 to disable fuzzy matching (exact only).
func ComputeWithThreshold(entities []Entity, refs []DocRef, fuzzyThreshold float64) *CoverageResult {
	// Index entities by multiple lookup keys.
	type entityEntry struct {
		entity Entity
		tokens []string // camelCase-split tokens for fuzzy matching
	}
	byName := make(map[string]*entityEntry)
	allEntries := make([]*entityEntry, 0, len(entities))
	for i := range entities {
		e := &entities[i]
		entry := &entityEntry{
			entity: *e,
			tokens: Tokenize(e.Name),
		}
		allEntries = append(allEntries, entry)
		// Bare name: "GraphCache"
		byName[e.Name] = entry
		// Qualified: "graph.GraphCache"
		if e.Package != "" {
			byName[e.Package+"."+e.Name] = entry
		}
	}

	// Match doc refs against entities.
	matchedEntities := make(map[string]bool) // entity Name → matched
	var stale []DocRef

	for _, ref := range refs {
		norm := normalizeRef(ref.Text)

		// Try exact match first.
		if entry, ok := byName[norm]; ok {
			matchedEntities[entry.entity.Name] = true
			continue
		}

		// Fuzzy fallback: tokenize the ref and find best Jaccard match.
		if fuzzyThreshold > 0 {
			refTokens := Tokenize(norm)
			bestSim := 0.0
			var bestEntry *entityEntry
			for _, entry := range allEntries {
				sim := Jaccard(entry.tokens, refTokens)
				if sim > bestSim {
					bestSim = sim
					bestEntry = entry
				}
			}
			if bestSim >= fuzzyThreshold && bestEntry != nil {
				matchedEntities[bestEntry.entity.Name] = true
				continue
			}
		}

		stale = append(stale, ref)
	}

	var covered, uncovered []Entity
	for i := range entities {
		if matchedEntities[entities[i].Name] {
			covered = append(covered, entities[i])
		} else {
			uncovered = append(uncovered, entities[i])
		}
	}

	var cov, stal float64
	if len(entities) > 0 {
		cov = float64(len(covered)) / float64(len(entities))
	}
	if len(refs) > 0 {
		stal = float64(len(stale)) / float64(len(refs))
	}

	return &CoverageResult{
		Entities:  entities,
		DocRefs:   refs,
		Covered:   covered,
		Uncovered: uncovered,
		Stale:     stale,
		Coverage:  cov,
		Staleness: stal,
	}
}

// normalizeRef cleans a doc reference for matching.
func normalizeRef(text string) string {
	text = strings.TrimSpace(text)
	// Strip trailing () — "Compute()" → "Compute"
	text = strings.TrimSuffix(text, "()")
	return text
}
