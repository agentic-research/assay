package artifact

// Provenance records where a producer or consumer fact was observed in source.
type Provenance struct {
	File string // Source file path (relative to a scan root).
	Line int    // 1-based line number; 0 if unknown.
}

// Artifact is an identified producible/consumable unit. Name is an optional,
// human-facing display label; Identity is the load-bearing value.
type Artifact struct {
	Identity Identity
	Name     string // Optional display name (e.g. "Compute", "assay").
}

// Producer is a declaration/build of an artifact found in a source: the thing a
// scan root produces. It carries the artifact Identity and where it was found.
type Producer struct {
	Identity   Identity
	Provenance Provenance
}

// Kind returns the kind of the produced artifact.
func (p Producer) Kind() Kind { return p.Identity.Kind }

// Consumer is a reference to an artifact found in a source: a use of something
// that may or may not be produced within the scanned roots. It carries the
// artifact Identity and where it was found.
type Consumer struct {
	Identity   Identity
	Provenance Provenance
}

// Kind returns the kind of the consumed artifact.
func (c Consumer) Kind() Kind { return c.Identity.Kind }

// Edge is a directed producer→consumer usage relation over a shared Identity,
// created by the resolver when a Consumer's reference matches a Producer's
// Identity. Repo boundaries are invisible: the endpoints may live in different
// scan roots.
type Edge struct {
	Producer Producer
	Consumer Consumer
}
