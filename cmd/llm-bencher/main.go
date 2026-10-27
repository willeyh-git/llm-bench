package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/willyhetland/llm-bencher/internal/bench"
	"github.com/willyhetland/llm-bencher/internal/tui"
)

const defaultBaseURL = "http://gaming:8080/v1"

func main() {
	var opts tui.Options
	flag.StringVar(&opts.BaseURL, "base-url", defaultBaseURL, "OpenAI-compatible base URL")
	flag.StringVar(&opts.Out, "out", "", "result JSON path (default results/<UTC timestamp>-<model slug>.json)")
	flag.StringVar(&opts.Model, "model", "", "model id to benchmark, skips the picker")
	flag.Parse()

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
