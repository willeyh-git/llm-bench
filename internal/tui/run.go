package tui

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/willyhetland/llm-bencher/internal/bench"
)

var (
	okStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	warnStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
)

type phaseLine struct {
	name    string
	label   string
	done    bool
	failed  bool
	skipped bool
	detail  string
}

type runEventMsg struct {
	ev bench.Event
}

type runDoneMsg struct {
	res *bench.Result
	err error
}

type run struct {
	opts          Options
	model         string
	contextLength int
	spin          spinner.Model
	phases        []phaseLine
	state         *runState
}

type runState struct {
	events  chan bench.Event
	done    chan runDoneMsg
	cancel  context.CancelFunc
	result  *bench.Result
	err     error
	running bool
}

func newRun(opts Options, model string, contextLength int) run {
	s := spinner.New()
	s.Spinner = spinner.Dot
	return run{
		opts:          opts,
		model:         model,
		contextLength: contextLength,
		spin:          s,
		phases: []phaseLine{
			{name: bench.PhaseWarmup, label: "warm-up"},
			{name: bench.PhaseDecodeGreedy, label: fmt.Sprintf("decode greedy (%d runs)", opts.Runs.Count("greedy"))},
			{name: bench.PhaseDecode4K, label: fmt.Sprintf("4K prefill + decode (%d runs)", opts.Runs.Count("4k"))},
			{name: bench.PhaseDecode16K, label: fmt.Sprintf("16K prefill + decode (%d runs)", opts.Runs.Count("16k"))},
			{name: bench.PhaseDecode32K, label: fmt.Sprintf("32K prefill + decode (%d runs)", opts.Runs.Count("32k"))},
			{name: bench.PhaseDecode64K, label: fmt.Sprintf("64K prefill + decode (%d runs)", opts.Runs.Count("64k"))},
			{name: bench.PhaseDecode96K, label: fmt.Sprintf("96K prefill + decode (%d runs)", opts.Runs.Count("96k"))},
			{name: bench.PhaseDecode128K, label: fmt.Sprintf("128K prefill + decode (%d runs)", opts.Runs.Count("128k"))},
		},
	}
}

func (r run) start() (run, tea.Cmd) {
	ctx, cancel := context.WithCancel(context.Background())
	events := make(chan bench.Event, 16)
	done := make(chan runDoneMsg, 1)
	r.state = &runState{events: events, done: done, cancel: cancel, running: true}
	runner := &bench.Runner{Base: r.opts.BaseURL, Model: r.model, Out: r.opts.Out, ContextLength: r.contextLength, Runs: r.opts.Runs, Events: events}
	go func() {
		res, err := runner.Run(ctx)
		done <- runDoneMsg{res: res, err: err}
	}()
	return r, tea.Batch(r.spin.Tick, waitEvent(events, done))
}

func waitEvent(events <-chan bench.Event, done <-chan runDoneMsg) tea.Cmd {
	return func() tea.Msg {
		select {
		case ev := <-events:
			return runEventMsg{ev: ev}
		case d := <-done:
			return d
		}
	}
}

func (r run) update(msg tea.Msg) (run, tea.Cmd) {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		var cmd tea.Cmd
		r.spin, cmd = r.spin.Update(msg)
		return r, cmd
	case runEventMsg:
		if msg.ev.Kind == bench.EventDone {
			if result, ok := msg.ev.Data.(*bench.Result); ok {
				r.state.running = false
				r.state.result = result
				return r, nil
			}
		}
		r.apply(msg.ev)
		return r, waitEvent(r.state.events, r.state.done)
	case runDoneMsg:
		r.state.running = false
		r.state.result = msg.res
		r.state.err = msg.err
	}
	return r, nil
}

func (r *run) apply(ev bench.Event) {
	for i := range r.phases {
		p := &r.phases[i]
		if p.name != ev.Phase {
			continue
		}
		switch ev.Kind {
		case bench.EventPhaseEnd:
			p.done = true
			p.detail = describe(ev.Data)
		case bench.EventError:
			p.failed = true
			p.detail = fmt.Sprint(ev.Data)
		case bench.EventSkipped:
			p.skipped = true
			p.detail = fmt.Sprint(ev.Data)
		}
	}
}

func describe(data any) string {
	switch d := data.(type) {
	case bench.ContextDecode:
		return fmt.Sprintf("prefill %.1f tok/s, TTFT %.2fs · decode median %.1f tok/s", d.Prefill.TPS, d.Prefill.TTFTSeconds, d.Decode.TPSMedian)
	case bench.DecodeResult:
		return fmt.Sprintf("median %.1f tok/s", d.TPSMedian)
	}
	return ""
}

func (r run) view() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Benchmarking "+r.model) + "\n\n")
	for _, p := range r.phases {
		mark := r.spin.View()
		switch {
		case p.failed:
			mark = warnStyle.Render("✗")
		case p.skipped:
			mark = subtleStyle.Render("–")
		case p.done:
			mark = okStyle.Render("✓")
		}
		b.WriteString(fmt.Sprintf("%s %-26s %s\n", mark, p.label, p.detail))
	}
	if r.state != nil && r.state.err != nil && !r.state.running {
		b.WriteString("\n" + errorStyle.Render("Run failed: "+r.state.err.Error()) + "\n")
	}
	b.WriteString("\n" + subtleStyle.Render("ctrl+c cancel"))
	return b.String()
}
