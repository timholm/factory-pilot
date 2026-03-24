package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/timholm/factory-pilot/internal/act"
	"github.com/timholm/factory-pilot/internal/analyze"
	"github.com/timholm/factory-pilot/internal/api"
	"github.com/timholm/factory-pilot/internal/config"
	"github.com/timholm/factory-pilot/internal/diagnose"
	"github.com/timholm/factory-pilot/internal/report"
	"github.com/timholm/factory-pilot/internal/think"
)

var version = "dev"

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	cfg := config.Load()

	// Check for --execute flag to disable dry-run
	for i, arg := range os.Args {
		if arg == "--execute" {
			cfg.DryRun = false
			os.Args = append(os.Args[:i], os.Args[i+1:]...)
			break
		}
	}

	switch os.Args[1] {
	case "run":
		runLoop(cfg)
	case "diagnose":
		runDiagnose(cfg)
	case "report":
		runReport(cfg)
	case "serve":
		runServe(cfg)
	case "analyze":
		runAnalyze(cfg)
	case "evolve":
		runEvolve(cfg)
	case "fix-code":
		runFixCode(cfg)
	case "version":
		fmt.Printf("factory-pilot %s\n", version)
	default:
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `factory-pilot — autonomous factory improvement agent

Usage:
  factory-pilot <command> [flags]

Commands:
  run        Start the continuous improvement loop
  diagnose   Run one diagnosis cycle and print results
  report     Print the latest daily report
  serve      Start the HTTP monitoring API
  analyze    Analyze build outcomes (ship rate, failure patterns)
  evolve     Analyze builds + evolve prompt templates
  fix-code   Clone a repo and fix it with Claude
               --repo <name>   GitHub repo name (e.g. "my-tool")

Flags:
  --execute  Actually apply fixes (default is dry-run)

Version: %s
`, version)
}

// runLoop is the main continuous improvement loop.
func runLoop(cfg *config.Config) {
	log.Printf("[pilot] starting factory-pilot %s (cycle=%s, max_fixes=%d, dry_run=%v)",
		version, cfg.CycleInterval, cfg.MaxFixes, cfg.DryRun)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Graceful shutdown
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sig
		log.Println("[pilot] shutting down...")
		cancel()
	}()

	collector := diagnose.NewCollector(cfg)
	thinker := think.NewThinker(cfg)
	executor := act.NewExecutor(cfg)
	reporter := report.NewReporter(cfg)

	// Start the HTTP API in the background so K8s probes work
	server := api.NewServer(cfg, collector, reporter)
	go func() {
		if err := server.ListenAndServe(); err != nil {
			log.Printf("[pilot] api server: %v", err)
		}
	}()

	// Ensure report table exists
	if err := reporter.EnsureTable(ctx); err != nil {
		log.Printf("[pilot] warning: could not ensure report table: %v", err)
	}

	// Run first cycle immediately
	runCycle(ctx, cfg, collector, thinker, executor, reporter)

	// Then loop on interval
	ticker := time.NewTicker(cfg.CycleInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("[pilot] stopped")
			return
		case <-ticker.C:
			runCycle(ctx, cfg, collector, thinker, executor, reporter)
		}
	}
}

func runCycle(
	ctx context.Context,
	cfg *config.Config,
	collector *diagnose.Collector,
	thinker *think.Thinker,
	executor *act.Executor,
	reporter *report.Reporter,
) {
	cycleStart := time.Now()
	log.Println("[cycle] === starting improvement cycle ===")

	// 1. COLLECT system status (pods, Postgres, SQLite, router, GitHub)
	log.Println("[cycle] collecting system status...")
	status := collector.Collect(ctx)
	log.Printf("[cycle] diagnosis complete: %d errors during collection", len(status.Errors))
	for _, e := range status.Errors {
		log.Printf("[cycle]   %s", e)
	}

	// 2. ANALYZE builds (ship rate, failure patterns) — already included in status via collector
	if status.BuildAnalysis != nil {
		log.Printf("[cycle] build analysis: ship_rate=%.1f%% shipped=%d failed=%d patterns=%d",
			status.BuildAnalysis.ShipRate*100,
			status.BuildAnalysis.ShippedCount,
			status.BuildAnalysis.FailedCount,
			len(status.BuildAnalysis.FailureGroups))
	}

	// 3. THINK — send EVERYTHING to Opus for deep analysis
	log.Println("[cycle] analyzing with Claude Opus...")
	issues, err := thinker.Analyze(status)
	if err != nil {
		log.Printf("[cycle] ERROR: analysis failed: %v", err)
		// Generate report even without analysis
		rep := reporter.Generate(status, nil, nil)
		rep.Outcome = fmt.Sprintf("analysis failed: %v", err)
		if saveErr := reporter.Save(ctx, rep); saveErr != nil {
			log.Printf("[cycle] ERROR: could not save report: %v", saveErr)
		}
		reporter.Print(rep)
		return
	}
	log.Printf("[cycle] found %d issues", len(issues))

	// 4. ACT — execute fixes (kubectl, code edits, prompt evolution, docker rebuilds)
	log.Println("[cycle] executing fixes...")
	actions := executor.Execute(issues)

	// 5. REPORT
	rep := reporter.Generate(status, issues, actions)
	if err := reporter.Save(ctx, rep); err != nil {
		log.Printf("[cycle] ERROR: could not save report: %v", err)
	}
	reporter.Print(rep)

	log.Printf("[cycle] === cycle complete in %s ===", time.Since(cycleStart))
}

// runDiagnose runs a single diagnosis and prints the report.
func runDiagnose(cfg *config.Config) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	collector := diagnose.NewCollector(cfg)
	status := collector.Collect(ctx)
	fmt.Print(diagnose.FormatReport(status))
}

// runReport prints the latest daily report.
func runReport(cfg *config.Config) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	reporter := report.NewReporter(cfg)
	if err := reporter.PrintLatest(ctx); err != nil {
		log.Fatalf("report: %v", err)
	}
}

// runServe starts the HTTP monitoring API.
func runServe(cfg *config.Config) {
	collector := diagnose.NewCollector(cfg)
	reporter := report.NewReporter(cfg)

	ctx := context.Background()
	if err := reporter.EnsureTable(ctx); err != nil {
		log.Printf("[serve] warning: could not ensure report table: %v", err)
	}

	server := api.NewServer(cfg, collector, reporter)
	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("server: %v", err)
	}
}

// runAnalyze runs build analysis and prints the report.
func runAnalyze(cfg *config.Config) {
	dbPath := filepath.Join(cfg.FactoryDataDir, "factory.db")

	report, err := analyze.AnalyzeBuilds(dbPath)
	if err != nil {
		log.Fatalf("analyze: %v", err)
	}

	fmt.Println(report.String())
}

// runEvolve runs build analysis + prompt evolution.
func runEvolve(cfg *config.Config) {
	dbPath := filepath.Join(cfg.FactoryDataDir, "factory.db")

	log.Println("[evolve] analyzing builds...")
	buildReport, err := analyze.AnalyzeBuilds(dbPath)
	if err != nil {
		log.Fatalf("analyze: %v", err)
	}

	fmt.Println(buildReport.String())

	factoryRepoPath := filepath.Join(cfg.FactoryGitDir, "claude-code-factory")

	log.Println("[evolve] evolving prompts...")
	if err := act.EvolvePrompts(buildReport, factoryRepoPath, cfg.ClaudeBinary); err != nil {
		log.Fatalf("evolve: %v", err)
	}

	log.Println("[evolve] prompts evolved and pushed successfully")
}

// runFixCode clones a repo and fixes it with Claude.
func runFixCode(cfg *config.Config) {
	var repoName string
	var prompt string

	// Parse --repo flag
	for i, arg := range os.Args[2:] {
		if arg == "--repo" && i+1 < len(os.Args[2:])-1 {
			repoName = os.Args[2+i+1]
		}
	}

	if repoName == "" {
		fmt.Fprintln(os.Stderr, "Usage: factory-pilot fix-code --repo <name> [prompt]")
		fmt.Fprintln(os.Stderr, "Example: factory-pilot fix-code --repo my-tool 'fix the failing tests'")
		os.Exit(1)
	}

	// Remaining args after --repo <name> are the prompt
	args := os.Args[2:]
	var remaining []string
	skip := false
	for _, arg := range args {
		if arg == "--repo" {
			skip = true
			continue
		}
		if skip {
			skip = false
			continue
		}
		remaining = append(remaining, arg)
	}
	if len(remaining) > 0 {
		prompt = remaining[0]
	} else {
		prompt = "Fix all issues: ensure go build ./... and go test ./... pass, fix lint issues, ensure proper error handling."
	}

	repoURL := fmt.Sprintf("https://github.com/%s/%s.git", cfg.GithubUser, repoName)

	log.Printf("[fix-code] cloning %s and applying fix...", repoURL)
	if err := act.CloneAndFix(repoURL, prompt, cfg.ClaudeBinary); err != nil {
		log.Fatalf("fix-code: %v", err)
	}

	log.Println("[fix-code] fix applied and pushed successfully")
}
