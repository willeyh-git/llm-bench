package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/willyhetland/llm-bencher/internal/bench"
	"github.com/willyhetland/llm-bencher/internal/config"
	"github.com/willyhetland/llm-bencher/internal/tui"
)

func main() {
	var opts tui.Options
	flag.StringVar(&opts.BaseURL, "base-url", "", "OpenAI-compatible base URL for this session (not saved to "+config.FileName+")")
	flag.StringVar(&opts.Out, "out", "", "result JSON path (default results/<UTC timestamp>-<model slug>.json)")
	flag.StringVar(&opts.Model, "model", "", "model id to benchmark, skips the picker")
	flag.Parse()

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
