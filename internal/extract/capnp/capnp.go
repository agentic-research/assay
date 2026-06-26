// Package capnp extracts service-topology facts from Cap'n Proto CONST DATA
// declarations — the cluster/config files a deployment owner writes to wire its
// runtime processes together. It surfaces the runtime MCP/proxy edges between
// bundles (cloister→mache, cloister→rosary, cloister→notme) that a hand-drawn
// runtime graph would otherwise have to track by hand.
//
// Scope is deliberately narrow. This extractor reads only `const` data instances
// — the `( name = "...", image = "...", ... )` records a deployment owner
// declares. A capnp file that is pure SCHEMA (only struct/interface/annotation
// definitions, no `const`) carries no runtime data to observe and contributes
// nothing: Available stays true and Extract emits no facts for it. That keeps the
// big manifest schema (manifest/cluster.capnp's struct defs) from producing false
// positives while still mining the sibling const-data files that instantiate it.
//
// Like every extractor here, it emits RAW references as facts and never decides
// whether a referenced service is internal or external — that is the resolver's
// job. Extraction is observation; resolution is inference.
//
// Parse approach: a focused TEXT parser over the const-data records, not the
// `capnp` CLI. `capnp eval`/`compile` would need every imported schema
// (`/cloister/manifest/cluster.capnp`, `/workerd/workerd.capnp`) to resolve from
// each scan root's import path — brittle and environment-dependent across a
// multi-repo scan, and it would make the extractor unavailable wherever the capnp
// toolchain is absent. The const records have a regular `( key = value, ... )`
// shape that parses deterministically from the file bytes alone, with exact
// file+line provenance, so the text parser is both more portable and more
// reproducible here.
package capnp

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/agentic-research/assay/internal/artifact"
	"github.com/agentic-research/assay/internal/extract"
)

// compile-time assertion: Extractor honors the extract.Extractor contract.
var _ extract.Extractor = Extractor{}

// Extractor parses every *.capnp file under a scan root. It has no environment
// dependencies (pure file parsing, no capnp toolchain), so it is always
// available.
type Extractor struct{}

// New returns a capnp Extractor.
func New() Extractor { return Extractor{} }

// Name is the stable identifier used to attribute this extractor's facts.
func (Extractor) Name() string { return "capnp" }

// Available always reports true: the text parser needs no external binary.
func (Extractor) Available() (bool, string) { return true, "" }

// skipDirs are directory names never worth walking for capnp files: VCS and
// build caches, dependency trees, agent worktree scratch, and test fixtures of
// other extractors.
var skipDirs = map[string]struct{}{
	".git":         {},
	".claude":      {},
	"node_modules": {},
	"vendor":       {},
	"dist":         {},
	"target":       {},
	"testdata":     {},
}

// Extract walks the scan root for every *.capnp file and emits the service
// producers/consumers found in each one's const data. It finds files at any
// depth, not just <root>/*.capnp, so a repo whose cluster/config capnp lives in a
// subdirectory still contributes its topology. A root with no const-bearing capnp
// is not an error — it simply contributes no service facts. Files are visited in
// lexical order, so output is deterministic.
func (Extractor) Extract(root string) ([]artifact.Producer, []artifact.Consumer, error) {
	var (
		producers []artifact.Producer
		consumers []artifact.Consumer
	)

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if _, skip := skipDirs[d.Name()]; skip && path != root {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".capnp") {
			return nil
		}
		p, c, err := extractFile(path)
		if err != nil {
			return err
		}
		producers = append(producers, p...)
		consumers = append(consumers, c...)
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return producers, consumers, nil
}

// extractFile reads one capnp file and parses its service facts. A vanished file
// (a race during the walk) yields nothing rather than an error; any other read
// failure is returned so the Registry records it as a soft, per-file Failed and
// keeps gathering.
func extractFile(path string) ([]artifact.Producer, []artifact.Consumer, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	return parseConst(path, data)
}
