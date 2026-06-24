package dockerfile

import "testing"

func TestNormalizeImageRef(t *testing.T) {
	tests := map[string]string{
		// Bare official name ⇒ docker.io/library namespace, tag stripped.
		"golang:1.26": "docker.io/library/golang",
		"alpine":      "docker.io/library/alpine",
		// Hub user image (one slash, no host) ⇒ docker.io, no library namespace.
		"library/golang:1.26": "docker.io/library/golang",
		"someuser/img:v1":     "docker.io/someuser/img",
		// Explicit registries are kept; the host is lowercased.
		"gcr.io/distroless/static-debian12":  "gcr.io/distroless/static-debian12",
		"GCR.IO/Distroless/Static":           "gcr.io/Distroless/Static",
		"ghcr.io/agentic-research/rosary:v1": "ghcr.io/agentic-research/rosary",
		// Digest beats tag, both stripped from the identity key.
		"alpine@sha256:deadbeef":    "docker.io/library/alpine",
		"gcr.io/x/y:1.0@sha256:abc": "gcr.io/x/y",
		// Registry port must not be mistaken for a tag.
		"localhost:5000/app":     "localhost:5000/app",
		"localhost:5000/app:dev": "localhost:5000/app",
		// Empty / whitespace.
		"":    "",
		"   ": "",
	}
	for raw, want := range tests {
		if got := NormalizeImageRef(raw); got != want {
			t.Errorf("NormalizeImageRef(%q) = %q, want %q", raw, got, want)
		}
	}
}
