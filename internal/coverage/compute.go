package coverage

import "strings"

// Compute performs set operations between code entities and doc references.
func Compute(entities []Entity, refs []DocRef) *CoverageResult {
	// Index entities by multiple lookup keys.
	type entityEntry struct {
		entity Entity
		seen   bool
	}
	byName := make(map[string]*entityEntry)
	for i := range entities {
		e := &entities[i]
		entry := &entityEntry{entity: *e}
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
		if entry, ok := byName[norm]; ok {
			entry.seen = true
			matchedEntities[entry.entity.Name] = true
		} else {
			stale = append(stale, ref)
		}
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
