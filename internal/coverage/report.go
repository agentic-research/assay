package coverage

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
)

// FormatText writes a human-readable coverage report. It returns the first
// write error encountered, if any.
func FormatText(w io.Writer, result *CoverageResult, verbose bool) error {
	var writeErr error
	pf := func(format string, a ...any) {
		if writeErr != nil {
			return
		}
		_, writeErr = fmt.Fprintf(w, format, a...)
	}

	pf("assay: documentation coverage report\n")
	pf("====================================\n\n")

	pkgs := countPackages(result.Entities)
	docFiles := countDocFiles(result.DocRefs)

	pf("Source: %d packages, %d exported entities\n", pkgs, len(result.Entities))
	pf("Docs:   %d files, %d code references\n\n", docFiles, len(result.DocRefs))

	pf("Coverage:  %d/%d (%.1f%%)\n", len(result.Covered), len(result.Entities), result.Coverage*100)
	pf("Staleness: %d/%d (%.1f%%)\n", len(result.Stale), len(result.DocRefs), result.Staleness*100)

	if len(result.Uncovered) > 0 {
		pf("\nUncovered (%d):\n", len(result.Uncovered))
		sorted := sortEntities(result.Uncovered)
		for _, e := range sorted {
			pf("  %-10s %-30s %s\n", e.Kind, e.Name, e.File)
		}
	}

	if len(result.Stale) > 0 {
		pf("\nStale (%d):\n", len(result.Stale))
		for _, r := range result.Stale {
			pf("  `%s`  %s:%d\n", r.Text, r.File, r.Line)
		}
	}

	if verbose && len(result.Covered) > 0 {
		pf("\nCovered (%d):\n", len(result.Covered))
		sorted := sortEntities(result.Covered)
		for _, e := range sorted {
			pf("  %-10s %-30s %s\n", e.Kind, e.Name, e.File)
		}
	}

	return writeErr
}

// FormatJSON writes a JSON coverage report.
func FormatJSON(w io.Writer, result *CoverageResult) error {
	type jsonReport struct {
		Coverage  float64  `json:"coverage"`
		Staleness float64  `json:"staleness"`
		Total     int      `json:"total_entities"`
		Covered   int      `json:"covered_count"`
		Uncovered []Entity `json:"uncovered,omitempty"`
		Stale     []DocRef `json:"stale,omitempty"`
	}
	report := jsonReport{
		Coverage:  result.Coverage,
		Staleness: result.Staleness,
		Total:     len(result.Entities),
		Covered:   len(result.Covered),
		Uncovered: result.Uncovered,
		Stale:     result.Stale,
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}

func countPackages(entities []Entity) int {
	pkgs := make(map[string]bool)
	for _, e := range entities {
		key := e.Package
		if key == "" {
			key = strings.SplitN(e.File, "/", 2)[0]
		}
		pkgs[key] = true
	}
	return len(pkgs)
}

func countDocFiles(refs []DocRef) int {
	files := make(map[string]bool)
	for _, r := range refs {
		files[r.File] = true
	}
	return len(files)
}

func sortEntities(entities []Entity) []Entity {
	sorted := make([]Entity, len(entities))
	copy(sorted, entities)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Kind != sorted[j].Kind {
			return sorted[i].Kind < sorted[j].Kind
		}
		return sorted[i].Name < sorted[j].Name
	})
	return sorted
}
