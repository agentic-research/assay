package embeddings

import (
	"bufio"
	"bytes"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// SearchResult holds a single kNN result from leyline search.
type SearchResult struct {
	NodeID   string
	Distance float64
}

// EmbedNodes shells out to `leyline embed` to embed all file nodes from a DB.
// Returns the count of nodes embedded.
func EmbedNodes(dbPath, vecDBPath string) (int, error) {
	cmd := exec.Command("leyline", "embed",
		"--db", dbPath,
		"--vec-db", vecDBPath,
		"--model", "minilm-q",
		"--batch-size", "32",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return 0, fmt.Errorf("leyline embed: %w: %s", err, out)
	}
	// Parse "<N> nodes embedded"
	s := strings.TrimSpace(string(out))
	parts := strings.Fields(s)
	if len(parts) >= 1 {
		n, err := strconv.Atoi(parts[0])
		if err == nil {
			return n, nil
		}
	}
	return 0, nil
}

// Search shells out to `leyline search` for kNN vector similarity.
// queryVec is a JSON array of float32 values (384-dim for MiniLM).
// Returns results sorted by distance (ascending = closest).
func Search(vecDBPath, queryVec string, k int) ([]SearchResult, error) {
	cmd := exec.Command("leyline", "search",
		"--db", vecDBPath,
		"--query", queryVec,
		"--k", strconv.Itoa(k),
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("leyline search: %w: %s", err, out)
	}
	return parseSearchOutput(out)
}

// Available returns true if the leyline CLI is found in PATH.
func Available() bool {
	_, err := exec.LookPath("leyline")
	return err == nil
}

func parseSearchOutput(data []byte) ([]SearchResult, error) {
	var results []SearchResult
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || line == "(no results)" {
			continue
		}
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) != 2 {
			continue
		}
		dist, err := strconv.ParseFloat(parts[0], 64)
		if err != nil {
			continue
		}
		results = append(results, SearchResult{
			NodeID:   parts[1],
			Distance: dist,
		})
	}
	return results, scanner.Err()
}
