package ci

import (
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// collectEnv reads a workflow's top-level env: mapping into a name→value map.
// These are the values that ${{ env.NAME }} references resolve against
// (decision 0002: workflow env: is consulted before the scan context).
func collectEnv(root *yaml.Node) map[string]string {
	env := map[string]string{}
	for i := 0; i+1 < len(root.Content); i += 2 {
		if root.Content[i].Value != "env" {
			continue
		}
		val := root.Content[i+1]
		if val.Kind != yaml.MappingNode {
			break
		}
		for j := 0; j+1 < len(val.Content); j += 2 {
			k, v := val.Content[j], val.Content[j+1]
			if v.Kind == yaml.ScalarNode {
				env[k.Value] = v.Value
			}
		}
		break
	}
	return env
}

// exprRe matches a single ${{ … }} GitHub Actions expression.
var exprRe = regexp.MustCompile(`\$\{\{([^}]*)\}\}`)

// interpolator resolves ${{ … }} expressions to literals using the repository
// owner and the workflow's env: block, per decision 0002's interpolation pass.
type interpolator struct {
	owner string
	env   map[string]string
}

func newInterpolator(owner string, env map[string]string) *interpolator {
	return &interpolator{owner: owner, env: env}
}

// resolve substitutes every ${{ … }} expression in s. It returns the resolved
// string and ok=true only when every expression resolved to a literal; if any
// expression is unresolvable (a secret, a matrix value, an unknown context),
// it returns ok=false so the caller can exclude the ref from matching rather
// than emit a half-interpolated identity.
func (in *interpolator) resolve(s string) (string, bool) {
	return in.resolveDepth(s, 0)
}

// resolveDepth bounds env.* → env.* indirection so a self-referential env block
// cannot loop forever.
func (in *interpolator) resolveDepth(s string, depth int) (string, bool) {
	if !strings.Contains(s, "${{") {
		return s, true
	}
	if depth > 10 {
		return s, false
	}

	resolved := true
	out := exprRe.ReplaceAllStringFunc(s, func(match string) string {
		inner := strings.TrimSpace(exprRe.FindStringSubmatch(match)[1])
		lit, ok := in.resolveExpr(inner, depth)
		if !ok {
			resolved = false
			return match // leave intact; caller sees ${{ … }} remains
		}
		return lit
	})
	if !resolved {
		return out, false
	}
	// A resolved env.* value may itself contain expressions; resolve again.
	if strings.Contains(out, "${{") {
		return in.resolveDepth(out, depth+1)
	}
	return out, true
}

// resolveExpr resolves one expression's body (the text between ${{ and }}).
func (in *interpolator) resolveExpr(expr string, depth int) (string, bool) {
	switch {
	case expr == "github.repository_owner":
		if in.owner == "" {
			return "", false
		}
		return in.owner, true
	case strings.HasPrefix(expr, "env."):
		name := strings.TrimPrefix(expr, "env.")
		val, ok := in.env[name]
		if !ok {
			return "", false
		}
		// The env value may reference further expressions.
		return in.resolveDepth(val, depth+1)
	default:
		// secrets.*, matrix.*, inputs.*, github.ref_name, etc. are not
		// statically resolvable here.
		return "", false
	}
}

// imageIdentity reports whether ref looks like a container image reference and,
// if so, returns its identity ref: registry/repository with any :tag or @digest
// stripped (decision 0002 identity key). The registry host is lowercased; the
// repository path is left as-is (OCI is case-sensitive there).
//
// A string is treated as an image only when its first path segment is a
// registry host — it contains a "." or a ":" (port). This excludes bare local
// paths and plain env values that are not images. Bare Docker Hub names
// (no registry, e.g. "golang") are intentionally NOT inferred here: CI env
// blocks declaring first-party images always use an explicit registry, and the
// inference would misclassify ordinary scalar values.
func imageIdentity(ref string) (string, bool) {
	ref = strings.TrimSpace(ref)
	if ref == "" || strings.ContainsAny(ref, " \t") {
		return "", false
	}

	// Strip digest first (@sha256:…), then tag (:tag) from the final segment.
	repo := ref
	if at := strings.Index(repo, "@"); at >= 0 {
		repo = repo[:at]
	}
	repo = stripTag(repo)

	slash := strings.IndexByte(repo, '/')
	if slash < 0 {
		return "", false // bare name: not inferred (see doc comment)
	}
	host := repo[:slash]
	if !strings.ContainsAny(host, ".:") && host != "localhost" {
		return "", false // first segment is not a registry host
	}

	host = strings.ToLower(host)
	return host + repo[slash:], true
}

// stripTag removes a trailing :tag from an image reference, taking care not to
// mistake a registry :port (which precedes a "/") for a tag.
func stripTag(ref string) string {
	colon := strings.LastIndexByte(ref, ':')
	if colon < 0 {
		return ref
	}
	// A colon before the last "/" is a registry port, not a tag.
	if slash := strings.LastIndexByte(ref, '/'); slash > colon {
		return ref
	}
	return ref[:colon]
}

// splitScriptLines splits a run: script into its lines. Continuation lines
// ending in "\" are joined so a wrapped command is read as one invocation, but
// the offset of the first line is preserved for provenance.
func splitScriptLines(script string) []string {
	raw := strings.Split(script, "\n")
	out := make([]string, len(raw))
	var carry strings.Builder
	carryIdx := -1
	for i, line := range raw {
		trimmed := strings.TrimRight(line, " \t")
		if strings.HasSuffix(trimmed, "\\") {
			if carryIdx < 0 {
				carryIdx = i
			}
			carry.WriteString(strings.TrimSuffix(trimmed, "\\"))
			carry.WriteByte(' ')
			continue
		}
		if carryIdx >= 0 {
			carry.WriteString(line)
			out[carryIdx] = carry.String()
			carry.Reset()
			carryIdx = -1
			continue
		}
		out[i] = line
	}
	if carryIdx >= 0 {
		out[carryIdx] = carry.String()
	}
	return out
}

// commandFields tokenizes a single shell line into whitespace-separated fields,
// dropping leading env-assignment prefixes (FOO=bar cmd) and a leading "sudo".
// It returns nil for blank lines, comments, and lines that are pure shell
// control flow rather than a command.
func commandFields(line string) []string {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return nil
	}
	fields := strings.Fields(line)

	// Drop leading VAR=value assignment prefixes and sudo, e.g.
	// `CGO_ENABLED=0 go build` → `go build`. A bare `VAR=$(...)` or
	// `VAR=value` with no trailing command is a plain assignment, not a
	// prefixed invocation, and yields no command.
	for len(fields) > 0 {
		head := fields[0]
		if head == "sudo" {
			fields = fields[1:]
			continue
		}
		if !envAssignmentPrefix(head) {
			break
		}
		fields = fields[1:]
	}
	return fields
}

// envAssignmentPrefix reports whether tok is a simple NAME=value environment
// prefix that precedes a command (e.g. CGO_ENABLED=0). It is NOT one when the
// value contains shell metacharacters — `LL_PKG=$(find …)` is a command
// substitution assignment, not a prefix, and stripping it would mistake its
// substituted command for the invocation.
func envAssignmentPrefix(tok string) bool {
	eq := strings.IndexByte(tok, '=')
	if eq <= 0 {
		return false
	}
	name := tok[:eq]
	if strings.ContainsAny(name, "/.:$") {
		return false // a path or expression, not a NAME
	}
	value := tok[eq+1:]
	return !strings.ContainsAny(value, "$(`\"'")
}

// dockerImageArgs returns the positional (non-flag) arguments of a docker
// subcommand, stripping flags and their values so the image reference can be
// found. It is heuristic but adequate for the build/push/pull/run forms in the
// corpus.
func dockerImageArgs(args []string) []string {
	var out []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-t" || a == "--tag":
			// `docker build -t <image>`: the image is the flag's value.
			if i+1 < len(args) {
				out = append(out, args[i+1])
				i++
			}
		case strings.HasPrefix(a, "-"):
			// Other flag; skip. We do not consume a value because docker's
			// common flags here (--push, --no-cache, --yes) are boolean.
			continue
		default:
			out = append(out, a)
		}
	}
	return out
}

// shellBuiltins are leading tokens that are shell control, not a program worth
// recording as a CLI consumer.
var shellBuiltins = map[string]struct{}{
	"if": {}, "then": {}, "else": {}, "elif": {}, "fi": {},
	"for": {}, "while": {}, "do": {}, "done": {}, "case": {}, "esac": {},
	"echo": {}, "cd": {}, "export": {}, "set": {}, "true": {}, "false": {},
	"mkdir": {}, "cp": {}, "mv": {}, "rm": {}, "ls": {}, "cat": {}, "test": {},
	"[": {}, "[[": {}, "exit": {}, "return": {}, "source": {}, ".": {},
}

// binaryName returns the invoked binary's base name for a leading command token,
// resolving paths like target/release/rsry → rsry. It returns ok=false for
// shell builtins, blanks, and tokens that still contain unresolved expressions
// or shell metacharacters.
func binaryName(tok string) (string, bool) {
	tok = strings.TrimSpace(tok)
	if tok == "" || strings.Contains(tok, "${{") {
		return "", false
	}
	if strings.ContainsAny(tok, "|&;<>()$`\"'") {
		return "", false
	}
	base := tok
	if slash := strings.LastIndexByte(base, '/'); slash >= 0 {
		base = base[slash+1:]
	}
	if base == "" {
		return "", false
	}
	if _, ok := shellBuiltins[base]; ok {
		return "", false
	}
	return base, true
}
