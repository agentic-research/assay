package coverage

import (
	"strings"
	"unicode"
)

// DefaultFuzzyThreshold is the minimum Jaccard similarity for a soft match.
const DefaultFuzzyThreshold = 0.4

// Tokenize splits a code identifier or doc text into lowercase subword tokens.
// Handles camelCase, PascalCase, snake_case, dot.qualified, and plain words.
//
//	"NewGraphCache"     → [new, graph, cache]
//	"memory_store"      → [memory, store]
//	"graph.NewStore"    → [graph, new, store]
//	"the graph cache"   → [the, graph, cache]
func Tokenize(s string) []string {
	var tokens []string
	// First split on dots, underscores, spaces, hyphens.
	parts := splitDelimiters(s)
	for _, part := range parts {
		// Then split camelCase within each part.
		tokens = append(tokens, splitCamelCase(part)...)
	}
	return tokens
}

// Jaccard computes the Jaccard similarity between two token sets.
// Returns a value in [0.0, 1.0].
func Jaccard(a, b []string) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 0
	}
	setA := toSet(a)
	setB := toSet(b)

	inter := 0
	for k := range setA {
		if setB[k] {
			inter++
		}
	}

	union := len(setA)
	for k := range setB {
		if !setA[k] {
			union++
		}
	}

	if union == 0 {
		return 0
	}
	return float64(inter) / float64(union)
}

func splitDelimiters(s string) []string {
	return strings.FieldsFunc(s, func(r rune) bool {
		return r == '.' || r == '_' || r == '-' || r == ' ' || r == '/' || r == ':' || r == '(' || r == ')'
	})
}

func splitCamelCase(s string) []string {
	if s == "" {
		return nil
	}

	var tokens []string
	runes := []rune(s)
	start := 0

	for i := 1; i < len(runes); i++ {
		// Split at transitions: lower→Upper or letter→digit or digit→letter
		prev, cur := runes[i-1], runes[i]
		split := false

		if unicode.IsLower(prev) && unicode.IsUpper(cur) {
			split = true // camelCase boundary
		} else if unicode.IsLetter(prev) && unicode.IsDigit(cur) {
			split = true
		} else if unicode.IsDigit(prev) && unicode.IsLetter(cur) {
			split = true
		} else if unicode.IsUpper(prev) && unicode.IsUpper(cur) && i+1 < len(runes) && unicode.IsLower(runes[i+1]) {
			split = true // "HTMLParser" → "HTML", "Parser"
		}

		if split {
			tokens = append(tokens, strings.ToLower(string(runes[start:i])))
			start = i
		}
	}
	tokens = append(tokens, strings.ToLower(string(runes[start:])))

	return tokens
}

func toSet(tokens []string) map[string]bool {
	s := make(map[string]bool, len(tokens))
	for _, t := range tokens {
		if len(t) > 1 { // Skip single-char tokens (articles, noise)
			s[t] = true
		}
	}
	return s
}
