package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/timholm/factory-pilot/internal/act"
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

	// 1. DIAGNOSE
	log.Println("[cycle] collecting system status...")
	status := collector.Collect(ctx)
	log.Printf("[cycle] diagnosis complete: %d errors during collection", len(status.Errors))
	for _, e := range status.Errors {
		log.Printf("[cycle]   %s", e)
	}

	// 2. THINK
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

	// 3. ACT
	log.Println("[cycle] executing fixes...")
	actions := executor.Execute(issues)

	// 4. REPORT
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
