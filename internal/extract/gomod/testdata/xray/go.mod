module github.com/agentic-research/x-ray

go 1.25.0

require (
	github.com/agentic-research/mache v0.5.5
	github.com/gorilla/websocket v1.5.3
	github.com/tmc/it2 v0.0.0-20251116041255-d10afde85159
)

require (
	github.com/google/go-cmp v0.7.0 // indirect
	golang.org/x/sys v0.42.0 // indirect
)

// Local development only — comment out for CI/public:
// replace github.com/agentic-research/mache => ../mache
