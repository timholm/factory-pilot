package report

import (
	"strings"
	"testing"
	"time"

	"github.com/timholm/factory-pilot/internal/act"
	"github.com/timholm/factory-pilot/internal/config"
	"github.com/timholm/factory-pilot/internal/diagnose"
	"github.com/timholm/factory-pilot/internal/think"
)

func TestGenerateReport(t *testing.T) {
	cfg := &config.Config{
		PostgresURL: "postgres://localhost:5432/test",
	}
	reporter := NewReporter(cfg)

	status := &diagnose.SystemStatus{
		Timestamp: time.Now(),
		Papers:    diagnose.PaperStats{Total: 100},
	}

	issues := []think.Issue{
		{Title: "test issue", Severity: "critical", FixType: "kubectl"},
	}

	actions := []act.ActionResult{
		{
			Issue:    issues[0],
			Executed: false,
			Output:   "dry run",
			Duration: "1s",
		},
	}

	report := reporter.Generate(status, issues, actions)

	if report.Date.IsZero() {
		t.Error("expected non-zero date")
	}
	if report.Outcome == "" {
		t.Error("expected non-empty outcome")
	}
}

func TestSummarizeOutcome(t *testing.T) {
	tests := []struct {
		name     string
		issues   []think.Issue
		actions  []act.ActionResult
		contains string
	}{
		{
			name:     "no issues",
			issues:   nil,
			actions:  nil,
			contains: "all systems healthy",
		},
		{
			name: "critical issues with dry run",
			issues: []think.Issue{
				{Severity: "critical"},
				{Severity: "high"},
			},
			actions: []act.ActionResult{
				{Executed: false},
				{Executed: false},
			},
			contains: "1 critical",
		},
		{
			name: "executed fixes",
			issues: []think.Issue{
				{Severity: "high"},
			},
			actions: []act.ActionResult{
				{Executed: true},
			},
			contains: "1 fixes applied",
		},
		{
			name: "failed fixes",
			issues: []think.Issue{
				{Severity: "high"},
			},
			actions: []act.ActionResult{
				{Executed: true, Error: "failed"},
			},
			contains: "1 fixes failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outcome := summarizeOutcome(tt.issues, tt.actions)
			if !strings.Contains(outcome, tt.contains) {
				t.Errorf("outcome %q should contain %q", outcome, tt.contains)
			}
		})
	}
}
