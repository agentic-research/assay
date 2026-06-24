package report

import (
	"encoding/json"
	"io"
	"sort"

	"github.com/agentic-research/assay/internal/artifact"
)

// jsonReport is the stable JSON schema. Every slice is sorted before emission so
// the encoded bytes are reproducible. The shape is flat and explicit — artifacts
// carry their identity and bucket, edges reference endpoints inline with
// provenance — so a consumer never has to join across the document.
type jsonReport struct {
	Artifacts []jsonArtifact `json:"artifacts"`
	Edges     []jsonEdge     `json:"edges"`
	Skipped   []jsonSkipped  `json:"skipped,omitempty"`
	Failed    []jsonFailed   `json:"failed,omitempty"`
}

// jsonArtifact is one identified unit and the bucket it landed in. An artifact
// referenced by a resolved edge appears here once (as resolved); external and
// dangling artifacts appear with their respective buckets.
type jsonArtifact struct {
	Kind   string `json:"kind"`
	Ref    string `json:"ref"`
	Bucket Bucket `json:"bucket"`
}

// jsonEdge is a resolved producer→consumer relation with both endpoints'
// provenance and the version-match annotation the resolver computed.
type jsonEdge struct {
	Kind         string       `json:"kind"`
	Producer     jsonEndpoint `json:"producer"`
	Consumer     jsonEndpoint `json:"consumer"`
	VersionMatch string       `json:"version_match"`
}

// jsonEndpoint is one side of an edge: the concrete reference observed there
// (which may carry a version the version-stripped identity drops) and where it
// was found.
type jsonEndpoint struct {
	Ref        string         `json:"ref"`
	Provenance jsonProvenance `json:"provenance"`
}

type jsonProvenance struct {
	File string `json:"file"`
	Line int    `json:"line"`
}

type jsonSkipped struct {
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

// jsonFailed is one per-(extractor, root) extraction failure: an input the
// extractor could not parse (e.g. a malformed go.mod). It is non-fatal
// provenance, parallel to jsonSkipped.
type jsonFailed struct {
	Extractor string `json:"extractor"`
	Root      string `json:"root"`
	Error     string `json:"error"`
}

// RenderJSON writes the graph as indented JSON to w. Output is byte-deterministic
// for a given Graph: artifacts are keyed and sorted by (kind, ref, bucket), edges
// by their resolver order (already identity-sorted), skips by name.
func RenderJSON(w io.Writer, g *Graph) error {
	rep := jsonReport{
		Artifacts: jsonArtifacts(g),
		Edges:     jsonEdges(g),
		Skipped:   jsonSkips(g),
		Failed:    jsonFails(g),
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(rep)
}

// jsonArtifacts collects the distinct artifacts across all buckets. Resolved
// edges contribute both endpoints (as resolved artifacts); external and dangling
// contribute their single artifact. De-duplication is by (kind, ref, bucket) so
// an artifact named by many edges appears once.
func jsonArtifacts(g *Graph) []jsonArtifact {
	seen := make(map[jsonArtifact]struct{})
	var out []jsonArtifact
	add := func(a jsonArtifact) {
		if _, ok := seen[a]; ok {
			return
		}
		seen[a] = struct{}{}
		out = append(out, a)
	}
	for _, e := range g.Resolved {
		add(artifactOf(e.Producer.Identity, BucketResolved))
		add(artifactOf(e.Consumer.Identity, BucketResolved))
	}
	for _, c := range g.External {
		add(artifactOf(c.Identity, BucketExternal))
	}
	for _, p := range g.Dangling {
		add(artifactOf(p.Identity, BucketDangling))
	}
	sort.Slice(out, func(i, j int) bool { return artifactLess(out[i], out[j]) })
	return out
}

func artifactOf(id artifact.Identity, bucket Bucket) jsonArtifact {
	return jsonArtifact{Kind: id.Kind.String(), Ref: id.Ref, Bucket: bucket}
}

func artifactLess(a, b jsonArtifact) bool {
	if a.Kind != b.Kind {
		return a.Kind < b.Kind
	}
	if a.Ref != b.Ref {
		return a.Ref < b.Ref
	}
	return a.Bucket < b.Bucket
}

func jsonEdges(g *Graph) []jsonEdge {
	out := make([]jsonEdge, 0, len(g.Resolved))
	for _, e := range g.Resolved {
		out = append(out, jsonEdge{
			Kind: e.Producer.Identity.Kind.String(),
			Producer: jsonEndpoint{
				Ref: e.Producer.Identity.Ref,
				Provenance: jsonProvenance{
					File: e.Producer.Provenance.File,
					Line: e.Producer.Provenance.Line,
				},
			},
			Consumer: jsonEndpoint{
				Ref: e.Consumer.Identity.Ref,
				Provenance: jsonProvenance{
					File: e.Consumer.Provenance.File,
					Line: e.Consumer.Provenance.Line,
				},
			},
			VersionMatch: string(e.VersionMatch),
		})
	}
	return out
}

func jsonSkips(g *Graph) []jsonSkipped {
	if len(g.Skipped) == 0 {
		return nil
	}
	out := make([]jsonSkipped, 0, len(g.Skipped))
	for _, s := range g.Skipped {
		out = append(out, jsonSkipped{Name: s.Name, Reason: s.Reason})
	}
	return out
}

func jsonFails(g *Graph) []jsonFailed {
	if len(g.Failed) == 0 {
		return nil
	}
	out := make([]jsonFailed, 0, len(g.Failed))
	for _, f := range g.Failed {
		out = append(out, jsonFailed{Extractor: f.Extractor, Root: f.Root, Error: f.Err.Error()})
	}
	return out
}
