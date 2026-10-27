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
			{name: bench.PhasePrefill4K, label: "prefill 4K"},
			{name: bench.PhasePrefill16K, label: "prefill 16K"},
			{name: bench.PhasePrefill32K, label: "prefill 32K"},
			{name: bench.PhasePrefill64K, label: "prefill 64K"},
			{name: bench.PhasePrefill96K, label: "prefill 96K"},
			{name: bench.PhasePrefill128K, label: "prefill 128K"},
			{name: bench.PhaseDecodeGreedy, label: "decode greedy (5 runs)"},
			{name: bench.PhaseDecode4K, label: "decode 4K (5 runs)"},
			{name: bench.PhaseDecode16K, label: "decode 16K (5 runs)"},
			{name: bench.PhaseDecode32K, label: "decode 32K (5 runs)"},
			{name: bench.PhaseDecode64K, label: "decode 64K (5 runs)"},
			{name: bench.PhaseDecode96K, label: "decode 96K (2 runs)"},
			{name: bench.PhaseDecode128K, label: "decode 128K (2 runs)"},
		},
	}
}

func (r run) start() (run, tea.Cmd) {
	ctx, cancel := context.WithCancel(context.Background())
	events := make(chan bench.Event, 16)
	done := make(chan runDoneMsg, 1)
	r.state = &runState{events: events, done: done, cancel: cancel, running: true}
	runner := &bench.Runner{Base: r.opts.BaseURL, Model: r.model, Out: r.opts.Out, ContextLength: r.contextLength, Events: events}
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
	case bench.PrefillResult:
		if len(d.Invalid) > 0 {
			return fmt.Sprintf("%.1f tok/s, TTFT %.2fs · invalid: %s", d.TPS, d.TTFTSeconds, strings.Join(d.Invalid, "; "))
		}
		return fmt.Sprintf("%.1f tok/s, TTFT %.2fs", d.TPS, d.TTFTSeconds)
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
