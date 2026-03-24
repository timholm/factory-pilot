package think

import (
	"encoding/json"
	"testing"

	"github.com/timholm/factory-pilot/internal/config"
)

func TestNewThinker(t *testing.T) {
	cfg := &config.Config{
		ClaudeBinary: "/usr/bin/claude",
		MaxFixes:     5,
	}
	thinker := NewThinker(cfg)
	if thinker == nil {
		t.Fatal("NewThinker returned nil")
	}
	if thinker.cfg != cfg {
		t.Error("Thinker does not reference the provided config")
	}
}

func TestParseResponse_ValidJSON(t *testing.T) {
	input := `[
		{
			"issue": "pod crashlooping",
			"severity": "critical",
			"root_cause": "OOM kill",
			"fix_type": "kubectl",
			"fix_commands": ["kubectl rollout restart deployment/worker"],
			"expected_outcome": "pod stops crashing"
		},
		{
			"issue": "high error rate",
			"severity": "high",
			"root_cause": "rate limiting",
			"fix_type": "config",
			"fix_commands": ["kubectl patch configmap/router -p '{\"data\":{\"rate_limit\":\"100\"}}'"],
			"expected_outcome": "error rate drops"
		}
	]`

	issues, err := parseResponse(input)
	if err != nil {
		t.Fatalf("parseResponse returned error: %v", err)
	}
	if len(issues) != 2 {
		t.Fatalf("got %d issues, want 2", len(issues))
	}
	if issues[0].Title != "pod crashlooping" {
		t.Errorf("issues[0].Title = %q, want 'pod crashlooping'", issues[0].Title)
	}
	if issues[0].Severity != "critical" {
		t.Errorf("issues[0].Severity = %q, want 'critical'", issues[0].Severity)
	}
	if issues[1].Title != "high error rate" {
		t.Errorf("issues[1].Title = %q, want 'high error rate'", issues[1].Title)
	}
}

func TestParseResponse_MarkdownWrapped(t *testing.T) {
	input := "```json\n" + `[
		{
			"issue": "test failure",
			"severity": "medium",
			"root_cause": "flaky test",
			"fix_type": "code",
			"fix_commands": ["repo:org/repo fix flaky test"],
			"expected_outcome": "tests pass"
		}
	]` + "\n```"

	issues, err := parseResponse(input)
	if err != nil {
		t.Fatalf("parseResponse with markdown returned error: %v", err)
	}
	if len(issues) != 1 {
		t.Fatalf("got %d issues, want 1", len(issues))
	}
	if issues[0].Title != "test failure" {
		t.Errorf("issues[0].Title = %q, want 'test failure'", issues[0].Title)
	}
}

func TestParseResponse_WithLeadingText(t *testing.T) {
	input := "Here is my analysis:\n\n" + `[{"issue":"disk full","severity":"critical","root_cause":"logs","fix_type":"kubectl","fix_commands":["kubectl delete pod/old"],"expected_outcome":"space freed"}]`

	issues, err := parseResponse(input)
	if err != nil {
		t.Fatalf("parseResponse with leading text returned error: %v", err)
	}
	if len(issues) != 1 {
		t.Fatalf("got %d issues, want 1", len(issues))
	}
	if issues[0].Title != "disk full" {
		t.Errorf("issue title = %q, want 'disk full'", issues[0].Title)
	}
}

func TestParseResponse_EmptyArray(t *testing.T) {
	input := "[]"
	issues, err := parseResponse(input)
	if err != nil {
		t.Fatalf("parseResponse on empty array returned error: %v", err)
	}
	if len(issues) != 0 {
		t.Errorf("got %d issues, want 0", len(issues))
	}
}

func TestParseResponse_InvalidJSON(t *testing.T) {
	input := "this is not json at all"
	_, err := parseResponse(input)
	if err == nil {
		t.Error("parseResponse should return error for invalid JSON")
	}
}

func TestParseResponse_NoArray(t *testing.T) {
	input := `{"issue": "single object, not array"}`
	_, err := parseResponse(input)
	if err == nil {
		t.Error("parseResponse should fail for non-array JSON")
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		input string
		n     int
		want  string
	}{
		{"hello", 10, "hello"},
		{"hello", 5, "hello"},
		{"hello world", 5, "hello..."},
		{"", 5, ""},
		{"ab", 1, "a..."},
	}

	for _, tt := range tests {
		got := truncate(tt.input, tt.n)
		if got != tt.want {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.n, got, tt.want)
		}
	}
}

func TestIssueJSONTags(t *testing.T) {
	issue := Issue{
		Title:           "test",
		Severity:        "low",
		RootCause:       "cause",
		FixType:         "kubectl",
		FixCommands:     []string{"cmd1"},
		ExpectedOutcome: "outcome",
	}

	data, err := json.Marshal(issue)
	if err != nil {
		t.Fatalf("Marshal issue: %v", err)
	}

	var m map[string]interface{}
	json.Unmarshal(data, &m)

	// Verify JSON field names match the tags
	expectedKeys := []string{"issue", "severity", "root_cause", "fix_type", "fix_commands", "expected_outcome"}
	for _, key := range expectedKeys {
		if _, ok := m[key]; !ok {
			t.Errorf("JSON output missing key %q", key)
		}
	}
}
