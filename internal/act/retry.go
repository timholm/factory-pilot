package act

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

// RetryRunner resets failed builds and re-queues them.
type RetryRunner struct {
	dataDir string
}

// NewRetryRunner creates a retry runner.
func NewRetryRunner(dataDir string) *RetryRunner {
	return &RetryRunner{dataDir: dataDir}
}

// Run executes a retry command. Supported formats:
//   - "retry build <id>" — reset a specific failed build to queued
//   - "retry all-failed" — reset all failed builds to queued
//   - "retry recent <n>" — reset the N most recent failed builds
func (r *RetryRunner) Run(command string) (string, error) {
	parts := strings.Fields(command)
	if len(parts) < 2 {
		return "", fmt.Errorf("retry command too short: %s", command)
	}

	dbPath := filepath.Join(r.dataDir, "factory.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return "", fmt.Errorf("open factory db: %w", err)
	}
	defer db.Close()

	switch parts[1] {
	case "build":
		if len(parts) < 3 {
			return "", fmt.Errorf("retry build requires an ID")
		}
		return r.retryBuild(db, parts[2])

	case "all-failed":
		return r.retryAllFailed(db)

	case "recent":
		n := 5
		if len(parts) >= 3 {
			n, _ = strconv.Atoi(parts[2])
			if n <= 0 {
				n = 5
			}
		}
		return r.retryRecent(db, n)

	default:
		return "", fmt.Errorf("unknown retry subcommand: %s", parts[1])
	}
}

func (r *RetryRunner) retryBuild(db *sql.DB, id string) (string, error) {
	result, err := db.Exec(
		"UPDATE builds SET status = 'queued', error_message = '', updated_at = CURRENT_TIMESTAMP WHERE id = ? AND status IN ('failed', 'error')",
		id)
	if err != nil {
		return "", fmt.Errorf("retry build %s: %w", id, err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return "", fmt.Errorf("build %s not found or not in failed state", id)
	}
	return fmt.Sprintf("reset build %s to queued", id), nil
}

func (r *RetryRunner) retryAllFailed(db *sql.DB) (string, error) {
	result, err := db.Exec(
		"UPDATE builds SET status = 'queued', error_message = '', updated_at = CURRENT_TIMESTAMP WHERE status IN ('failed', 'error')")
	if err != nil {
		return "", fmt.Errorf("retry all failed: %w", err)
	}
	rows, _ := result.RowsAffected()
	return fmt.Sprintf("reset %d failed builds to queued", rows), nil
}

func (r *RetryRunner) retryRecent(db *sql.DB, n int) (string, error) {
	result, err := db.Exec(
		`UPDATE builds SET status = 'queued', error_message = '', updated_at = CURRENT_TIMESTAMP
		 WHERE id IN (
			 SELECT id FROM builds WHERE status IN ('failed', 'error')
			 ORDER BY updated_at DESC LIMIT ?
		 )`, n)
	if err != nil {
		return "", fmt.Errorf("retry recent %d: %w", n, err)
	}
	rows, _ := result.RowsAffected()
	return fmt.Sprintf("reset %d recent failed builds to queued", rows), nil
}
