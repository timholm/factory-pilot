package think

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"sort"
	"strings"

	"github.com/timholm/factory-pilot/internal/config"
	"github.com/timholm/factory-pilot/internal/diagnose"
)

const diagnosisPrompt = `You are a brutal quality auditor for a software factory. Your job is to find everything that is broken, half-assed, or not actually working — and demand it gets fixed.

DO NOT BE POLITE. DO NOT SAY "LOOKS GOOD." If something shipped without tests, say it. If a Docker image has the wrong Go version, say it. If repos are empty on GitHub, say it. If the embedder keeps dying, say why and how to make it never die again.

## Current System Status
%s

## What You Must Check
1. **Are repos actually shipping to GitHub with real code?** Not empty repos. Not repos blocked by push protection. Real code that compiles.
2. **Does every shipped repo have: working tests, correct module path (github.com/timholm/X), README with references, no leaked secrets?** If not, which ones fail and why.
3. **What is the ACTUAL ship rate?** Shipped / (shipped + failed). If below 60%%, this is a CRITICAL failure.
4. **Are embeddings making progress?** What %% complete. If stalled, why.
5. **Is the idea-engine finding 7 papers AND 7 repos per candidate?** If finding 0 papers, why. If repos are generic keyword matches instead of real implementations, say so.
6. **Are Claude credentials working in all pods?** Test it.
7. **What is the ACTUAL throughput?** How many repos shipped to GitHub in the last 2 hours. If zero, this is a CRITICAL failure.
8. **Is anything silently failing?** Pods that look "Running" but are actually stuck, processes that log "done" but did nothing, env vars that aren't set.

## What You Can Fix
- kubectl: restart pods, scale deployments
- code: edit source files to fix bugs
- prompt: improve build/seo/review templates
- retry: reset failed builds
- config: change env vars or K8s manifests
- docker: rebuild and push Docker images
- evolve: rewrite prompts based on failure analysis

## Output
JSON array of issues, max 15, ordered by severity. Be SPECIFIC — not "improve quality" but "repo X has no tests because the build prompt doesn't enforce pytest discovery for Python projects."
[{
  "issue": "brief title",
  "severity": "critical|high|medium|low",
  "root_cause": "the REAL reason, not a symptom",
  "fix_type": "kubectl|code|prompt|retry|config|docker|evolve",
  "fix_commands": ["exact command 1", "exact command 2"],
  "expected_outcome": "measurable result — not 'should improve' but 'ship rate goes from 12%% to 60%%'"
}]

IMPORTANT: Return ONLY the JSON array. No markdown, no commentary. If you return fewer than 5 issues, you're not looking hard enough.`

// Thinker uses Claude Opus with extended thinking to analyze system status.
type Thinker struct {
	cfg *config.Config
}

// NewThinker creates a new thinker.
func NewThinker(cfg *config.Config) *Thinker {
	return &Thinker{cfg: cfg}
}

// Analyze sends the full system status to Claude Opus and returns prioritized issues.
func (t *Thinker) Analyze(status *diagnose.SystemStatus) ([]Issue, error) {
	report := diagnose.FormatReport(status)
	prompt := fmt.Sprintf(diagnosisPrompt, report)

	issues, err := t.callClaude(prompt)
	if err != nil {
		return nil, fmt.Errorf("claude analysis: %w", err)
	}

	// Sort by severity
	sort.Slice(issues, func(i, j int) bool {
		return SeverityOrder(issues[i].Severity) < SeverityOrder(issues[j].Severity)
	})

	// Cap at max fixes
	if len(issues) > t.cfg.MaxFixes {
		issues = issues[:t.cfg.MaxFixes]
	}

	return issues, nil
}

// callClaude invokes the Claude CLI with the analysis prompt.
func (t *Thinker) callClaude(prompt string) ([]Issue, error) {
	cmd := exec.Command(t.cfg.ClaudeBinary,
		"-p", prompt,
		"--model", "opus",
		"--max-turns", "3",
		"--output-format", "text",
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("claude exec: %s: %w", stderr.String(), err)
	}

	return parseResponse(stdout.String())
}

// parseResponse extracts the JSON array from Claude's response.
func parseResponse(raw string) ([]Issue, error) {
	// Find JSON array in response — Claude might wrap it in markdown
	text := strings.TrimSpace(raw)

	// Strip markdown code fences if present
	if idx := strings.Index(text, "["); idx >= 0 {
		text = text[idx:]
	}
	if idx := strings.LastIndex(text, "]"); idx >= 0 {
		text = text[:idx+1]
	}

	var issues []Issue
	if err := json.Unmarshal([]byte(text), &issues); err != nil {
		return nil, fmt.Errorf("parse issues JSON: %w (raw: %s)", err, truncate(raw, 500))
	}

	return issues, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
