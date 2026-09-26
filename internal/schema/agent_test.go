package schema

import "testing"

func validAgent() *Agent {
	return &Agent{
		ID:       "agent-01",
		Name:     "demo agent",
		Kind:     KindClaudeCode,
		Platform: PlatformLinux,
		Version:  "1.2.3",
		PID:      4242,
		PPID:     1,
	}
}

func TestAgentValid(t *testing.T) {
	if err := validAgent().Validate(); err != nil {
		t.Fatalf("expected valid, got %v", err)
	}
	// optional fields may be absent
	a := validAgent()
	a.PID, a.PPID, a.Version = 0, 0, ""
	if err := a.Validate(); err != nil {
		t.Fatalf("zero optionals must be valid, got %v", err)
	}
}

func TestAgentInvalid(t *testing.T) {
	cases := []struct {
		name string
		mut  func(*Agent)
	}{
		{"empty id", func(a *Agent) { a.ID = "" }},
		{"id with space", func(a *Agent) { a.ID = "bad id" }},
		{"id with slash", func(a *Agent) { a.ID = "../etc/passwd" }},
		{"id leading dash", func(a *Agent) { a.ID = "-lead" }},
		{"id too long", func(a *Agent) { a.ID = string(make([]byte, 65)) }},
		{"empty name", func(a *Agent) { a.Name = "" }},
		{"name too long", func(a *Agent) { a.Name = string(make([]byte, MaxNameLen+1)) }},
		{"unknown kind", func(a *Agent) { a.Kind = AgentKind("ghost") }},
		{"unknown platform", func(a *Agent) { a.Platform = Platform("plan9") }},
		{"negative pid", func(a *Agent) { a.PID = -1 }},
		{"negative ppid", func(a *Agent) { a.PPID = -7 }},
		{"version too long", func(a *Agent) { a.Version = string(make([]byte, MaxNameLen+1)) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := validAgent()
			tc.mut(a)
			if err := a.Validate(); err == nil {
				t.Fatalf("expected error for %s", tc.name)
			}
		})
	}
}

func TestPlatformAndKindEnums(t *testing.T) {
	for _, p := range []Platform{PlatformLinux, PlatformDarwin, PlatformWindows} {
		if !p.Valid() {
			t.Fatalf("platform %s should be valid", p)
		}
	}
	if Platform("windows ").Valid() { // trailing space must not pass
		t.Fatal("padded platform must be invalid")
	}
	for _, k := range []AgentKind{KindClaudeCode, KindCodex, KindOpenClaw, KindMCPServer, KindSystem, KindUnknown} {
		if !k.Valid() {
			t.Fatalf("kind %s should be valid", k)
		}
	}
}
