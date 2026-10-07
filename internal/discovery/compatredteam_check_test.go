package discovery

import (
	"os/exec"
	"path/filepath"
	"testing"
)

// TestCompatRedTeamMatrix runs the independent dual-source check of the
// compatibility matrix and the red-team plan over the real tree from
// inside the Go test surface, so the lockstep between the human tables,
// the machine-readable mirror and the shipped source code is asserted on
// every platform build and not only by the local gate scripts. The
// checker carries its own positive controls (--selftest); this test
// drives the production check against the untouched tree.
func TestCompatRedTeamMatrix(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node runtime not available on this build host")
	}
	cmd := exec.Command(node, filepath.Join("..", "..", "scripts", "compat-redteam-check.mjs"))
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("compatibility matrix / red-team plan check failed:\n%s", out)
	}
	t.Logf("%s", out)
}
