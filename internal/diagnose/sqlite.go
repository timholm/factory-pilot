package diagnose

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

// SQLiteCollector queries the factory build registry (SQLite).
type SQLiteCollector struct {
	dataDir string
}

// NewSQLiteCollector creates a SQLite collector.
func NewSQLiteCollector(dataDir string) *SQLiteCollector {
	return &SQLiteCollector{dataDir: dataDir}
}

func (s *SQLiteCollector) dbPath() string {
	return filepath.Join(s.dataDir, "factory.db")
}

// CollectBuilds returns build pipeline statistics from the factory registry.
func (s *SQLiteCollector) CollectBuilds(ctx context.Context) (BuildStats, error) {
	db, err := sql.Open("sqlite3", s.dbPath()+"?mode=ro")
	if err != nil {
		return BuildStats{}, fmt.Errorf("open sqlite: %w", err)
	}
	defer db.Close()

	stats := BuildStats{
		ErrorPatterns: make(map[string]int),
	}

	// Count by status
	rows, err := db.QueryContext(ctx,
		"SELECT status, COUNT(*) FROM builds GROUP BY status")
	if err != nil {
		return stats, fmt.Errorf("query builds: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			continue
		}
		stats.Total += count
		switch strings.ToLower(status) {
		case "shipped", "complete", "done":
			stats.Shipped += count
		case "failed", "error":
			stats.Failed += count
		case "queued", "pending":
			stats.Queued += count
		case "building", "in_progress":
			stats.InProgress += count
		}
	}

	// Collect recent error messages and patterns
	errRows, err := db.QueryContext(ctx,
		`SELECT error_message FROM builds
		 WHERE status IN ('failed', 'error') AND error_message != ''
		 ORDER BY updated_at DESC LIMIT 20`)
	if err != nil {
		// Non-fatal: table might not have error_message column
		return stats, nil
	}
	defer errRows.Close()

	for errRows.Next() {
		var msg string
		if err := errRows.Scan(&msg); err != nil {
			continue
		}
		stats.RecentErrors = append(stats.RecentErrors, msg)
		pattern := categorizeError(msg)
		stats.ErrorPatterns[pattern]++
	}

	return stats, nil
}

// categorizeError extracts a general pattern from an error message.
func categorizeError(msg string) string {
	lower := strings.ToLower(msg)
	switch {
	case strings.Contains(lower, "compile") || strings.Contains(lower, "build"):
		return "compilation_error"
	case strings.Contains(lower, "test"):
		return "test_failure"
	case strings.Contains(lower, "timeout") || strings.Contains(lower, "timed out"):
		return "timeout"
	case strings.Contains(lower, "rate limit") || strings.Contains(lower, "429"):
		return "rate_limit"
	case strings.Contains(lower, "permission") || strings.Contains(lower, "auth"):
		return "auth_error"
	case strings.Contains(lower, "not found") || strings.Contains(lower, "404"):
		return "not_found"
	case strings.Contains(lower, "prompt") || strings.Contains(lower, "token"):
		return "prompt_error"
	default:
		return "unknown"
	}
}
