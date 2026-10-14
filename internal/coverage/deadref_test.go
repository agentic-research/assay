package coverage

import (
	"strings"
	"testing"
)

// deadRefEntities is a small codebase: package "coverage" with a function, a
// type and one method on that type.
func deadRefEntities() []Entity {
	return []Entity{
		{Name: "Gate", Kind: "function", Package: "coverage"},
		{Name: "MarkdownSource", Kind: "type", Package: "coverage"},
		{Name: "MarkdownSource.Claims", Kind: "method", Package: "coverage"},
	}
}

func span(text string) DocRef {
	return DocRef{Text: text, Kind: "code_span", File: "docs/ARCH.md", Line: 7}
}

func TestDeadRefs_FlagsLocalReferencesThatResolveToNothing(t *testing.T) {
	for _, text := range []string{
		// Package-qualified, deleted. Fuzzy matching resolves this to Gate,
		// which is exactly why the dead check must be exact.
		"coverage.GateThatWasDeleted",
		"coverage.GateThatWasDeleted()",
		// Package + receiver qualified, method deleted.
		"coverage.MarkdownSource.Walk",
		// Receiver-qualified on a local type, method deleted.
		"MarkdownSource.Walk",
	} {
		t.Run(text, func(t *testing.T) {
			dead := DeadRefs(deadRefEntities(), []DocRef{span(text)})
			if len(dead) != 1 {
				t.Fatalf("%q names a local entity that does not exist; want 1 dead ref, got %v", text, dead)
			}
		})
	}
}

func TestDeadRefs_IgnoresWhatItCannotAttribute(t *testing.T) {
	for _, ref := range []DocRef{
		span("coverage.Gate"),                  // resolves
		span("Gate()"),                         // resolves (bare)
		span("coverage.MarkdownSource.Claims"), // resolves
		span("MarkdownSource.Claims"),          // resolves
		span("RemovedExportedThing"),           // bare: cannot tell local from foreign
		span("graph.GraphCache"),               // graph is not a package here
		span("Config.Load"),                    // Config is not a type here
		span("wrangler.toml"),                  // filename
		span("coverage.unexported"),            // not an exported name
		span("coverage.Gate(result, opts)"),    // call expression, not a name
		span("a.b.c.d"),                        // too deep to be a Go reference
		{Text: "coverage.GateThatWasDeleted", Kind: "heading", File: "x.md", Line: 1},
	} {
		if dead := DeadRefs(deadRefEntities(), []DocRef{ref}); len(dead) != 0 {
			t.Errorf("%q (%s) must not be a dead ref, got %v", ref.Text, ref.Kind, dead)
		}
	}
}

// TestGate_DeadRefs_FiresOnDeletedEntity_NotOnClean is the load-bearing pair
// for the staleness direction: same options, opposite verdicts, and the
// failure names both the missing entity and the doc that names it.
func TestGate_DeadRefs_FiresOnDeletedEntity_NotOnClean(t *testing.T) {
	opts := GateOptions{FailOnDeadRefs: true}

	clean := ComputeWithThreshold(deadRefEntities(),
		[]DocRef{span("coverage.Gate"), span("MarkdownSource.Claims")}, DefaultFuzzyThreshold)
	if err := Gate(clean, opts); err != nil {
		t.Fatalf("docs naming only existing entities must pass, got: %v", err)
	}

	stale := ComputeWithThreshold(deadRefEntities(),
		[]DocRef{span("coverage.Gate"), span("coverage.GateThatWasDeleted")}, DefaultFuzzyThreshold)
	err := Gate(stale, opts)
	if err == nil {
		t.Fatal("a doc naming a deleted entity must fail the gate, got nil")
	}
	for _, want := range []string{"coverage.GateThatWasDeleted", "docs/ARCH.md:7"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("gate error must name %q, got: %v", want, err)
		}
	}

	if err := Gate(stale, GateOptions{}); err != nil {
		t.Errorf("zero-value options must not gate dead refs, got: %v", err)
	}
}
