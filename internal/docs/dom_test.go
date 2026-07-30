package docs

import (
	"errors"
	"testing"

	"github.com/agentic-research/assay/internal/coverage"
)

// DOMSource is the live-capture seam: it implements coverage.ClaimSource but,
// until the x-ray CDP+ax capture stack is wired, it is never Available() and
// never fabricates claims — it returns the sentinel error so the failure mode
// is explicit (mirrors structural.MacheVerifier on the code side).
func TestDOMSource_ImplementsClaimSource(t *testing.T) {
	var _ coverage.ClaimSource = NewDOMSource("http://example.test")
}

func TestDOMSource_UnavailableByDefault(t *testing.T) {
	s := NewDOMSource("http://example.test")
	if s.Available() {
		t.Fatal("Available() = true, want false (CDP capture is not wired)")
	}
}

func TestDOMSource_ClaimsRefusesToFakeSuccess(t *testing.T) {
	s := NewDOMSource("http://example.test")
	got, err := s.Claims()
	if got != nil {
		t.Fatalf("Claims() returned %d refs, want nil (must not fabricate)", len(got))
	}
	if !errors.Is(err, ErrDOMCaptureUnavailable) {
		t.Fatalf("Claims() error = %v, want ErrDOMCaptureUnavailable", err)
	}
}
