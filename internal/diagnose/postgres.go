package diagnose

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// PostgresCollector queries Postgres for paper and candidate stats.
type PostgresCollector struct {
	connStr string
}

// NewPostgresCollector creates a Postgres collector.
func NewPostgresCollector(connStr string) *PostgresCollector {
	return &PostgresCollector{connStr: connStr}
}

// CollectPapers returns paper ingestion statistics.
func (p *PostgresCollector) CollectPapers(ctx context.Context) (PaperStats, error) {
	conn, err := pgx.Connect(ctx, p.connStr)
	if err != nil {
		return PaperStats{}, fmt.Errorf("postgres connect: %w", err)
	}
	defer conn.Close(ctx)

	var stats PaperStats

	err = conn.QueryRow(ctx, "SELECT COUNT(*) FROM papers").Scan(&stats.Total)
	if err != nil {
		return stats, fmt.Errorf("count papers: %w", err)
	}

	err = conn.QueryRow(ctx,
		"SELECT COUNT(*) FROM papers WHERE embedding IS NOT NULL").Scan(&stats.Embedded)
	if err != nil {
		return stats, fmt.Errorf("count embedded: %w", err)
	}

	err = conn.QueryRow(ctx,
		"SELECT COUNT(*) FROM papers WHERE created_at > NOW() - INTERVAL '24 hours'").Scan(&stats.Recent)
	if err != nil {
		return stats, fmt.Errorf("count recent: %w", err)
	}

	return stats, nil
}

// CollectCandidates returns idea-engine candidate pipeline stats.
func (p *PostgresCollector) CollectCandidates(ctx context.Context) (CandidateStats, error) {
	conn, err := pgx.Connect(ctx, p.connStr)
	if err != nil {
		return CandidateStats{}, fmt.Errorf("postgres connect: %w", err)
	}
	defer conn.Close(ctx)

	var stats CandidateStats
	stats.ByStatus = make(map[string]int)

	err = conn.QueryRow(ctx, "SELECT COUNT(*) FROM candidates").Scan(&stats.Total)
	if err != nil {
		return stats, fmt.Errorf("count candidates: %w", err)
	}

	rows, err := conn.Query(ctx, "SELECT status, COUNT(*) FROM candidates GROUP BY status")
	if err != nil {
		return stats, fmt.Errorf("group candidates: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			continue
		}
		stats.ByStatus[status] = count
		switch status {
		case "approved":
			stats.Approved = count
		case "rejected":
			stats.Rejected = count
		case "pending":
			stats.Pending = count
		case "shipped":
			stats.Shipped = count
		}
	}

	return stats, nil
}
