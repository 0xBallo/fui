package main

import (
	"os/exec"
	"strings"
	"testing"
)

// TestNoNetworkDependency asserts the fui binary never links anything that could open a network
// connection: FUI is local-first by design (see docs/specs/tecniche.md "Sicurezza" and
// .golangci.yml's forbidigo rule against net/http and net/rpc). `go list -deps` walks the whole
// dependency graph, including transitive third-party packages gofumpt/golangci-lint's forbidigo
// (source-only) cannot see.
func TestNoNetworkDependency(t *testing.T) {
	out, err := exec.Command("go", "list", "-deps", ".").Output()
	if err != nil {
		t.Fatalf("go list -deps .: %v\n%s", err, out)
	}
	deps := strings.Split(strings.TrimSpace(string(out)), "\n")
	for _, forbidden := range []string{"net/http", "net/rpc"} {
		for _, dep := range deps {
			if dep == forbidden {
				t.Errorf("fui depends on %q transitively; fui must never open a network connection", forbidden)
			}
		}
	}
}
