package coverage

import (
	"strings"
	"unicode"
)

// DeadRefs returns the doc references that unambiguously name an entity of
// THIS codebase and resolve to nothing — a doc still describing a symbol the
// code no longer has.
//
// It is deliberately much narrower than CoverageResult.Stale. Stale is every
// backtick span that matched no entity, which on a real tree is dominated by
// filenames, CLI flags, bead IDs and other projects' types (see Gate). A
// reference counts here only when its own shape says "this repo's symbol":
//
//   - package-qualified: `pkg.Name` or `pkg.Type.Method`, where pkg is the
//     package name of some extracted entity and Name starts upper-case
//     (exported). `coverage.Gate` qualifies; `wrangler.toml` does not.
//   - receiver-qualified: `Type.Method`, where Type is an extracted type.
//     `MarkdownSource.Claims` qualifies; `Foo.Bar` with no local Foo does not.
//
// A trailing "()" is ignored. Resolution is exact — no fuzzy layer — because a
// fuzzy match is precisely how a deleted `coverage.GateThatWasDeleted` passes
// as "documents Gate".
//
// Bare names (`Gate`) are never checked: a bare identifier cannot be told
// apart from a type quoted from another project, so flagging it would put the
// false-positive rate back where Stale has it. The other blind spot is a
// package whose exported entities were ALL deleted: its name is learnt from
// the entity set, so it stops being recognised as local.
//
// Only code spans are considered; headings are prose.
func DeadRefs(entities []Entity, refs []DocRef) []DocRef {
	packages := make(map[string]bool)
	types := make(map[string]bool)         // "Type"
	qualified := make(map[string]bool)     // "pkg.Name", "pkg.Type.Method"
	receiverNames := make(map[string]bool) // "Type.Method"
	for _, e := range entities {
		if e.Package != "" {
			packages[e.Package] = true
			qualified[e.Package+"."+e.Name] = true
		}
		if e.Kind == "type" {
			types[e.Name] = true
		}
		if strings.Contains(e.Name, ".") {
			receiverNames[e.Name] = true
		}
	}

	var dead []DocRef
	for _, r := range refs {
		if r.Kind != "code_span" {
			continue
		}
		segs, ok := identPath(normalizeRef(r.Text))
		if !ok || len(segs) < 2 {
			continue
		}
		switch {
		case packages[segs[0]] && isExportedIdent(segs[1]):
			// pkg.Name / pkg.Type.Method
			if !qualified[strings.Join(segs, ".")] {
				dead = append(dead, r)
			}
		case len(segs) == 2 && types[segs[0]]:
			// Type.Method
			if !receiverNames[segs[0]+"."+segs[1]] {
				dead = append(dead, r)
			}
		}
	}
	return dead
}

// identPath splits a dotted Go identifier path of two or three segments
// ("pkg.Name", "Type.Method", "pkg.Type.Method"). Anything else — spaces,
// slashes, flags, call arguments — is not an entity reference.
func identPath(s string) ([]string, bool) {
	segs := strings.Split(s, ".")
	if len(segs) > 3 {
		return nil, false
	}
	for _, seg := range segs {
		if !isIdent(seg) {
			return nil, false
		}
	}
	return segs, true
}

func isIdent(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		if r == '_' || unicode.IsLetter(r) || (i > 0 && unicode.IsDigit(r)) {
			continue
		}
		return false
	}
	return true
}

func isExportedIdent(s string) bool {
	for _, r := range s {
		return unicode.IsUpper(r)
	}
	return false
}
