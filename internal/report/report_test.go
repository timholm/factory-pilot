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

func TestNewReporter(t *testing.T) {
	cfg := &config.Config{
		PostgresURL: "postgres://localhost:5432/test",
	}
	reporter := NewReporter(cfg)
	if reporter == nil {
		t.Fatal("NewReporter returned nil")
	}
	if reporter.cfg != cfg {
		t.Error("Reporter does not reference the provided config")
	}
	if reporter.db == nil {
		t.Error("Reporter.db is nil")
	}
}

func TestGetDB(t *testing.T) {
	cfg := &config.Config{
		PostgresURL: "postgres://localhost:5432/test",
	}
	reporter := NewReporter(cfg)
	db := reporter.GetDB()
	if db == nil {
		t.Fatal("GetDB returned nil")
	}
}

func TestNewDB(t *testing.T) {
	db := NewDB("postgres://localhost:5432/test")
	if db == nil {
		t.Fatal("NewDB returned nil")
	}
	if db.connStr != "postgres://localhost:5432/test" {
		t.Errorf("connStr = %q, want postgres://localhost:5432/test", db.connStr)
	}
}

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
	if report.Status == nil {
		t.Error("Status should not be nil")
	}
	if report.Issues == nil {
		t.Error("Issues should not be nil")
	}
	if report.Actions == nil {
		t.Error("Actions should not be nil")
	}
}

func TestGenerateReport_NilInputs(t *testing.T) {
	cfg := &config.Config{
		PostgresURL: "postgres://localhost:5432/test",
	}
	reporter := NewReporter(cfg)

	status := &diagnose.SystemStatus{
		Timestamp: time.Now(),
	}

	report := reporter.Generate(status, nil, nil)
	if report == nil {
		t.Fatal("Generate returned nil for nil issues/actions")
	}
	if report.Outcome != "all systems healthy, no issues found" {
		t.Errorf("Outcome = %q, want 'all systems healthy, no issues found'", report.Outcome)
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
			name:     "empty issues",
			issues:   []think.Issue{},
			actions:  []act.ActionResult{},
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
		{
			name: "mixed results",
			issues: []think.Issue{
				{Severity: "critical"},
				{Severity: "high"},
				{Severity: "medium"},
			},
			actions: []act.ActionResult{
				{Executed: true},
				{Executed: true, Error: "failed"},
				{Executed: false},
			},
			contains: "1 critical",
		},
		{
			name: "all critical",
			issues: []think.Issue{
				{Severity: "critical"},
				{Severity: "critical"},
				{Severity: "critical"},
			},
			actions: []act.ActionResult{
				{Executed: true},
				{Executed: true},
				{Executed: true},
			},
			contains: "3 critical",
		},
		{
			name: "no critical only low",
			issues: []think.Issue{
				{Severity: "low"},
				{Severity: "low"},
			},
			actions: []act.ActionResult{
				{Executed: false},
				{Executed: false},
			},
			contains: "0 critical",
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

func TestSummarizeOutcome_CountAccuracy(t *testing.T) {
	issues := []think.Issue{
		{Severity: "critical"},
		{Severity: "high"},
		{Severity: "medium"},
		{Severity: "low"},
	}
	actions := []act.ActionResult{
		{Executed: true},                   // executed
		{Executed: true, Error: "err"},     // failed
		{Executed: false},                  // dry-run
		{Executed: false},                  // dry-run
	}

	outcome := summarizeOutcome(issues, actions)

	// Should have: 4 issues, 1 critical, 1 fix applied, 2 dry-run, 1 failed
	if !strings.Contains(outcome, "4 issues found") {
		t.Errorf("outcome should report 4 issues: %q", outcome)
	}
	if !strings.Contains(outcome, "1 critical") {
		t.Errorf("outcome should report 1 critical: %q", outcome)
	}
	if !strings.Contains(outcome, "1 fixes applied") {
		t.Errorf("outcome should report 1 fix applied: %q", outcome)
	}
	if !strings.Contains(outcome, "2 fixes in dry-run") {
		t.Errorf("outcome should report 2 dry-run: %q", outcome)
	}
	if !strings.Contains(outcome, "1 fixes failed") {
		t.Errorf("outcome should report 1 failed: %q", outcome)
	}
}

func TestDailyReport_Fields(t *testing.T) {
	now := time.Now()
	report := DailyReport{
		Date:    now,
		Status:  "test_status",
		Issues:  "test_issues",
		Actions: "test_actions",
		Outcome: "test outcome",
	}

	if report.Date != now {
		t.Error("Date mismatch")
	}
	if report.Status != "test_status" {
		t.Error("Status mismatch")
	}
	if report.Issues != "test_issues" {
		t.Error("Issues mismatch")
	}
	if report.Actions != "test_actions" {
		t.Error("Actions mismatch")
	}
	if report.Outcome != "test outcome" {
		t.Error("Outcome mismatch")
	}
}
