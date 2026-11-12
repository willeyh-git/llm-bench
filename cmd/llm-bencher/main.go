package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/willyhetland/llm-bencher/internal/bench"
	"github.com/willyhetland/llm-bencher/internal/config"
	"github.com/willyhetland/llm-bencher/internal/tui"
)

func main() {
	var opts tui.Options
	var contexts string
	flag.StringVar(&opts.BaseURL, "base-url", "", "OpenAI-compatible base URL for this session (not saved to "+config.FileName+")")
	flag.StringVar(&opts.Out, "out", "", "result JSON path (default results/<UTC timestamp>-<model slug>.json)")
	flag.StringVar(&opts.Model, "model", "", "model id to benchmark, skips the picker")
	flag.StringVar(&opts.Tag, "tag", "", "optional tag to associate with the benchmark result")
	flag.StringVar(&contexts, "contexts", "", "context tiers to run, comma-separated: 4,16,32,64,96,128 (default: all)")
	flag.Parse()
	var err error
	if opts.Contexts, err = parseContexts(contexts); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	cfg, err := config.Load(config.FileName)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	opts.Runs = cfg.Runs
	if opts.BaseURL == "" {
		opts.BaseURL = cfg.BaseURL
		opts.ConfigPath = config.FileName
	}

	if err := bench.VerifyCorpus(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	final, err := tea.NewProgram(tui.NewModel(opts)).Run()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if m, ok := final.(tui.Model); ok && m.Selected != "" {
		fmt.Printf("selected model: %s\n", m.Selected)
	}
}

func parseContexts(value string) ([]string, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	valid := map[string]bool{"4": true, "16": true, "32": true, "64": true, "96": true, "128": true}
	seen := make(map[string]bool)
	var contexts []string
	for _, item := range strings.Split(value, ",") {
		item = strings.TrimSpace(item)
		if !valid[item] {
			return nil, fmt.Errorf("invalid context %q; choose from 4,16,32,64,96,128", item)
		}
		if seen[item] {
			return nil, fmt.Errorf("duplicate context %q", item)
		}
		seen[item] = true
		contexts = append(contexts, item+"k")
	}
	return contexts, nil
}
