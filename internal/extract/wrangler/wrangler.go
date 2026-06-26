// Package wrangler extracts service-topology facts from Cloudflare Workers
// configuration (wrangler.toml): the worker a config declares (a Producer, from
// the top-level name) and every other worker it binds to as a service (Consumers,
// from service bindings). It emits RAW service-name references as facts and never
// decides whether a bound service is internal or external — that is the
// resolver's job. Extraction is observation; resolution is inference.
//
// A service binding is emitted by the BOUND SERVICE's name (the `service = "..."`
// target), not the local binding alias, so it matches a producer of the same name
// across roots: cloister's `[[services]] service = "notme-bot"` resolves to
// notme's `name = "notme-bot"` producer. A service has no version concept, so the
// whole name is its identity.
//
// Durable Object bindings ([durable_objects].bindings) are deliberately NOT
// emitted: they name same-worker classes, not cross-worker service edges. Only
// service bindings — which target another deployed worker — become consumers.
package wrangler

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

// Extractor parses every wrangler.toml under a scan root. It has no environment
// dependencies (pure file parsing), so it is always available.
type Extractor struct{}

// New returns a wrangler Extractor.
func New() Extractor { return Extractor{} }

// Name is the stable identifier used to attribute this extractor's facts.
func (Extractor) Name() string { return "wrangler" }

// Available always reports true: parsing a wrangler.toml needs no external
// binary.
func (Extractor) Available() (bool, string) { return true, "" }

// skipDirs are directory names never worth walking for wrangler.toml files: VCS
// and build caches, dependency trees, agent worktree scratch, and test fixtures
// of other extractors.
var skipDirs = map[string]struct{}{
	".git":         {},
	".claude":      {},
	"node_modules": {},
	"vendor":       {},
	"dist":         {},
	"testdata":     {},
}

// Extract walks the scan root for every wrangler.toml and emits each one's worker
// Producer (its top-level name) and service-binding Consumers. It finds configs
// at any depth, not just <root>/wrangler.toml, so a repo whose worker config
// lives in a subdirectory (e.g. notme's worker/wrangler.toml) still contributes
// its producer and a binding elsewhere resolves to it. A root with no
// wrangler.toml is not an error — it simply contributes no service facts. Files
// are visited in lexical order, so output is deterministic.
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
		if d.Name() != "wrangler.toml" {
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

// extractFile reads one wrangler.toml and parses its producer and consumer facts.
// A vanished file (a race during the walk) yields nothing rather than an error;
// any other read or parse failure is returned so the Registry records it as a
// soft, per-file Failed and keeps gathering.
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
