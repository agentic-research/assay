// Package gocode is the Go-code extractor: it observes a scan root's Go source
// and emits package-symbol producers and import/reference consumers, with
// provenance, in the internal/artifact vocabulary.
//
// It has two backends behind one Extractor (decision 0001, mache coupling):
//
//   - tree-sitter (always available) — wraps the in-tree internal/code
//     extraction. This is the floor: the pipeline is never blocked on mache.
//   - mache .db (preferred when present) — opens a mache .db read-only via the
//     canonical v_defs/v_refs views (pure-Go modernc.org/sqlite, no cgo, mache
//     need not be running) and emits equivalent facts with mache's structural
//     fidelity.
//
// Selection: if a mache .db is configured AND readable, the mache backend is
// active; otherwise the tree-sitter floor is active. Available() is honest —
// the Extractor itself is always available (the floor), but when a mache .db
// was explicitly requested and is missing, Available() reports false with a
// clear reason rather than silently degrading to an empty success.
package gocode

import (
	"github.com/agentic-research/assay/internal/artifact"
	"github.com/agentic-research/assay/internal/extract"
)

// Extractor is the Go-code extractor. Construct it with New.
type Extractor struct {
	treeSitter   treeSitterBackend
	mache        macheBackend
	macheDBSet   bool // a mache .db was explicitly requested
	exportedOnly bool
}

// Option configures an Extractor.
type Option func(*Extractor)

// WithMacheDB requests the mache .db backend, reading the given .db read-only.
// When set, the mache backend is preferred; if the .db is missing, Available()
// reports the reason (it does not silently fall back).
func WithMacheDB(dbPath string) Option {
	return func(e *Extractor) {
		e.mache = newMacheBackend(dbPath)
		e.macheDBSet = true
	}
}

// WithExportedOnly restricts the tree-sitter floor to exported symbols.
func WithExportedOnly(exportedOnly bool) Option {
	return func(e *Extractor) { e.exportedOnly = exportedOnly }
}

// New returns a Go-code Extractor. With no options it runs the always-available
// tree-sitter backend. Pass WithMacheDB to prefer the mache .db backend.
func New(opts ...Option) *Extractor {
	e := &Extractor{}
	for _, opt := range opts {
		opt(e)
	}
	e.treeSitter = newTreeSitterBackend(e.exportedOnly)
	return e
}

// Name is the stable extractor identifier.
func (e *Extractor) Name() string { return "gocode" }

// useMache reports whether the mache backend is configured AND can run.
func (e *Extractor) useMache() bool {
	if !e.macheDBSet {
		return false
	}
	ok, _ := e.mache.available()
	return ok
}

// ActiveBackend names the backend Extract will use: "mache" when a readable
// .db is configured, else "tree-sitter".
func (e *Extractor) ActiveBackend() string {
	if e.useMache() {
		return "mache"
	}
	return "tree-sitter"
}

// Available reports whether the extractor can run, honestly. The tree-sitter
// floor is always available, so the extractor is available and the reason names
// the active backend — UNLESS a mache .db was explicitly requested but is
// missing, in which case it reports false with that reason rather than silently
// using an empty/faked result.
func (e *Extractor) Available() (bool, string) {
	if e.macheDBSet {
		if ok, reason := e.mache.available(); !ok {
			return false, reason
		}
		return true, "mache backend (.db) active"
	}
	return true, "tree-sitter backend active (always available)"
}

// Extract observes root with the active backend and emits Producer/Consumer
// facts. It never matches edges (resolver's job).
func (e *Extractor) Extract(root string) ([]artifact.Producer, []artifact.Consumer, error) {
	if e.useMache() {
		return e.mache.extract(root)
	}
	return e.treeSitter.extract(root)
}

// Compile-time assertion that *Extractor satisfies the Extractor contract.
var _ extract.Extractor = (*Extractor)(nil)
