package report

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/timholm/factory-pilot/internal/act"
	"github.com/timholm/factory-pilot/internal/config"
	"github.com/timholm/factory-pilot/internal/diagnose"
	"github.com/timholm/factory-pilot/internal/think"
)

// Reporter generates and persists daily improvement reports.
type Reporter struct {
	cfg *config.Config
	db  *DB
}

// NewReporter creates a reporter.
func NewReporter(cfg *config.Config) *Reporter {
	return &Reporter{
		cfg: cfg,
		db:  NewDB(cfg.PostgresURL),
	}
}

// Generate creates a daily report from the cycle results.
func (r *Reporter) Generate(
	status *diagnose.SystemStatus,
	issues []think.Issue,
	actions []act.ActionResult,
) *DailyReport {
	outcome := summarizeOutcome(issues, actions)

	return &DailyReport{
		Date:    time.Now().UTC(),
		Status:  status,
		Issues:  issues,
		Actions: actions,
		Outcome: outcome,
	}
}

// Save persists the report to Postgres.
func (r *Reporter) Save(ctx context.Context, report *DailyReport) error {
	if err := r.db.EnsureTable(ctx); err != nil {
		return fmt.Errorf("ensure table: %w", err)
	}
	return r.db.Save(ctx, report)
}

// Print outputs the report summary to stdout.
func (r *Reporter) Print(report *DailyReport) {
	fmt.Println("=== Factory Pilot Daily Report ===")
	fmt.Printf("Date: %s\n\n", report.Date.Format("2006-01-02"))

	issues, ok := report.Issues.([]think.Issue)
	if ok {
		fmt.Printf("Issues found: %d\n", len(issues))
		for i, issue := range issues {
			fmt.Printf("  %d. [%s] %s\n", i+1, strings.ToUpper(issue.Severity), issue.Title)
			fmt.Printf("     Root cause: %s\n", issue.RootCause)
			fmt.Printf("     Fix type: %s\n", issue.FixType)
		}
	}

	actions, ok := report.Actions.([]act.ActionResult)
	if ok {
		fmt.Printf("\nActions taken: %d\n", len(actions))
		for i, a := range actions {
			status := "executed"
			if !a.Executed {
				status = "dry-run"
			}
			if a.Error != "" {
				status = "FAILED"
			}
			fmt.Printf("  %d. [%s] %s (%s)\n", i+1, status, a.Issue.Title, a.Duration)
		}
	}

	fmt.Printf("\nOutcome: %s\n", report.Outcome)
}

// GetDB returns the underlying database for API use.
func (r *Reporter) GetDB() *DB {
	return r.db
}

// EnsureTable creates the reports table if needed.
func (r *Reporter) EnsureTable(ctx context.Context) error {
	return r.db.EnsureTable(ctx)
}

// PrintLatest loads and prints the most recent report.
func (r *Reporter) PrintLatest(ctx context.Context) error {
	reports, err := r.db.ListRecent(ctx, 1)
	if err != nil {
		return err
	}
	if len(reports) == 0 {
		log.Println("no reports found")
		return nil
	}
	r.Print(&reports[0])
	return nil
}

func summarizeOutcome(issues []think.Issue, actions []act.ActionResult) string {
	if len(issues) == 0 {
		return "all systems healthy, no issues found"
	}

	executed := 0
	failed := 0
	dryRun := 0
	for _, a := range actions {
		switch {
		case a.Error != "":
			failed++
		case a.Executed:
			executed++
		default:
			dryRun++
		}
	}

	critical := 0
	for _, i := range issues {
		if i.Severity == "critical" {
			critical++
		}
	}

	parts := []string{
		fmt.Sprintf("%d issues found (%d critical)", len(issues), critical),
	}
	if executed > 0 {
		parts = append(parts, fmt.Sprintf("%d fixes applied", executed))
	}
	if dryRun > 0 {
		parts = append(parts, fmt.Sprintf("%d fixes in dry-run", dryRun))
	}
	if failed > 0 {
		parts = append(parts, fmt.Sprintf("%d fixes failed", failed))
	}

	return strings.Join(parts, ", ")
}
