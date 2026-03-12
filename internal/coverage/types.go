package coverage

// Entity represents a documentable code construct extracted from source.
type Entity struct {
	Name       string // "FuncName" or "Receiver.MethodName"
	Kind       string // "function", "method", "type", "constant", "variable"
	Package    string // Package name from tree-sitter
	File       string // Source file path (relative)
	Exported   bool
	DocComment string // Extracted doc comment text (for bridging)
}

// DocRef represents a code reference found in documentation.
type DocRef struct {
	Text string // Matched text from backtick or heading
	Kind string // "code_span", "heading"
	File string // Markdown file path (relative)
	Line int
}

// CoverageResult holds the set operation results.
type CoverageResult struct {
	Entities  []Entity
	DocRefs   []DocRef
	Covered   []Entity // C ∩ D
	Uncovered []Entity // C \ D
	Stale     []DocRef // D \ C: doc references with no matching entity
	Coverage  float64  // |C ∩ D| / |C|
	Staleness float64  // |D \ C| / |D|
}
