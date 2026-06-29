package harness

import (
	"fmt"
	"net"
	"path/filepath"
	"runtime"
	"testing"
)

// TestClusterPortsSurviveSwitchCreation guards against a race where Raft
// partition switches grab a node's reserved client port between allocation and
// bind, causing "address already in use" and wrong-node ping responses.
func TestClusterPortsSurviveSwitchCreation(t *testing.T) {
	root := repoRoot(t)
	prog := filepath.Join(root, "examples", "solutions", "build-your-own-raft", "go", "your_program.sh")

	for i := 0; i < 20; i++ {
		t.Run(fmt.Sprintf("iter-%d", i), func(t *testing.T) {
			cluster, err := NewCluster(prog, 3, func(string, ...any) {})
			if err != nil {
				t.Fatal(err)
			}
			defer cluster.Cleanup()

			ports := make(map[int]bool, 3)
			for id := 1; id <= 3; id++ {
				addr, err := cluster.Addr(id)
				if err != nil {
					t.Fatal(err)
				}
				_, portStr, err := net.SplitHostPort(addr)
				if err != nil {
					t.Fatal(err)
				}
				var port int
				fmt.Sscanf(portStr, "%d", &port)
				if ports[port] {
					t.Fatalf("duplicate port %d", port)
				}
				ports[port] = true
			}

			// Materialize every inter-node switch before any node binds, the
			// worst-case ordering that used to steal reserved ports.
			for from := 1; from <= 3; from++ {
				for to := 1; to <= 3; to++ {
					if from == to {
						continue
					}
					if _, err := cluster.switchFor(from, to); err != nil {
						t.Fatalf("switch %d->%d: %v", from, to, err)
					}
				}
			}

			if err := cluster.StartAll(); err != nil {
				t.Fatalf("StartAll: %v", err)
			}

			for id := 1; id <= 3; id++ {
				c, err := cluster.Dial(id)
				if err != nil {
					t.Fatalf("dial node %d: %v", id, err)
				}
				var res struct {
					NodeID string `json:"node_id"`
				}
				if err := c.Call("ping", nil, &res); err != nil {
					c.Close()
					t.Fatalf("ping node %d: %v", id, err)
				}
				c.Close()
				want := fmt.Sprint(id)
				if res.NodeID != want {
					t.Fatalf("node %d: got node_id %q", id, res.NodeID)
				}
			}
		})
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}
