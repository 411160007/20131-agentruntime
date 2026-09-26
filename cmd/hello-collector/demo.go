// Demo pipeline generation for hello-collector: exercise the full core
// model (Agent registration -> Event proposal -> Policy evaluation ->
// enforcement record -> action) end to end and persist it as JSONL.
//
// This is a construction smoke test of the event model, not the real
// process-discovery collector.
package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"20131.com/agentruntime/internal/policy"
	"20131.com/agentruntime/internal/schema"
)

func newEventID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand failing would break ID uniqueness; degrade to a
		// timestamp-based ID rather than aborting the demo.
		return fmt.Sprintf("id-%d", time.Now().UnixNano())
	}
	return "ev-" + hex.EncodeToString(b)
}

// DemoPolicy is the built-in sample policy used by hello-collector.
func DemoPolicy() *schema.Policy {
	return &schema.Policy{
		ID:            "policy-demo",
		Name:          "hello-collector demo policy",
		Version:       1,
		DefaultEffect: schema.EffectAllow,
		Rules: []schema.Rule{
			{ID: "block-etc-write", Priority: 100, Field: schema.FieldPath, Op: schema.OpPrefix, Value: "/etc/", Effect: schema.EffectWouldBlock, Severity: schema.SevHigh},
			{ID: "allow-echo", Priority: 10, Field: schema.FieldTool, Op: schema.OpEquals, Value: "echo", Effect: schema.EffectAllow, Severity: schema.SevInfo},
		},
	}
}

// DemoAgent registers the collector process itself as the observed agent.
func DemoAgent(name string) *schema.Agent {
	return &schema.Agent{
		ID:       "agent-hello-collector",
		Name:     name,
		Kind:     schema.KindSystem,
		Platform: currentPlatform(),
		Version:  version,
	}
}

// PipelineEvents produces one complete Agent Proposed -> Policy Evaluated ->
// Native Enforcement -> Action chain for agent, evaluated by ev.
func PipelineEvents(agent *schema.Agent, ev *policy.Evaluator) ([]*schema.Event, error) {
	if err := agent.Validate(); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	base := func(offsetMs int, stage schema.Stage, typ schema.EventType) *schema.Event {
		return &schema.Event{
			V:        schema.SchemaVersion,
			TS:       now.Add(time.Duration(offsetMs) * time.Millisecond),
			ID:       newEventID(),
			AgentID:  agent.ID,
			Stage:    stage,
			Type:     typ,
			Decision: schema.DecisionAllow,
			Severity: schema.SevInfo,
		}
	}

	// 1. collector lifecycle
	start := base(0, schema.StageObservation, schema.TypeCollectorStart)
	start.Summary = "hello-collector started (local JSONL audit only, no network)"

	// 2. Agent Proposed
	prop := base(10, schema.StageProposed, schema.TypeCommandProposed)
	prop.Summary = "demo agent proposed running a harmless local echo command"
	prop.Attrs = map[string]string{"tool": "echo", "path": "./hello.txt"}

	// 3. Policy Evaluated
	res, err := ev.Evaluate(prop)
	if err != nil {
		return nil, fmt.Errorf("pipeline: evaluate: %w", err)
	}
	dec, err := schema.DecisionFor(res.Effect)
	if err != nil {
		return nil, fmt.Errorf("pipeline: %w", err)
	}
	eval := base(20, schema.StageEvaluated, schema.TypePolicyDecision)
	eval.Summary = fmt.Sprintf("policy evaluated demo proposal: %s (%s)", res.Effect, res.Reason)
	eval.Decision = dec
	eval.Severity = res.Severity
	eval.Attrs = map[string]string{"policy_id": "policy-demo", "matched_rule": res.MatchedRule}

	// 4. Native Enforcement record (Phase 0: recorded only, nothing blocked)
	enf := base(30, schema.StageEnforcement, schema.TypeEnforceAction)
	enf.Summary = "enforcement stage pass-through: observation mode, decision recorded, nothing blocked"
	enf.Decision = dec
	enf.Severity = res.Severity
	enf.Attrs = map[string]string{"mode": "observe"}

	// 5. Action
	act := base(40, schema.StageAction, schema.TypeToolCall)
	act.Summary = "demo action completed: echo marker recorded locally (simulated, no subprocess run)"

	// 6. collector lifecycle
	stop := base(50, schema.StageObservation, schema.TypeCollectorStop)
	stop.Summary = "hello-collector finished writing demo pipeline events"

	events := []*schema.Event{start, prop, eval, enf, act, stop}
	for _, e := range events {
		if err := e.Validate(); err != nil {
			return nil, err
		}
	}
	return events, nil
}
