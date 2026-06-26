package capnp

import (
	"bufio"
	"bytes"
	"strings"
)

// constField is one `key = value` assignment observed inside a const data
// block, with the 1-based source line where it sits. A string assignment carries
// its content in value; a non-string assignment (a const reference, number,
// enum, list, or nested record) carries an empty value, recording the key's
// presence only. The topology we resolve on is entirely string-typed (names,
// images, service targets); the presence-only form exists so a marker key such
// as `worker = .cloisterWorker` can still be observed without parsing capnp's
// full value grammar.
type constField struct {
	key   string
	value string
	line  int
}

// hasConst reports whether a capnp file contains at least one `const`
// declaration. A file with none is pure schema (struct/interface/annotation
// defs) and carries no runtime data to observe, so the extractor skips it
// entirely rather than risk a false positive from a struct's field names.
func hasConst(data []byte) bool {
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for sc.Scan() {
		if isConstDecl(stripComment(sc.Text())) {
			return true
		}
	}
	return false
}

// isConstDecl reports whether a comment-stripped line begins a `const`
// declaration: the keyword `const` as the first token. capnp const decls are
// always top-level, so a leading-whitespace check is enough to avoid matching a
// field named e.g. `constraint`.
func isConstDecl(line string) bool {
	t := strings.TrimSpace(line)
	return t == "const" || strings.HasPrefix(t, "const ")
}

// scanFields walks a capnp file's bytes once and returns every `key = value`
// field it observes (string assignments with their content, non-string ones as
// presence-only), with provenance lines. It is intentionally shallow: it does
// not model capnp's nested-record grammar, it simply harvests the flat sequence
// of assignments in source order. Because every identity we emit (a bundle name,
// an image, a wired service, a binding's service target) is a distinct key, the
// flat harvest is sufficient and robust to whitespace and nesting changes in the
// const layout.
//
// Comments (`# ...`) are stripped before matching so a `#` inside prose never
// looks like a key. A `=` inside a quoted string is not a field separator: the
// value scan starts at the first quote after the `=`, so only the quoted span is
// captured.
func scanFields(data []byte) []constField {
	var fields []constField
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	line := 0
	for sc.Scan() {
		line++
		text := stripComment(sc.Text())
		for _, f := range fieldsOnLine(text) {
			f.line = line
			fields = append(fields, f)
		}
	}
	return fields
}

// fieldsOnLine extracts every `key = value` assignment on a single
// comment-stripped line. capnp permits several comma-separated fields per line
// (e.g. `metadata = ( name = "x", version = "y" )`), so the line is scanned
// left to right rather than split on the first `=`.
//
// A string value (`key = "..."`) is captured with its content; a non-string
// value (`key = .ref`, `key = 7532`, `key = cluster`, a list, a nested record)
// is captured as the key with an empty value. The empty-valued form is what lets
// the emit policy observe a presence-only key like `worker = .cloisterWorker`
// (a const reference, never a string) without parsing capnp's full value grammar.
func fieldsOnLine(line string) []constField {
	var out []constField
	rest := line
	for {
		eq := strings.IndexByte(rest, '=')
		if eq < 0 {
			break
		}
		key := lastIdent(rest[:eq])
		after := rest[eq+1:]
		if key == "" {
			// No identifier before this `=` (e.g. a `==` fragment, or list
			// punctuation): skip it and keep scanning.
			rest = after
			continue
		}
		q := strings.IndexByte(after, '"')
		nextEq := strings.IndexByte(after, '=')
		if q < 0 || (nextEq >= 0 && nextEq < q) {
			// Either no string follows, or the next `=` precedes the quote so the
			// quote belongs to an INNER field of a nested record
			// (`kind = (external = ( image = "..." ))`). Record the key with an
			// empty value (presence only) and advance past this `=` so any inner
			// `key = "value"` is still parsed on the next iteration.
			out = append(out, constField{key: key})
			rest = after
			continue
		}
		valEnd := strings.IndexByte(after[q+1:], '"')
		if valEnd < 0 {
			break // unterminated string: nothing more parseable on this line
		}
		out = append(out, constField{
			key:   key,
			value: after[q+1 : q+1+valEnd],
		})
		rest = after[q+1+valEnd+1:]
	}
	return out
}

// lastIdent returns the trailing capnp identifier in s (the key immediately left
// of an `=`), or "" if s does not end in an identifier. Leading record syntax
// (`(`, `,`, whitespace) is discarded, so `( name` yields `name`.
func lastIdent(s string) string {
	s = strings.TrimRight(s, " \t")
	end := len(s)
	i := end
	for i > 0 && isIdentByte(s[i-1]) {
		i--
	}
	if i == end {
		return ""
	}
	return s[i:end]
}

// isIdentByte reports whether b is a legal capnp identifier byte.
func isIdentByte(b byte) bool {
	return b == '_' ||
		(b >= 'a' && b <= 'z') ||
		(b >= 'A' && b <= 'Z') ||
		(b >= '0' && b <= '9')
}

// stripComment removes a trailing `# ...` comment from a line. A `#` inside a
// quoted string is preserved: the scan tracks quote state so only a `#` outside
// a string ends the line.
func stripComment(line string) string {
	inStr := false
	for i := 0; i < len(line); i++ {
		switch line[i] {
		case '"':
			inStr = !inStr
		case '#':
			if !inStr {
				return line[:i]
			}
		}
	}
	return line
}
