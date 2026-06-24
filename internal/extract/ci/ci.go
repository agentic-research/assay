// Package ci extracts container-image and CLI-binary facts from GitHub Actions
// workflow YAML.
//
// It reads every workflow under <root>/.github/workflows/*.yml|*.yaml and emits:
//
//   - Producers: container images the workflow declares — most importantly
//     images named in a workflow- or job-level env: block with
//     ${{ github.* }} / ${{ env.* }} interpolation (e.g. rosary's
//     IMAGE_NAME: ghcr.io/${{ github.repository_owner }}/rosary), plus images
//     built/pushed in run: steps.
//   - Consumers: images pulled (docker pull/run …) and binaries invoked in
//     run: steps.
//
// It does NOT assume docker/build-push-action is present; image names are read
// generically from env: and from run: scripts. Identities are normalized per
// decision 0002: image refs key on registry/repository (tag/digest stripped),
// and ${{ … }} expressions are resolved against the interpolation context
// before normalization.
//
// The extractor only observes facts and carries provenance (workflow file path
// relative to the scan root, plus 1-based line). It never matches Consumers to
// Producers — that is the resolver's job.
package ci

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/agentic-research/assay/internal/artifact"
	"gopkg.in/yaml.v3"
)

// defaultRepositoryOwner is the owner ${{ github.repository_owner }} resolves to
// when no override is supplied. Per decision 0002 the corpus owner is
// agentic-research; callers that scan another owner's repos pass
// WithRepositoryOwner.
const defaultRepositoryOwner = "agentic-research"

// Extractor parses GitHub Actions workflows under a scan root and emits image /
// binary facts. It is always Available.
type Extractor struct {
	owner string // value for ${{ github.repository_owner }} interpolation
}

// Option configures an Extractor.
type Option func(*Extractor)

// WithRepositoryOwner sets the literal that ${{ github.repository_owner }}
// resolves to during interpolation (decision 0002: the owner segment of the
// scan root's origin remote).
func WithRepositoryOwner(owner string) Option {
	return func(e *Extractor) { e.owner = owner }
}

// New returns a CI workflow extractor.
func New(opts ...Option) *Extractor {
	e := &Extractor{owner: defaultRepositoryOwner}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// Name identifies the extractor.
func (e *Extractor) Name() string { return "ci" }

// Available is always true: parsing YAML needs no external binary.
func (e *Extractor) Available() (bool, string) { return true, "" }

// Extract walks <root>/.github/workflows/*.yml|*.yaml and emits the image and
// binary facts it observes, each with provenance relative to root. A root with
// no workflows yields no facts and no error.
func (e *Extractor) Extract(root string) ([]artifact.Producer, []artifact.Consumer, error) {
	files, err := workflowFiles(root)
	if err != nil {
		return nil, nil, err
	}

	var producers []artifact.Producer
	var consumers []artifact.Consumer
	for _, abs := range files {
		rel, err := filepath.Rel(root, abs)
		if err != nil {
			rel = abs
		}
		ps, cs, err := e.extractFile(abs, rel)
		if err != nil {
			return nil, nil, fmt.Errorf("ci: %s: %w", rel, err)
		}
		producers = append(producers, ps...)
		consumers = append(consumers, cs...)
	}
	return producers, consumers, nil
}

// workflowFiles returns the absolute paths of every workflow file under
// root/.github/workflows, sorted for deterministic output.
func workflowFiles(root string) ([]string, error) {
	dir := filepath.Join(root, ".github", "workflows")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("ci: read workflows dir: %w", err)
	}
	var files []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		switch strings.ToLower(filepath.Ext(entry.Name())) {
		case ".yml", ".yaml":
			files = append(files, filepath.Join(dir, entry.Name()))
		}
	}
	sort.Strings(files)
	return files, nil
}

// extractFile parses one workflow and emits its facts. rel is the file path
// relative to the scan root, used for provenance.
func (e *Extractor) extractFile(abs, rel string) ([]artifact.Producer, []artifact.Consumer, error) {
	data, err := os.ReadFile(abs)
	if err != nil {
		return nil, nil, err
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, nil, fmt.Errorf("parse yaml: %w", err)
	}
	root := documentRoot(&doc)
	if root == nil {
		return nil, nil, nil // empty workflow
	}

	// Resolve the workflow-level env: block first so env.* references in
	// nested env: blocks and run: scripts can be expanded against it.
	interp := newInterpolator(e.owner, collectEnv(root))

	wf := &workflow{rel: rel, interp: interp}
	wf.walk(root)
	return wf.dedupe()
}

// documentRoot returns the mapping node at the top of a parsed workflow, or nil.
func documentRoot(doc *yaml.Node) *yaml.Node {
	if doc.Kind == yaml.DocumentNode {
		if len(doc.Content) == 0 {
			return nil
		}
		doc = doc.Content[0]
	}
	if doc.Kind != yaml.MappingNode {
		return nil
	}
	return doc
}
