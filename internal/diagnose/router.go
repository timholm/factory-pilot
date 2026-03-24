package diagnose

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// RouterCollector gathers stats from the llm-router service.
type RouterCollector struct {
	baseURL string
	client  *http.Client
}

// NewRouterCollector creates a router stats collector.
func NewRouterCollector() *RouterCollector {
	return &RouterCollector{
		baseURL: "http://llm-router.factory.svc.cluster.local:8080",
		client:  &http.Client{Timeout: 10 * time.Second},
	}
}

// routerStatsResponse is the expected JSON from llm-router /stats.
type routerStatsResponse struct {
	TotalRequests int            `json:"total_requests"`
	ModelCounts   map[string]int `json:"model_counts"`
	ErrorRate     float64        `json:"error_rate"`
	AvgLatencyMs  float64        `json:"avg_latency_ms"`
}

// CollectStats queries the llm-router /stats endpoint.
func (r *RouterCollector) CollectStats(ctx context.Context) (RouterStats, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.baseURL+"/stats", nil)
	if err != nil {
		return RouterStats{}, fmt.Errorf("create request: %w", err)
	}

	resp, err := r.client.Do(req)
	if err != nil {
		return RouterStats{}, fmt.Errorf("router /stats: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return RouterStats{}, fmt.Errorf("router /stats returned %d: %s", resp.StatusCode, string(body))
	}

	var raw routerStatsResponse
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return RouterStats{}, fmt.Errorf("decode router stats: %w", err)
	}

	return RouterStats{
		TotalRequests: raw.TotalRequests,
		ModelCounts:   raw.ModelCounts,
		ErrorRate:     raw.ErrorRate,
		AvgLatencyMs:  raw.AvgLatencyMs,
	}, nil
}
