// Package cargo extracts artifact facts from Cargo.toml manifests: the crate a
// manifest declares (a Producer, from [package].name) and every crate it depends
// on (Consumers, from the dependency tables). It emits RAW crate-name references
// as facts and never decides whether a dependency is internal or external — that
// is the resolver's job. Extraction is observation; resolution is inference.
//
// A dependency is emitted by its CRATE NAME so it matches a producer of the same
// name across roots: a path, git, or registry dependency
// `leyline-cas-ffi = { git = "..." }` in one repo resolves to the
// `[package] name = "leyline-cas-ffi"` producer in another. When a dependency
// table key is only a local alias for a differently-named crate (the
// `package = "..."` rename), the rename is the identity; the table key is the
// alias. A crate's version requirement is not part of its identity and is
// ignored — there is nothing to strip because facts carry the bare crate name.
//
// Workspace members need no special resolution: Extract walks the whole root, so
// every member crate's own Cargo.toml is parsed directly. A virtual workspace
// root (a [workspace] manifest with no [package]) simply contributes no producer.
package cargo

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/agentic-research/assay/internal/artifact"
	"github.com/agentic-research/assay/internal/extract"
)

// compile-time assertion: Extractor honors the extract.Extractor contract.
var _ extract.Extractor = Extractor{}

// Extractor parses every Cargo.toml under a scan root. It has no environment
// dependencies (pure file parsing), so it is always available.
type Extractor struct{}

// New returns a Cargo Extractor.
func New() Extractor { return Extractor{} }

// Name is the stable identifier used to attribute this extractor's facts.
func (Extractor) Name() string { return "cargo" }

// Available always reports true: parsing a Cargo.toml needs no external binary.
func (Extractor) Available() (bool, string) { return true, "" }

// skipDirs are directory names never worth walking for Cargo.toml files: VCS and
// build caches (Rust's target/ holds vendored manifests), dependency trees, agent
// worktree scratch, and test fixtures of other extractors.
var skipDirs = map[string]struct{}{
	".git":         {},
	".claude":      {},
	"target":       {},
	"node_modules": {},
	"vendor":       {},
	"testdata":     {},
}

// Extract walks the scan root for every Cargo.toml and emits each one's crate
// Producer (its [package].name) and dependency Consumers. It finds member crates
// at any depth, not just <root>/Cargo.toml, so a workspace whose crates live in
// subdirectories still contributes every member's producer and a require
// elsewhere resolves to it. A root with no Cargo.toml is not an error — it simply
// contributes no Cargo facts. Files are visited in lexical order, so output is
// deterministic.
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
		if d.Name() != "Cargo.toml" {
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

// extractFile reads one Cargo.toml and parses its producer and consumer facts. A
// vanished file (a race during the walk) yields nothing rather than an error; any
// other read or parse failure is returned so the Registry records it as a soft,
// per-file Failed and keeps gathering.
func extractFile(path string) ([]artifact.Producer, []artifact.Consumer, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	return parseManifest(path, data)
}
