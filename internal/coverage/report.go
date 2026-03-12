package coverage

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
)

// FormatText writes a human-readable coverage report.
func FormatText(w io.Writer, result *CoverageResult, verbose bool) {
	fmt.Fprintf(w, "assay: documentation coverage report\n")
	fmt.Fprintf(w, "====================================\n\n")

	pkgs := countPackages(result.Entities)
	docFiles := countDocFiles(result.DocRefs)

	fmt.Fprintf(w, "Source: %d packages, %d exported entities\n", pkgs, len(result.Entities))
	fmt.Fprintf(w, "Docs:   %d files, %d code references\n\n", docFiles, len(result.DocRefs))

	fmt.Fprintf(w, "Coverage:  %d/%d (%.1f%%)\n", len(result.Covered), len(result.Entities), result.Coverage*100)
	fmt.Fprintf(w, "Staleness: %d/%d (%.1f%%)\n", len(result.Stale), len(result.DocRefs), result.Staleness*100)

	if len(result.Uncovered) > 0 {
		fmt.Fprintf(w, "\nUncovered (%d):\n", len(result.Uncovered))
		sorted := sortEntities(result.Uncovered)
		for _, e := range sorted {
			fmt.Fprintf(w, "  %-10s %-30s %s\n", e.Kind, e.Name, e.File)
		}
	}

	if len(result.Stale) > 0 {
		fmt.Fprintf(w, "\nStale (%d):\n", len(result.Stale))
		for _, r := range result.Stale {
			fmt.Fprintf(w, "  `%s`  %s:%d\n", r.Text, r.File, r.Line)
		}
	}

	if verbose && len(result.Covered) > 0 {
		fmt.Fprintf(w, "\nCovered (%d):\n", len(result.Covered))
		sorted := sortEntities(result.Covered)
		for _, e := range sorted {
			fmt.Fprintf(w, "  %-10s %-30s %s\n", e.Kind, e.Name, e.File)
		}
	}
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
