package report

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// DB handles pilot_reports table operations.
type DB struct {
	connStr string
}

// NewDB creates a report database handle.
func NewDB(connStr string) *DB {
	return &DB{connStr: connStr}
}

// EnsureTable creates the pilot_reports table if it doesn't exist.
func (d *DB) EnsureTable(ctx context.Context) error {
	conn, err := pgx.Connect(ctx, d.connStr)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer conn.Close(ctx)

	_, err = conn.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS pilot_reports (
			id          SERIAL PRIMARY KEY,
			date        DATE NOT NULL DEFAULT CURRENT_DATE,
			status_json JSONB NOT NULL,
			issues_json JSONB NOT NULL,
			actions_json JSONB NOT NULL,
			outcome     TEXT NOT NULL DEFAULT '',
			created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			UNIQUE(date)
		)
	`)
	if err != nil {
		return fmt.Errorf("create table: %w", err)
	}

	return nil
}

// Save upserts a daily report.
func (d *DB) Save(ctx context.Context, r *DailyReport) error {
	conn, err := pgx.Connect(ctx, d.connStr)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer conn.Close(ctx)

	statusJSON, _ := json.Marshal(r.Status)
	issuesJSON, _ := json.Marshal(r.Issues)
	actionsJSON, _ := json.Marshal(r.Actions)

	_, err = conn.Exec(ctx, `
		INSERT INTO pilot_reports (date, status_json, issues_json, actions_json, outcome)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (date) DO UPDATE SET
			status_json = EXCLUDED.status_json,
			issues_json = EXCLUDED.issues_json,
			actions_json = EXCLUDED.actions_json,
			outcome = EXCLUDED.outcome,
			created_at = NOW()
	`, r.Date, statusJSON, issuesJSON, actionsJSON, r.Outcome)
	if err != nil {
		return fmt.Errorf("upsert report: %w", err)
	}

	return nil
}

// GetByDate retrieves a report for a specific date.
func (d *DB) GetByDate(ctx context.Context, date string) (*DailyReport, error) {
	conn, err := pgx.Connect(ctx, d.connStr)
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	defer conn.Close(ctx)

	var r DailyReport
	var statusJSON, issuesJSON, actionsJSON []byte

	err = conn.QueryRow(ctx,
		"SELECT date, status_json, issues_json, actions_json, outcome FROM pilot_reports WHERE date = $1",
		date).Scan(&r.Date, &statusJSON, &issuesJSON, &actionsJSON, &r.Outcome)
	if err != nil {
		return nil, fmt.Errorf("get report: %w", err)
	}

	json.Unmarshal(statusJSON, &r.Status)
	json.Unmarshal(issuesJSON, &r.Issues)
	json.Unmarshal(actionsJSON, &r.Actions)

	return &r, nil
}

// ListRecent returns the most recent N reports.
func (d *DB) ListRecent(ctx context.Context, limit int) ([]DailyReport, error) {
	conn, err := pgx.Connect(ctx, d.connStr)
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	defer conn.Close(ctx)

	rows, err := conn.Query(ctx,
		"SELECT date, status_json, issues_json, actions_json, outcome FROM pilot_reports ORDER BY date DESC LIMIT $1",
		limit)
	if err != nil {
		return nil, fmt.Errorf("list reports: %w", err)
	}
	defer rows.Close()

	var reports []DailyReport
	for rows.Next() {
		var r DailyReport
		var statusJSON, issuesJSON, actionsJSON []byte
		if err := rows.Scan(&r.Date, &statusJSON, &issuesJSON, &actionsJSON, &r.Outcome); err != nil {
			continue
		}
		json.Unmarshal(statusJSON, &r.Status)
		json.Unmarshal(issuesJSON, &r.Issues)
		json.Unmarshal(actionsJSON, &r.Actions)
		reports = append(reports, r)
	}

	return reports, nil
}

// GetCurrentIssues returns issues from the most recent report.
func (d *DB) GetCurrentIssues(ctx context.Context) ([]byte, error) {
	conn, err := pgx.Connect(ctx, d.connStr)
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	defer conn.Close(ctx)

	var issuesJSON []byte
	err = conn.QueryRow(ctx,
		"SELECT issues_json FROM pilot_reports ORDER BY date DESC LIMIT 1").Scan(&issuesJSON)
	if err != nil {
		return nil, fmt.Errorf("get current issues: %w", err)
	}

	return issuesJSON, nil
}

// DailyReport is the full report for one pilot cycle.
type DailyReport struct {
	Date    time.Time   `json:"date"`
	Status  interface{} `json:"status"`
	Issues  interface{} `json:"issues"`
	Actions interface{} `json:"actions"`
	Outcome string      `json:"outcome"`
}
