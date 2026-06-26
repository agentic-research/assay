// Package artifact defines the vocabulary of assay's artifact/usage graph:
// the producible/consumable units, their stable global identities, and the
// directed producer→consumer edges between them.
//
// This package is pure types and helpers. It does no parsing and no file I/O,
// and it depends on nothing else in assay. Extractors emit Producer/Consumer
// facts in these terms; the resolver matches Consumers to Producers by Identity.
package artifact

// Kind classifies an artifact. It is string-backed (not an iota) so that the
// canonical Identity key is stable and human-readable across runs and
// serialization boundaries. The set is extensible: add a new const and list it
// in validKinds.
type Kind string

const (
	// KindGoModule is a Go module, identified by its module path.
	KindGoModule Kind = "go_module"
	// KindGoPackageSymbol is a Go package-level symbol, identified by its
	// fully-qualified import path plus symbol name.
	KindGoPackageSymbol Kind = "go_package_symbol"
	// KindContainerImage is a container image, identified by its reference
	// (registry/repository[:tag][@digest]).
	KindContainerImage Kind = "container_image"
	// KindCLIBinary is a command-line binary, identified by its invoked name.
	KindCLIBinary Kind = "cli_binary"
	// KindCargoCrate is a Rust crate (a Cargo package), identified by its crate
	// name: the [package].name a manifest declares, or the package-renamed name a
	// dependency resolves to. A crate's version/version requirement is never part
	// of its identity.
	KindCargoCrate Kind = "cargo_crate"
	// KindService is a deployable runtime service, identified by its service
	// name: the worker a Cloudflare wrangler.toml declares (its top-level name),
	// or the service a binding targets. A service has no version concept — the
	// whole name is its identity.
	KindService Kind = "service"
)

// validKinds is the set of recognized kinds. Extend it when adding a Kind.
var validKinds = map[Kind]struct{}{
	KindGoModule:        {},
	KindGoPackageSymbol: {},
	KindContainerImage:  {},
	KindCLIBinary:       {},
	KindCargoCrate:      {},
	KindService:         {},
}

// String returns the kind's stable string form.
func (k Kind) String() string { return string(k) }

// Valid reports whether k is a recognized kind.
func (k Kind) Valid() bool {
	_, ok := validKinds[k]
	return ok
}
