package discovery

import (
	"os/exec"
	"path/filepath"
	"testing"
)

// TestThreatModelUplift runs the independent bidirectional check of the
// threat model over the real tree from inside the Go test surface, so
// the lockstep between the built-in rule table, the threat rows, the
// pinned five-row family uplift with its clause map, and the three
// supply-chain counter tokens parsed from the shipped agency-guard
// source is asserted on every platform build and not only by the local
// gate scripts. The checker carries its own positive controls
// (--selftest); this test drives the production check against the
// untouched tree.
func TestThreatModelUplift(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node runtime not available on this build host")
	}
	cmd := exec.Command(node, filepath.Join("..", "..", "scripts", "threatmodel-check.mjs"))
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("threat model uplift check failed:\n%s", out)
	}
	t.Logf("%s", out)
}
