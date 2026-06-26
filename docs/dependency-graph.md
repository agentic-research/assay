# assay map

- Resolved edges: 0
- External dependencies: 208
- Dangling producers: 1

## Graph

```mermaid
graph LR
  subgraph k180db655["Go modules"]
    n89b362ba6a29["bufio"]
    n3eb6f6cfa493["bytes"]
    n5057772039ce["context"]
    n8841fa351443["crypto/sha1"]
    n24b5be5a8bd8["database/sql"]
    nc126039d8a11["encoding/hex"]
    n2f0ad72d3662["encoding/json"]
    nc3f6f1252c0b["errors"]
    n3e9996689220["fmt"]
    n1f4fe6ae37d6["github.com/agentic-research/assay"]
    nb99ad35c6585["github.com/agentic-research/assay/cmd"]
    nff86a555a54c["github.com/agentic-research/assay/internal/artifact"]
    nf3bf57c6d081["github.com/agentic-research/assay/internal/code"]
    n3016d1e25dbe["github.com/agentic-research/assay/internal/coverage"]
    nb07c8971fcf3["github.com/agentic-research/assay/internal/docs"]
    n2ba2840d08d8["github.com/agentic-research/assay/internal/extract"]
    n9a531fd1473e["github.com/agentic-research/assay/internal/extract/capnp"]
    n2ad892f7ffc5["github.com/agentic-research/assay/internal/extract/cargo"]
    n9c295d6311d9["github.com/agentic-research/assay/internal/extract/ci"]
    n6b6846d45da1["github.com/agentic-research/assay/internal/extract/dockerfile"]
    n7dd5fda058e5["github.com/agentic-research/assay/internal/extract/gocode"]
    nc9270fef7747["github.com/agentic-research/assay/internal/extract/gomod"]
    n4be777845823["github.com/agentic-research/assay/internal/extract/wrangler"]
    n852b1ec07131["github.com/agentic-research/assay/internal/report"]
    n4b579e83c84f["github.com/agentic-research/assay/internal/resolve"]
    n1650d597ebc4["github.com/agext/levenshtein"]
    n5865e24c730f["github.com/containerd/typeurl"]
    n46c521508237["github.com/containerd/typeurl/v2"]
    n909a41dbdd81["github.com/davecgh/go-spew"]
    n0f00bd7dca0b["github.com/docker/go-units"]
    n336f397a35a4["github.com/dustin/go-humanize"]
    ncd0ce1bba974["github.com/google/uuid"]
    n15873aaa693c["github.com/inconshreveable/mousetrap"]
    n414f6eb714f0["github.com/mattn/go-isatty"]
    n6c1d30cd1e67["github.com/moby/buildkit"]
    nad7fb764f896["github.com/moby/buildkit/frontend/dockerfile/instructions"]
    n01434d44f73e["github.com/moby/buildkit/frontend/dockerfile/parser"]
    ndfc6f1690bad["github.com/moby/buildkit/frontend/dockerfile/shell"]
    n7b98157fbb3c["github.com/moby/docker-image-spec"]
    n7875648850e1["github.com/ncruces/go-strftime"]
    n9a75a242afd6["github.com/opencontainers/go-digest"]
    n93c9c47ddc4c["github.com/opencontainers/image-spec"]
    ne7a7ddd25433["github.com/pelletier/go-toml"]
    n5b9043b01ff7["github.com/pelletier/go-toml/v2"]
    n43267d7c8bf2["github.com/pelletier/go-toml/v2/unstable"]
    n528cc8982ad0["github.com/pkg/errors"]
    n94e89419f26a["github.com/planetscale/vtprotobuf"]
    nc5ee35bb2469["github.com/pmezard/go-difflib"]
    nf3264d8df737["github.com/remyoudompheng/bigfft"]
    ncd218f94f142["github.com/smacker/go-tree-sitter"]
    n430c5f50e5f0["github.com/smacker/go-tree-sitter/golang"]
    n52ecc2b62164["github.com/smacker/go-tree-sitter/markdown"]
    n1f4532de63ce["github.com/spf13/cobra"]
    n5d772b126b80["github.com/spf13/pflag"]
    nbf0d6c442c97["github.com/stretchr/testify"]
    n6c56ea394212["github.com/tonistiigi/go-csvvalue"]
    n17c266f9ccd7["golang.org/x/mod"]
    n08e7f686577e["golang.org/x/mod/modfile"]
    ne22e1725e7db["golang.org/x/sys"]
    n714cc57c654b["google.golang.org/protobuf"]
    nc4662d5c6be8["gopkg.in/yaml.v3"]
    n40f6cf6985d8["io"]
    n1f6e86c147e0["io/fs"]
    n2ae63bda236c["modernc.org/libc"]
    n995f76cf2a4f["modernc.org/mathutil"]
    nba7d22ef1678["modernc.org/memory"]
    n29df5f9c5bbb["modernc.org/sqlite"]
    nd70318886a49["os"]
    n6e26e07e76b3["os/exec"]
    n3ff03aa9db84["path/filepath"]
    n2ee5e6940c75["regexp"]
    n641191019d94["sort"]
    n7096a075b3f2["strconv"]
    ne798671c964e["strings"]
    n28372c6bdd43["unicode"]
  end
  subgraph k01708225["CLI binaries"]
    nc086671ddc65["git"]
    nedac642d5687["go"]
    nf274dd52c2a7["task"]
  end
```

## External dependencies

- `git` (cli_binary) — .github/workflows/assay-ecosystem.yml:49
- `go` (cli_binary) — .github/workflows/assay-ecosystem.yml:38
- `task` (cli_binary) — .github/workflows/assay-ecosystem.yml:57
- `task` (cli_binary) — .github/workflows/assay-map.yml:39
- `bufio` (go_module) — internal/embeddings/leyline.go:4
- `bufio` (go_module) — internal/extract/capnp/parse.go:4
- `bufio` (go_module) — internal/extract/gocode/treesitter.go:4
- `bytes` (go_module) — internal/embeddings/leyline.go:5
- `bytes` (go_module) — internal/extract/capnp/parse.go:5
- `bytes` (go_module) — internal/extract/dockerfile/dockerfile.go:23
- `context` (go_module) — internal/code/extract.go:4
- `context` (go_module) — internal/docs/extract.go:4
- `context` (go_module) — internal/extract/gocode/treesitter.go:5
- `crypto/sha1` (go_module) — internal/report/mermaid.go:4
- `crypto/sha1` (go_module) — internal/report/mermaid_repo.go:4
- `database/sql` (go_module) — internal/extract/gocode/mache.go:4
- `encoding/hex` (go_module) — internal/report/mermaid.go:5
- `encoding/hex` (go_module) — internal/report/mermaid_repo.go:5
- `encoding/json` (go_module) — internal/coverage/report.go:4
- `encoding/json` (go_module) — internal/report/json.go:4
- `errors` (go_module) — internal/extract/capnp/capnp.go:31
- `errors` (go_module) — internal/extract/cargo/cargo.go:22
- `errors` (go_module) — internal/extract/gocode/mache.go:5
- `errors` (go_module) — internal/extract/gomod/gomod.go:19
- `errors` (go_module) — internal/extract/wrangler/wrangler.go:20
- `fmt` (go_module) — cmd/map.go:4
- `fmt` (go_module) — cmd/root.go:4
- `fmt` (go_module) — cmd/verify.go:4
- `fmt` (go_module) — cmd/version.go:4
- `fmt` (go_module) — internal/coverage/report.go:5
- `fmt` (go_module) — internal/embeddings/leyline.go:6
- `fmt` (go_module) — internal/extract/cargo/manifest.go:4
- `fmt` (go_module) — internal/extract/ci/ci.go:26
- `fmt` (go_module) — internal/extract/dockerfile/dockerfile.go:24
- `fmt` (go_module) — internal/extract/gocode/mache.go:6
- `fmt` (go_module) — internal/extract/wrangler/manifest.go:4
- `fmt` (go_module) — internal/report/mermaid.go:6
- `fmt` (go_module) — internal/report/mermaid_repo.go:6
- `github.com/agentic-research/assay/cmd` (go_module) — main.go:3
- `github.com/agentic-research/assay/internal/artifact` (go_module) — internal/extract/capnp/capnp.go:37
- `github.com/agentic-research/assay/internal/artifact` (go_module) — internal/extract/capnp/emit.go:3
- `github.com/agentic-research/assay/internal/artifact` (go_module) — internal/extract/cargo/cargo.go:27
- `github.com/agentic-research/assay/internal/artifact` (go_module) — internal/extract/cargo/manifest.go:9
- `github.com/agentic-research/assay/internal/artifact` (go_module) — internal/extract/ci/ci.go:32
- `github.com/agentic-research/assay/internal/artifact` (go_module) — internal/extract/ci/walk.go:6
- `github.com/agentic-research/assay/internal/artifact` (go_module) — internal/extract/dockerfile/dockerfile.go:34
- `github.com/agentic-research/assay/internal/artifact` (go_module) — internal/extract/extractor.go:14
- `github.com/agentic-research/assay/internal/artifact` (go_module) — internal/extract/gocode/gocode.go:22
- `github.com/agentic-research/assay/internal/artifact` (go_module) — internal/extract/gocode/mache.go:11
- `github.com/agentic-research/assay/internal/artifact` (go_module) — internal/extract/gocode/treesitter.go:15
- `github.com/agentic-research/assay/internal/artifact` (go_module) — internal/extract/gomod/gomod.go:27
- `github.com/agentic-research/assay/internal/artifact` (go_module) — internal/extract/registry.go:3
- `github.com/agentic-research/assay/internal/artifact` (go_module) — internal/extract/wrangler/manifest.go:9
- `github.com/agentic-research/assay/internal/artifact` (go_module) — internal/extract/wrangler/wrangler.go:25
- `github.com/agentic-research/assay/internal/artifact` (go_module) — internal/report/json.go:8
- `github.com/agentic-research/assay/internal/artifact` (go_module) — internal/report/mermaid.go:11
- `github.com/agentic-research/assay/internal/artifact` (go_module) — internal/report/report.go:19
- `github.com/agentic-research/assay/internal/artifact` (go_module) — internal/resolve/buckets.go:4
- `github.com/agentic-research/assay/internal/artifact` (go_module) — internal/resolve/resolve.go:17
- `github.com/agentic-research/assay/internal/code` (go_module) — cmd/verify.go:10
- `github.com/agentic-research/assay/internal/code` (go_module) — internal/extract/gocode/treesitter.go:16
- `github.com/agentic-research/assay/internal/coverage` (go_module) — cmd/verify.go:11
- `github.com/agentic-research/assay/internal/coverage` (go_module) — internal/code/extract.go:14
- `github.com/agentic-research/assay/internal/coverage` (go_module) — internal/docs/extract.go:13
- `github.com/agentic-research/assay/internal/docs` (go_module) — cmd/verify.go:12
- `github.com/agentic-research/assay/internal/extract` (go_module) — cmd/map.go:10
- `github.com/agentic-research/assay/internal/extract` (go_module) — internal/extract/capnp/capnp.go:38
- `github.com/agentic-research/assay/internal/extract` (go_module) — internal/extract/cargo/cargo.go:28
- `github.com/agentic-research/assay/internal/extract` (go_module) — internal/extract/dockerfile/dockerfile.go:35
- `github.com/agentic-research/assay/internal/extract` (go_module) — internal/extract/gocode/gocode.go:23
- `github.com/agentic-research/assay/internal/extract` (go_module) — internal/extract/gomod/gomod.go:28
- `github.com/agentic-research/assay/internal/extract` (go_module) — internal/extract/wrangler/wrangler.go:26
- `github.com/agentic-research/assay/internal/extract` (go_module) — internal/report/report.go:20
- `github.com/agentic-research/assay/internal/extract` (go_module) — internal/resolve/resolve.go:18
- `github.com/agentic-research/assay/internal/extract/capnp` (go_module) — cmd/map.go:11
- `github.com/agentic-research/assay/internal/extract/cargo` (go_module) — cmd/map.go:12
- `github.com/agentic-research/assay/internal/extract/ci` (go_module) — cmd/map.go:13
- `github.com/agentic-research/assay/internal/extract/dockerfile` (go_module) — cmd/map.go:14
- `github.com/agentic-research/assay/internal/extract/gocode` (go_module) — cmd/map.go:15
- `github.com/agentic-research/assay/internal/extract/gomod` (go_module) — cmd/map.go:16
- `github.com/agentic-research/assay/internal/extract/wrangler` (go_module) — cmd/map.go:17
- `github.com/agentic-research/assay/internal/report` (go_module) — cmd/map.go:18
- `github.com/agentic-research/assay/internal/resolve` (go_module) — cmd/map.go:19
- `github.com/agentic-research/assay/internal/resolve` (go_module) — internal/report/mermaid.go:12
- `github.com/agentic-research/assay/internal/resolve` (go_module) — internal/report/report.go:21
- `github.com/agext/levenshtein` (go_module) — go.mod:17
- `github.com/containerd/typeurl` (go_module) — go.mod:18
- `github.com/containerd/typeurl/v2` (go_module) — go.mod:18
- `github.com/davecgh/go-spew` (go_module) — go.mod:19
- `github.com/docker/go-units` (go_module) — go.mod:20
- `github.com/dustin/go-humanize` (go_module) — go.mod:21
- `github.com/google/uuid` (go_module) — go.mod:22
- `github.com/inconshreveable/mousetrap` (go_module) — go.mod:23
- `github.com/mattn/go-isatty` (go_module) — go.mod:24
- `github.com/moby/buildkit` (go_module) — go.mod:6
- `github.com/moby/buildkit/frontend/dockerfile/instructions` (go_module) — internal/extract/dockerfile/dockerfile.go:30
- `github.com/moby/buildkit/frontend/dockerfile/parser` (go_module) — internal/extract/dockerfile/dockerfile.go:31
- `github.com/moby/buildkit/frontend/dockerfile/shell` (go_module) — internal/extract/dockerfile/dockerfile.go:32
- `github.com/moby/docker-image-spec` (go_module) — go.mod:25
- `github.com/ncruces/go-strftime` (go_module) — go.mod:26
- `github.com/opencontainers/go-digest` (go_module) — go.mod:27
- `github.com/opencontainers/image-spec` (go_module) — go.mod:28
- `github.com/pelletier/go-toml` (go_module) — go.mod:7
- `github.com/pelletier/go-toml/v2` (go_module) — go.mod:7
- `github.com/pelletier/go-toml/v2/unstable` (go_module) — internal/extract/cargo/manifest.go:7
- `github.com/pelletier/go-toml/v2/unstable` (go_module) — internal/extract/wrangler/manifest.go:7
- `github.com/pkg/errors` (go_module) — go.mod:29
- `github.com/planetscale/vtprotobuf` (go_module) — go.mod:30
- `github.com/pmezard/go-difflib` (go_module) — go.mod:31
- `github.com/remyoudompheng/bigfft` (go_module) — go.mod:32
- `github.com/smacker/go-tree-sitter` (go_module) — go.mod:8
- `github.com/smacker/go-tree-sitter` (go_module) — internal/code/extract.go:11
- `github.com/smacker/go-tree-sitter` (go_module) — internal/docs/extract.go:10
- `github.com/smacker/go-tree-sitter` (go_module) — internal/extract/gocode/treesitter.go:12
- `github.com/smacker/go-tree-sitter/golang` (go_module) — internal/code/extract.go:12
- `github.com/smacker/go-tree-sitter/golang` (go_module) — internal/extract/gocode/treesitter.go:13
- `github.com/smacker/go-tree-sitter/markdown` (go_module) — internal/docs/extract.go:11
- `github.com/spf13/cobra` (go_module) — cmd/map.go:8
- `github.com/spf13/cobra` (go_module) — cmd/root.go:7
- `github.com/spf13/cobra` (go_module) — cmd/verify.go:8
- `github.com/spf13/cobra` (go_module) — cmd/version.go:6
- `github.com/spf13/cobra` (go_module) — go.mod:9
- `github.com/spf13/pflag` (go_module) — go.mod:33
- `github.com/stretchr/testify` (go_module) — go.mod:10
- `github.com/tonistiigi/go-csvvalue` (go_module) — go.mod:34
- `golang.org/x/mod` (go_module) — go.mod:11
- `golang.org/x/mod/modfile` (go_module) — internal/extract/gomod/gomod.go:25
- `golang.org/x/sys` (go_module) — go.mod:35
- `google.golang.org/protobuf` (go_module) — go.mod:36
- `gopkg.in/yaml.v3` (go_module) — go.mod:12
- `gopkg.in/yaml.v3` (go_module) — internal/extract/ci/ci.go:33
- `gopkg.in/yaml.v3` (go_module) — internal/extract/ci/parse.go:7
- `gopkg.in/yaml.v3` (go_module) — internal/extract/ci/walk.go:7
- `io` (go_module) — cmd/map.go:5
- `io` (go_module) — internal/coverage/report.go:6
- `io` (go_module) — internal/report/json.go:5
- `io` (go_module) — internal/report/mermaid.go:7
- `io` (go_module) — internal/report/mermaid_repo.go:7
- `io/fs` (go_module) — internal/code/extract.go:5
- `io/fs` (go_module) — internal/docs/extract.go:5
- `io/fs` (go_module) — internal/extract/capnp/capnp.go:32
- `io/fs` (go_module) — internal/extract/cargo/cargo.go:23
- `io/fs` (go_module) — internal/extract/dockerfile/dockerfile.go:25
- `io/fs` (go_module) — internal/extract/gocode/treesitter.go:6
- `io/fs` (go_module) — internal/extract/gomod/gomod.go:20
- `io/fs` (go_module) — internal/extract/wrangler/wrangler.go:21
- `modernc.org/libc` (go_module) — go.mod:37
- `modernc.org/mathutil` (go_module) — go.mod:38
- `modernc.org/memory` (go_module) — go.mod:39
- `modernc.org/sqlite` (go_module) — go.mod:13
- `modernc.org/sqlite` (go_module) — internal/extract/gocode/mache.go:9
- `os` (go_module) — cmd/map.go:6
- `os` (go_module) — cmd/root.go:5
- `os` (go_module) — cmd/verify.go:5
- `os` (go_module) — internal/code/extract.go:6
- `os` (go_module) — internal/docs/extract.go:6
- `os` (go_module) — internal/extract/capnp/capnp.go:33
- `os` (go_module) — internal/extract/cargo/cargo.go:24
- `os` (go_module) — internal/extract/ci/ci.go:27
- `os` (go_module) — internal/extract/dockerfile/dockerfile.go:26
- `os` (go_module) — internal/extract/gocode/mache.go:7
- `os` (go_module) — internal/extract/gocode/treesitter.go:7
- `os` (go_module) — internal/extract/gomod/gomod.go:21
- `os` (go_module) — internal/extract/wrangler/wrangler.go:22
- `os/exec` (go_module) — internal/embeddings/leyline.go:7
- `path/filepath` (go_module) — cmd/verify.go:6
- `path/filepath` (go_module) — internal/code/extract.go:7
- `path/filepath` (go_module) — internal/docs/extract.go:7
- `path/filepath` (go_module) — internal/extract/capnp/capnp.go:34
- `path/filepath` (go_module) — internal/extract/cargo/cargo.go:25
- `path/filepath` (go_module) — internal/extract/ci/ci.go:28
- `path/filepath` (go_module) — internal/extract/dockerfile/dockerfile.go:27
- `path/filepath` (go_module) — internal/extract/gocode/treesitter.go:8
- `path/filepath` (go_module) — internal/extract/gomod/gomod.go:22
- `path/filepath` (go_module) — internal/extract/wrangler/wrangler.go:23
- `path/filepath` (go_module) — internal/report/mermaid_repo.go:8
- `regexp` (go_module) — internal/extract/ci/parse.go:4
- `sort` (go_module) — internal/coverage/report.go:7
- `sort` (go_module) — internal/extract/ci/ci.go:29
- `sort` (go_module) — internal/extract/ci/walk.go:4
- `sort` (go_module) — internal/extract/gocode/treesitter.go:9
- `sort` (go_module) — internal/report/json.go:6
- `sort` (go_module) — internal/report/mermaid.go:8
- `sort` (go_module) — internal/report/mermaid_repo.go:9
- `sort` (go_module) — internal/report/report.go:17
- `sort` (go_module) — internal/resolve/resolve.go:15
- `strconv` (go_module) — internal/embeddings/leyline.go:8
- `strings` (go_module) — internal/artifact/identity.go:3
- `strings` (go_module) — internal/code/extract.go:8
- `strings` (go_module) — internal/coverage/compute.go:3
- `strings` (go_module) — internal/coverage/report.go:8
- `strings` (go_module) — internal/coverage/tokenize.go:4
- `strings` (go_module) — internal/docs/extract.go:8
- `strings` (go_module) — internal/embeddings/leyline.go:9
- `strings` (go_module) — internal/extract/capnp/capnp.go:35
- `strings` (go_module) — internal/extract/capnp/parse.go:6
- `strings` (go_module) — internal/extract/cargo/manifest.go:5
- `strings` (go_module) — internal/extract/ci/ci.go:30
- `strings` (go_module) — internal/extract/ci/parse.go:5
- `strings` (go_module) — internal/extract/dockerfile/dockerfile.go:28
- `strings` (go_module) — internal/extract/dockerfile/normalize.go:3
- `strings` (go_module) — internal/extract/gocode/treesitter.go:10
- `strings` (go_module) — internal/extract/gomod/gomod.go:23
- `strings` (go_module) — internal/extract/wrangler/manifest.go:5
- `strings` (go_module) — internal/report/mermaid.go:9
- `strings` (go_module) — internal/report/mermaid_repo.go:10
- `unicode` (go_module) — internal/code/extract.go:9
- `unicode` (go_module) — internal/coverage/tokenize.go:5

## Dangling producers

- `github.com/agentic-research/assay` (go_module) — go.mod:1
