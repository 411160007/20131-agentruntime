package discovery

import (
	"os/exec"
	"path/filepath"
	"testing"
)

// TestBenchmarkPlanMirror runs the independent dual-source check of the
// performance benchmark plan over the real tree from inside the Go test
// surface, so the lockstep between the human plan, the machine mirror,
// the §70-§73 goal reverse lookup, the four-scenario batch census and
// the shipped-source symbol-absence facts is asserted on every platform
// build and not only by the local gate scripts. The checker carries its
// own positive controls (--selftest); this test drives the production
// check against the untouched tree.
func TestBenchmarkPlanMirror(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node runtime not available on this build host")
	}
	cmd := exec.Command(node, filepath.Join("..", "..", "scripts", "benchplan-check.mjs"))
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("performance benchmark plan check failed:\n%s", out)
	}
	t.Logf("%s", out)
}
