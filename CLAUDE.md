# CLAUDE.md

## Build & Test

```bash
task build    # Build binary → bin/assay
task test     # Run all tests (go test -race -v ./...)
task lint     # golangci-lint run ./...
task fmt      # gofumpt -w -extra .
task check    # fmt + vet + lint + test
```

## Architecture

Assay verifies documentation coverage by treating docs as claims about code.

### Data Flow

```
Source files (.go)  →  tree-sitter extraction  →  []Entity
Markdown files (.md) →  tree-sitter-markdown   →  []DocRef
                                                      ↓
                                              Set operations
                                                      ↓
                                            CoverageResult
```

### Key Packages

| Package | Role |
|---------|------|
| `internal/code/` | Tree-sitter entity extraction (Go, extensible to other langs) |
| `internal/docs/` | Markdown code reference extraction via tree-sitter-markdown |
| `internal/coverage/` | Types, set operations, report formatting |
| `cmd/` | Cobra CLI: `verify`, `version` |

### Tree-sitter Queries

Queries are derived from mache's `examples/go-schema.json` — proven patterns for extracting functions, methods, types, constants, variables.

### Levels (roadmap)

1. **Set coverage** (current) — `|C ∩ D| / |C|`
2. **FCA lattice** — formal concept analysis of coverage structure
3. **Signature drift** — compare extracted sigs vs doc claims
4. **Semantic embeddings** — cosine similarity via ley-line MiniLM
