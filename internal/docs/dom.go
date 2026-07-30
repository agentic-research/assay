package docs

import (
	"errors"

	"github.com/agentic-research/assay/internal/coverage"
)

// ErrDOMCaptureUnavailable is returned by DOMSource.Claims when the live-DOM
// capture backend is selected but cannot honestly run — the x-ray CDP+ax
// capture stack is not wired in, so there is no rendered page to read claims
// from. It is a sentinel (mirrors structural.ErrMacheBackendUnavailable) so
// callers can distinguish "backend not ready" from a genuine extraction error.
var ErrDOMCaptureUnavailable = errors.New(
	"docs: live-DOM capture unavailable (needs a running Chrome over CDP via " +
		"x-ray's internal/cdp + internal/ax; see internal/docs/dom.go)",
)

// DOMSource is the live-DOM capture seam: a coverage.ClaimSource that would
// extract claim-references from a *rendered* documentation page or UI, rather
// than from static HTML on disk (HTMLSource). It reads the page the way a user
// (or screen reader) actually sees it after JavaScript runs.
//
// Honest-experiment seam (mirrors structural.MacheVerifier on the code side):
// a live capture cannot run in CI because it requires a headless Chrome driven
// over the Chrome DevTools Protocol. So this backend is shipped guarded —
// Available() is false until wired, and Claims() returns
// ErrDOMCaptureUnavailable rather than fabricating an empty result that would
// read as "the page claims nothing".
//
// x-ray reuse decision — ADAPTED, not imported. The intended implementation
// reuses x-ray's proven capture stack:
//
//   - github.com/agentic-research/x-ray/internal/cdp drives a Chrome target
//     over CDP. cdp.PageText (Runtime.evaluate of document.body.innerText) and
//     cdp.MacheBackendMap (DOM.getDocument → backend-node ids) give the
//     rendered HTML/text; cdp.FullAXTree / cdp.CaptureAXAsync pull the
//     accessibility tree.
//   - github.com/agentic-research/x-ray/internal/ax (AXNode, FlattenTree,
//     ToSummaryLines) turns that tree into structured, role-labelled content —
//     headings and code/landmark roles are exactly the claim-bearing elements
//     ExtractHTMLSource keys on for static HTML.
//
// Those packages are NOT imported here for two reasons: (1) cdp is built around
// a live *cdp.Proxy holding an open Chrome connection, so importing it would
// pull chromedp/Chrome into assay's build and make the package un-CI-runnable —
// the same constraint dk6.3 hit with mache; (2) x-ray exposes them under
// internal/, so they are not importable across module boundaries at all. When
// the live path is wired, the rendered HTML from cdp.PageText feeds straight
// into ExtractHTMLSource, and the ax summary supplements it — both behind this
// unchanged interface.
type DOMSource struct {
	// URL is the rendered page to capture. Empty or set, the backend is
	// unavailable until the CDP+ax capture is wired.
	URL string
}

// NewDOMSource returns a DOMSource targeting the given rendered-page URL.
func NewDOMSource(url string) DOMSource {
	return DOMSource{URL: url}
}

// Available reports whether the live-DOM capture backend can run. It is always
// false today: the CDP+ax stack is not wired, and faking availability would let
// callers trust an empty claim set. Returns false so callers fall back to the
// static HTMLSource rather than getting a fabricated result.
func (s DOMSource) Available() bool { return false }

// Claims would capture the rendered page over CDP and extract claim-references
// from its HTML + accessibility tree. It is intentionally unimplemented while
// the live capture stack is unwired: returning an empty set here would be a
// false "page claims nothing" signal, so it returns ErrDOMCaptureUnavailable
// instead. When wired, cdp.PageText's rendered HTML feeds ExtractHTMLSource and
// the ax summary supplements it — see the type doc for the x-ray reuse plan.
func (s DOMSource) Claims() ([]coverage.DocRef, error) {
	return nil, ErrDOMCaptureUnavailable
}

// Compile-time assertion that DOMSource implements the interface.
var _ coverage.ClaimSource = DOMSource{}
