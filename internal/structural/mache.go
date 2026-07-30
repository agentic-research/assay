// Package structural holds StructuralVerifier backends that check claims
// against canonical, producer-agnostic AST views rather than against a
// name set extracted directly from source.
//
// The tree-sitter backend (internal/code.TreeSitterVerifier) is the default,
// always-available implementation. This package adds a mache-backed backend
// that runs mache's smell-rule engine over canonical v_defs/v_refs views in a
// leyline-parsed .db, so claims can be verified against structural rules
// (e.g. drift_doc_dead_symbol_reference) instead of just matching names.
package structural

import (
	"os/exec"

	"github.com/agentic-research/assay/internal/coverage"
)

// MacheVerifier is a coverage.StructuralVerifier backed by mache's smell-rule
// engine over canonical AST views (v_defs/v_refs) in a leyline-parsed .db.
//
// Honest-experiment seam (mirrors the leyline/HDC backends shipped in dk6.4):
// a fully-wired mache backend cannot truthfully verify claims in isolation
// yet, for two independent reasons —
//
//  1. It requires a built .db carrying the canonical v_defs/v_refs views,
//     produced by `leyline parse` / mache's build path. assay does not build
//     one, so the dependency is environmental, not in-tree.
//  2. The mache rule that maps documentation claims to real symbols —
//     drift_doc_dead_symbol_reference — is itself a v1 PLACEHOLDER in mache
//     today (WHERE 1=0, zero findings) pending a backtick-token preprocessor
//     (tracked upstream under mache-e1b6c8). Wiring to it now would return an
//     empty entity set that looks like "nothing exists" — a false signal.
//
// So this backend is shipped as a selectable-but-guarded seam: Available()
// gates on both the mache binary AND a configured .db, and returns false when
// either is missing. Entities() refuses to fabricate a result when the backend
// is unavailable. The interface is identical to the tree-sitter backend, so
// the two are interchangeable the moment the upstream rule + .db build land.
type MacheVerifier struct {
	// DBPath is the leyline-parsed SQLite database carrying the canonical
	// v_defs/v_refs views the smell-rule engine queries. Empty means the
	// backend has nothing to run against → unavailable.
	DBPath string
}

// NewMacheVerifier returns a mache-backed verifier configured against the
// given leyline-parsed .db. Pass an empty path to leave it unconfigured
// (Available() will report false).
func NewMacheVerifier(dbPath string) MacheVerifier {
	return MacheVerifier{DBPath: dbPath}
}

// Name identifies this backend in reports.
func (v MacheVerifier) Name() string { return "mache" }

// Available reports whether the mache backend can run: the `mache` binary
// must be on PATH AND a .db must be configured. Either missing → false, so
// callers fall back to the tree-sitter backend rather than getting a faked
// or empty result.
func (v MacheVerifier) Available() bool {
	if v.DBPath == "" {
		return false
	}
	_, err := exec.LookPath("mache")
	return err == nil
}

// Entities would query mache's canonical v_defs view (via `mache find-smells`
// over the smell-rule engine) to derive the entity set the code actually has.
//
// It is intentionally unimplemented while the upstream dependency is a
// placeholder: returning an empty or fabricated set here would be a false
// "nothing exists" signal. It instead returns ErrMacheBackendUnavailable so
// the seam is honest and the failure mode is explicit. Once mache's
// drift_doc_dead_symbol_reference rule fires for real and a .db build is in
// place, this is where the shell-out + JSON parse lands — see leyline.Search
// in internal/embeddings for the established CLI-shell-out shape.
func (v MacheVerifier) Entities() ([]coverage.Entity, error) {
	return nil, ErrMacheBackendUnavailable
}

// Compile-time assertion that MacheVerifier implements the interface.
var _ coverage.StructuralVerifier = MacheVerifier{}
