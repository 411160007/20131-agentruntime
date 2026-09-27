// D6 adapter commands: hook (receiver), integrate (config generator),
// mcp (stdio pass-through proxy). All three are observation surfaces:
// none of them can change what the integrated agent or MCP server does.
package main

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"20131.com/agentruntime/internal/adapter"
	"20131.com/agentruntime/internal/auditlog"
	"20131.com/agentruntime/internal/bus"
	"20131.com/agentruntime/internal/identity"
	"20131.com/agentruntime/internal/mcpproxy"
	"20131.com/agentruntime/internal/rules"
	"20131.com/agentruntime/internal/schema"
)

// newAdapterID mints event ids in the receiver/proxy processes.
func newAdapterID(prefix string) string {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
	}
	return prefix + "-" + hex.EncodeToString(b)
}

// notef reports an adapter note on stderr. The receiver's contract is
// that it NEVER writes to stdout and NEVER exits non-zero: a broken
// audit surface must not change what the integrated agent does.
func notef(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "adapter: "+format+"\n", args...)
}

// decisionRelay runs the judgement consumer loop on its own goroutine:
// events drained from ech are decided and their policy.decision lines
// written to lg. close(ech) ends the loop; done reports the
// deterministic drain point (every delivered event audited before the
// writer closes).
func decisionRelay(eng *rules.Engine, ech chan *schema.Event, lg *auditlog.Writer, done chan struct{}) {
	go func() {
		defer close(done)
		for e := range ech {
			d, derr := eng.Decide(e)
			if derr != nil {
				continue // undecidable input is dropped, never forged
			}
			line, lerr := rules.DecisionEvent(e, d)
			if lerr != nil {
				continue
			}
			if werr := lg.WriteEvent(line); werr != nil {
				notef("decision line write failed: %v", werr)
			}
		}
	}()
}

// runHookSub implements `agent-collector hook [--out file]`: read one
// hook payload from stdin, record the audit line(s), exit 0 with empty
// stdout no matter what. The pass-through property is structural: this
// code path contains no writes to os.Stdout at all.
func runHookSub(c config) error {
	raw, err := io.ReadAll(io.LimitReader(os.Stdin, adapter.MaxHookBytes+1))
	if err != nil {
		notef("stdin read failed: %v", err)
		return nil
	}
	if len(raw) == 0 {
		notef("empty payload, recording nothing")
		return nil
	}
	if len(raw) > adapter.MaxHookBytes {
		notef("payload too large, recording nothing")
		return nil
	}
	h, err := adapter.ParseHookInput(raw)
	if err != nil {
		if errors.Is(err, adapter.ErrUnknownHook) {
			notef("unsupported hook event, recording nothing")
		} else {
			notef("payload not parseable, recording nothing")
		}
		return nil
	}
	machine, _ := identity.MachineFingerprint()
	ev, err := adapter.HookEvent(h, machine, time.Now(), newAdapterID("hk"))
	if err != nil {
		notef("event build failed, recording nothing")
		return nil
	}
	lg, err := auditlog.Open(c.out)
	if err != nil {
		notef("audit open failed: %v", err)
		return nil
	}
	defer func() {
		if cerr := lg.Close(); cerr != nil {
			notef("audit close: %v", cerr)
		}
	}()
	b := bus.New(lg)
	src, err := b.Register("hook")
	if err != nil {
		notef("bus register: %v", err)
		return nil
	}
	ech := make(chan *schema.Event, 16)
	stop := b.AttachEngine(ech)
	done := make(chan struct{})
	decisionRelay(rules.MustDefault(), ech, lg, done)
	if err := src.Publish(ev); err != nil {
		notef("publish rejected: %v", err)
	}
	close(ech) // deterministic drain: relay finishes queued events, then signals
	<-done
	stop()
	return nil
}

// runIntegrateSub implements `agent-collector integrate --target
// <claude-code|codex|openclaw> --dir <config dir> [--bin <collector>]`:
// idempotent, fail-closed configuration generation. Writes ONLY inside
// --dir. codex writes nothing at all (documented gap).
func runIntegrateSub(c config) error {
	if c.target == "" || c.dir == "" {
		return fmt.Errorf("integrate: --target and --dir are required")
	}
	if fi, err := os.Stat(c.dir); err != nil || !fi.IsDir() {
		return fmt.Errorf("integrate: --dir must be an existing directory")
	}
	bin := c.bin
	if bin == "" {
		exe, err := os.Executable()
		if err != nil {
			return fmt.Errorf("integrate: cannot resolve collector path: %w", err)
		}
		if abs, err := filepath.Abs(exe); err == nil {
			exe = abs
		}
		bin = exe
	}
	switch c.target {
	case "codex":
		fmt.Println(adapter.CodexGap)
		return nil
	case "claude-code":
		return mergeIntoFile(filepath.Join(c.dir, "settings.json"), func(existing []byte) ([]byte, bool, error) {
			return adapter.MergeClaudeSettings(existing, bin)
		})
	case "openclaw":
		return mergeIntoFile(filepath.Join(c.dir, "openclaw.json"), func(existing []byte) ([]byte, bool, error) {
			return adapter.MergeOpenClaw(existing, bin)
		})
	default:
		return fmt.Errorf("integrate: unknown target %q (want claude-code|codex|openclaw)", c.target)
	}
}

func mergeIntoFile(path string, merge func([]byte) ([]byte, bool, error)) error {
	var existing []byte
	if b, err := os.ReadFile(path); err == nil {
		existing = b
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("integrate: read %s: %w", path, err)
	}
	out, changed, err := merge(existing)
	if err != nil {
		return err
	}
	if !changed {
		fmt.Printf("integrate: %s already wired, no changes\n", path)
		return nil
	}
	if err := os.WriteFile(path, out, 0o600); err != nil {
		return fmt.Errorf("integrate: write %s: %w", path, err)
	}
	fmt.Printf("integrate: updated %s\n", path)
	return nil
}

// runMCPSub implements `agent-collector mcp --server <exe> [server args]`:
// a stdio JSON-RPC relay that forwards every byte both ways untouched
// while recording tools/call round trips through the bus + judgement
// consumer. Annotations only: the relay path never consults the
// judgement, and the judgement never reaches the transport.
func runMCPSub(c config, serverArgs []string) error {
	if c.server == "" {
		return fmt.Errorf("mcp: --server is required")
	}
	lg, err := auditlog.Open(c.out)
	if err != nil {
		return err
	}
	defer lg.Close()
	b := bus.New(lg)
	src, err := b.Register("mcp")
	if err != nil {
		return err
	}
	ech := make(chan *schema.Event, 16)
	stop := b.AttachEngine(ech)
	done := make(chan struct{})
	decisionRelay(rules.MustDefault(), ech, lg, done)

	cmd := exec.Command(c.server, serverArgs...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("mcp: start server: %w", err)
	}

	machine, _ := identity.MachineFingerprint()
	label := filepath.Base(c.server)
	seq := 0
	observe := func(rec mcpproxy.Record) {
		seq++
		ev, err := adapter.MCPEvent(label, rec.Tool, rec.ArgsRaw, rec.Latency, rec.ReqBytes, rec.RespBytes,
			machine, time.Now(), fmt.Sprintf("mc-%05d", seq))
		if err != nil {
			return
		}
		_ = src.Publish(ev)
	}
	proxy := mcpproxy.New(observe)
	runErr := proxy.Run(os.Stdin, os.Stdout, stdin, stdout)
	close(ech) // no publishes after Run returns; drain decisions deterministically
	<-done
	stop()
	// Client stream ended: close server input, give the child a grace
	// window, then reap. None of this consults observations.
	_ = stdin.Close()
	waitDone := make(chan error, 1)
	go func() { waitDone <- cmd.Wait() }()
	select {
	case <-waitDone:
	case <-time.After(2 * time.Second):
		_ = cmd.Process.Kill()
		<-waitDone
	}
	if runErr != nil {
		return fmt.Errorf("mcp: relay: %w", runErr)
	}
	if n := proxy.Dropped(); n > 0 {
		notef("%d observation(s) dropped under load (transport unaffected)", n)
	}
	return nil
}
