package think

import "testing"

func TestSeverityOrder(t *testing.T) {
	tests := []struct {
		severity string
		want     int
	}{
		{"critical", 0},
		{"high", 1},
		{"medium", 2},
		{"low", 3},
		{"unknown", 4},
		{"", 4},
		{"CRITICAL", 4}, // case-sensitive
	}

	for _, tt := range tests {
		t.Run(tt.severity, func(t *testing.T) {
			got := SeverityOrder(tt.severity)
			if got != tt.want {
				t.Errorf("SeverityOrder(%q) = %d, want %d", tt.severity, got, tt.want)
			}
		})
	}
}

func TestSeverityOrder_Ordering(t *testing.T) {
	// Verify that ordering is correct: critical < high < medium < low
	if SeverityOrder("critical") >= SeverityOrder("high") {
		t.Error("critical should sort before high")
	}
	if SeverityOrder("high") >= SeverityOrder("medium") {
		t.Error("high should sort before medium")
	}
	if SeverityOrder("medium") >= SeverityOrder("low") {
		t.Error("medium should sort before low")
	}
}

func TestIssueStruct(t *testing.T) {
	issue := Issue{
		Title:           "pod crashlooping",
		Severity:        "critical",
		RootCause:       "OOM kill",
		FixType:         "kubectl",
		FixCommands:     []string{"kubectl rollout restart deployment/worker"},
		ExpectedOutcome: "pod stops crashing",
	}

	if issue.Title != "pod crashlooping" {
		t.Error("Title mismatch")
	}
	if issue.Severity != "critical" {
		t.Error("Severity mismatch")
	}
	if issue.RootCause != "OOM kill" {
		t.Error("RootCause mismatch")
	}
	if issue.FixType != "kubectl" {
		t.Error("FixType mismatch")
	}
	if len(issue.FixCommands) != 1 {
		t.Errorf("FixCommands length = %d, want 1", len(issue.FixCommands))
	}
	if issue.ExpectedOutcome != "pod stops crashing" {
		t.Error("ExpectedOutcome mismatch")
	}
}
