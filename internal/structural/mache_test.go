package structural

import (
	"errors"
	"testing"

	"github.com/agentic-research/assay/internal/coverage"
)

func TestMacheVerifier_ImplementsInterface(t *testing.T) {
	var v coverage.StructuralVerifier = NewMacheVerifier("")
	if v.Name() != "mache" {
		t.Fatalf("Name() = %q, want %q", v.Name(), "mache")
	}
}

// Unconfigured (no .db) → unavailable, regardless of whether `mache` is on
// PATH. The seam must report false rather than pretend it can run.
func TestMacheVerifier_UnconfiguredIsUnavailable(t *testing.T) {
	v := NewMacheVerifier("")
	if v.Available() {
		t.Fatal("Available() = true with no .db configured, want false")
	}
}

// The backend must never fabricate a result: with no honest way to verify,
// Entities returns the sentinel error rather than an empty/faked entity set.
func TestMacheVerifier_EntitiesRefusesToFakeSuccess(t *testing.T) {
	v := NewMacheVerifier("/nonexistent/canonical-views.db")
	got, err := v.Entities()
	if got != nil {
		t.Fatalf("Entities() returned %d entities, want nil (must not fabricate)", len(got))
	}
	if !errors.Is(err, ErrMacheBackendUnavailable) {
		t.Fatalf("Entities() error = %v, want ErrMacheBackendUnavailable", err)
	}
}
