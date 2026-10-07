package discovery

import (
	"os/exec"
	"path/filepath"
	"testing"
)

// TestCoverageTruthfulnessMatrix runs the independent dual-source
// re-count of the coverage truthfulness matrix over the real tree from
// inside the Go test surface, so the lockstep between the human table,
// the machine-readable mirror and the shipped source code is asserted on
// every platform build and not only by the local gate scripts. The
// checker carries its own positive controls (--selftest); this test
// drives the production check against the untouched tree.
func TestCoverageTruthfulnessMatrix(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node runtime not available on this build host")
	}
	cmd := exec.Command(node, filepath.Join("..", "..", "scripts", "coverage-truth-check.mjs"))
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("coverage truthfulness check failed:\n%s", out)
	}
	t.Logf("%s", out)
}
