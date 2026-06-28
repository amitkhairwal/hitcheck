package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/amitkhairwal/hitcheck/internal/check"
	"github.com/amitkhairwal/hitcheck/internal/config"
	"github.com/amitkhairwal/hitcheck/internal/load"
	"github.com/amitkhairwal/hitcheck/internal/report"
)

const version = "0.2.0"

const usage = `hitcheck - API health monitor and load tester

Usage:
  hitcheck check [flags] [url...]   run health checks once
  hitcheck watch [flags]            re-run checks on an interval
  hitcheck load  [flags]            load test a single endpoint
  hitcheck version                  print version

Run "hitcheck <command> -h" for the flags of a command.`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, usage)
		os.Exit(2)
	}

	// Cancel everything cleanly on Ctrl-C.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cmd, args := os.Args[1], os.Args[2:]

	var err error
	switch cmd {
	case "check":
		err = runCheck(ctx, args)
	case "watch":
		err = runWatch(ctx, args)
	case "load":
		err = runLoad(ctx, args)
	case "version", "-v", "--version":
		fmt.Printf("hitcheck %s\n", version)
	case "help", "-h", "--help":
		fmt.Println(usage)
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n%s\n", cmd, usage)
		os.Exit(2)
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func runCheck(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("check", flag.ExitOnError)
	cfgPath := fs.String("config", "", "path to a YAML config file")
	timeout := fs.Duration("timeout", 5*time.Second, "per-request timeout")
	jsonOut := fs.String("json", "", "write results to a JSON file")
	csvOut := fs.String("csv", "", "write results to a CSV file")
	fs.Parse(args)

	cfg, err := buildConfig(*cfgPath, fs.Args(), *timeout)
	if err != nil {
		return err
	}

	results := check.Run(ctx, cfg)
	report.CheckTable(os.Stdout, results)

	if err := exportChecks(results, *jsonOut, *csvOut); err != nil {
		return err
	}

	// Non-zero exit if anything is down, handy for cron and CI.
	if !check.Healthy(results) {
		os.Exit(1)
	}
	return nil
}

func runWatch(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("watch", flag.ExitOnError)
	cfgPath := fs.String("config", "", "path to a YAML config file")
	timeout := fs.Duration("timeout", 5*time.Second, "per-request timeout")
	interval := fs.Duration("interval", 0, "delay between runs (overrides config)")
	fs.Parse(args)

	cfg, err := buildConfig(*cfgPath, fs.Args(), *timeout)
	if err != nil {
		return err
	}
	if *interval > 0 {
		cfg.Interval.Duration = *interval
	}

	ticker := time.NewTicker(cfg.Interval.Duration)
	defer ticker.Stop()

	for {
		results := check.Run(ctx, cfg)
		clearScreen()
		fmt.Printf("hitcheck watch - every %s - %s\n\n",
			cfg.Interval.Duration, time.Now().Format("15:04:05"))
		report.CheckTable(os.Stdout, results)
		fmt.Println("\nCtrl-C to stop")

		select {
		case <-ctx.Done():
			fmt.Println("\nstopped")
			return nil
		case <-ticker.C:
		}
	}
}

func runLoad(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("load", flag.ExitOnError)
	url := fs.String("url", "", "target URL (required)")
	method := fs.String("method", "GET", "HTTP method")
	requests := fs.Int("n", 200, "total number of requests")
	concurrency := fs.Int("c", 20, "number of concurrent workers")
	timeout := fs.Duration("timeout", 10*time.Second, "per-request timeout")
	jsonOut := fs.String("json", "", "write results to a JSON file")
	csvOut := fs.String("csv", "", "write results to a CSV file")

	var headers headerFlag
	fs.Var(&headers, "H", "request header as 'Key: Value' (repeatable)")
	fs.Parse(args)

	if *url == "" {
		return fmt.Errorf("load: -url is required")
	}
	if *requests < 1 {
		return fmt.Errorf("load: -n must be at least 1")
	}
	if *concurrency > 1000 {
		return fmt.Errorf("load: -c is capped at 1000")
	}

	opts := load.Options{
		URL:         *url,
		Method:      *method,
		Requests:    *requests,
		Concurrency: *concurrency,
		Timeout:     *timeout,
		Headers:     headers.parsed(),
	}

	fmt.Printf("running %d requests at concurrency %d...\n\n", opts.Requests, opts.Concurrency)
	result := load.Run(ctx, opts)
	report.LoadSummary(os.Stdout, result)

	if *jsonOut != "" {
		if err := report.WriteJSON(*jsonOut, result); err != nil {
			return err
		}
		fmt.Printf("\nwrote %s\n", *jsonOut)
	}
	if *csvOut != "" {
		if err := report.LoadCSV(*csvOut, result); err != nil {
			return err
		}
		fmt.Printf("wrote %s\n", *csvOut)
	}
	return nil
}

// buildConfig loads a YAML config when a path is given, otherwise builds one
// from URLs passed on the command line.
func buildConfig(path string, urls []string, timeout time.Duration) (*config.Config, error) {
	if path != "" {
		return config.Load(path)
	}
	if len(urls) == 0 {
		return nil, fmt.Errorf("provide -config or one or more URLs")
	}

	cfg := &config.Config{}
	cfg.Timeout.Duration = timeout
	cfg.Interval.Duration = 10 * time.Second
	for _, u := range urls {
		cfg.Endpoints = append(cfg.Endpoints, config.Endpoint{Name: u, URL: u})
	}
	return cfg, nil
}

func exportChecks(results []check.Result, jsonOut, csvOut string) error {
	if jsonOut != "" {
		if err := report.WriteJSON(jsonOut, check.SortByName(results)); err != nil {
			return err
		}
		fmt.Printf("\nwrote %s\n", jsonOut)
	}
	if csvOut != "" {
		if err := report.CheckCSV(csvOut, check.SortByName(results)); err != nil {
			return err
		}
		fmt.Printf("wrote %s\n", csvOut)
	}
	return nil
}

func clearScreen() {
	// ANSI: move cursor home, clear screen below.
	fmt.Print("\033[H\033[2J")
}

// headerFlag collects repeated -H values and splits them into a map.
type headerFlag []string

func (h *headerFlag) String() string { return strings.Join(*h, ", ") }

func (h *headerFlag) Set(v string) error {
	if !strings.Contains(v, ":") {
		return fmt.Errorf("header must be 'Key: Value', got %q", v)
	}
	*h = append(*h, v)
	return nil
}

func (h headerFlag) parsed() map[string]string {
	if len(h) == 0 {
		return nil
	}
	out := make(map[string]string, len(h))
	for _, raw := range h {
		k, v, _ := strings.Cut(raw, ":")
		out[strings.TrimSpace(k)] = strings.TrimSpace(v)
	}
	return out
}
