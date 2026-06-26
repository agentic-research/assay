package capnp

import "github.com/agentic-research/assay/internal/artifact"

// parseConst turns one capnp file's const data into service-topology facts. A
// file with no `const` declaration is pure schema and yields nothing, so a
// struct's field names (a schema's `name`/`image` declarations) never become
// false-positive facts.
//
// The kind of every emitted fact is DERIVED from the form the data declares,
// never from a name table:
//
//   - a service binding (`service = "X"`) or a config service entry that fronts
//     a worker → a KindService fact, because the data declares X as a runtime
//     service.
//   - an external bundle's `image = "X"` → a KindContainerImage consumer using
//     the declared image ref, because the data declares the dependency as a
//     container image.
//
// Resolution is then left honest. cloister's config.capnp declares an MCP
// service binding to `notme-bot`, which notme publishes as a service, so that
// edge resolves. Its bindings to `mache-mcp` / `rosary-mcp` name MCP services
// nothing publishes, and its cluster.capnp references mache/rosary by local
// container-image tags (`mache:0.8.0`, `rosary:0.2.0`) that do not match any
// published image, so those fall to External — a true finding, not a gap to
// paper over. Making them resolve is an ecosystem change (the target declaring a
// service identity, or cloister referencing the published image ref), not
// something this extractor should fabricate by cross-kind or name-only matching.
func parseConst(path string, data []byte) ([]artifact.Producer, []artifact.Consumer, error) {
	if !hasConst(data) {
		return nil, nil, nil
	}

	var (
		producers []artifact.Producer
		consumers []artifact.Consumer
		// dedupe per file: the same declared identity (a service bound under
		// several aliases, an image declared once) yields a single fact.
		seenProd = make(map[artifact.Identity]struct{})
		seenCons = make(map[artifact.Identity]struct{})
	)

	rec := newRecordState()
	for _, f := range scanFields(data) {
		rec.observe(f.key)
		prov := artifact.Provenance{File: path, Line: f.line}
		switch f.key {
		case "service":
			// A service binding (config.capnp Worker binding) or a service-typed
			// reference: the data names a runtime service it depends on.
			addConsumer(&consumers, seenCons, artifact.KindService, f.value, prov)
		case "to":
			// A cluster wire's target: a sibling bundle this owner talks to.
			// Both endpoints live in the owner's own capnp, so this resolves to a
			// same-root self-loop (filtered from the repo view); it is emitted so
			// the intra-cluster topology is still visible at the artifact level.
			addConsumer(&consumers, seenCons, artifact.KindService, f.value, prov)
		case "image":
			// An external bundle's OCI image: the dependency is declared as a
			// container image, so it is emitted as one, with the declared ref.
			addConsumer(&consumers, seenCons, artifact.KindContainerImage, f.value, prov)
		case "name":
			// A config service entry that fronts a worker is this owner's own
			// service identity (e.g. config.capnp's `name = "cloister"` with a
			// `worker = ...`). It is a producer. The decision waits until the
			// record closes, because `worker` may follow `name`.
			rec.pendingServiceName = f.value
			rec.pendingServiceProv = prov
		case "worker":
			// Confirms the pending named entry is a worker-backed service.
			if rec.pendingServiceName != "" {
				addProducer(&producers, seenProd, artifact.KindService,
					rec.pendingServiceName, rec.pendingServiceProv)
				rec.pendingServiceName = ""
			}
		}
	}
	return producers, consumers, nil
}

// recordState carries the minimal cross-field context the emit policy needs: a
// config service entry's `name` is only a producer once a later `worker` field
// confirms it fronts a worker. A new `name` (or a `service`/`image`, which mark a
// different record shape) abandons any unconfirmed pending name.
type recordState struct {
	pendingServiceName string
	pendingServiceProv artifact.Provenance
}

func newRecordState() *recordState { return &recordState{} }

// observe resets the pending worker-backed-service candidate when a field that
// cannot belong to the same service entry arrives. A `service` or `image` field
// means the current record is a binding or a bundle, not a worker-fronting
// service entry, so a `name` seen just before it was not a service producer.
func (r *recordState) observe(key string) {
	switch key {
	case "service", "image":
		r.pendingServiceName = ""
	}
}

func addProducer(out *[]artifact.Producer, seen map[artifact.Identity]struct{}, kind artifact.Kind, ref string, prov artifact.Provenance) {
	id := artifact.NewIdentity(kind, ref)
	if id.Ref == "" {
		return
	}
	if _, dup := seen[id]; dup {
		return
	}
	seen[id] = struct{}{}
	*out = append(*out, artifact.Producer{Identity: id, Provenance: prov})
}

func addConsumer(out *[]artifact.Consumer, seen map[artifact.Identity]struct{}, kind artifact.Kind, ref string, prov artifact.Provenance) {
	id := artifact.NewIdentity(kind, ref)
	if id.Ref == "" {
		return
	}
	if _, dup := seen[id]; dup {
		return
	}
	seen[id] = struct{}{}
	*out = append(*out, artifact.Consumer{Identity: id, Provenance: prov})
}
