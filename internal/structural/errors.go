package structural

import "errors"

// ErrMacheBackendUnavailable is returned by MacheVerifier.Entities when the
// mache backend is selected but cannot honestly run — the `mache` binary or
// the canonical-view .db is missing, or the upstream claim-mapping rule is
// still a placeholder. It is a sentinel so callers can distinguish "backend
// not ready" from a genuine extraction error and fall back accordingly.
var ErrMacheBackendUnavailable = errors.New(
	"structural: mache backend unavailable (needs `mache` on PATH, a leyline-parsed .db, " +
		"and the upstream drift_doc_dead_symbol_reference rule; see internal/structural/mache.go)",
)
