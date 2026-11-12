package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/willyhetland/llm-bencher/internal/config"
)

type Options struct {
	BaseURL    string
	Out        string
	Model      string
	Tag        string
	Contexts   []string
	Runs       config.Runs
	ConfigPath string
}

type screen int

const (
	screenBaseURL screen = iota
	screenPicker
	screenRun
	screenSummary
)

type Model struct {
	opts     Options
	screen   screen
	url      baseURLPrompt
	picker   picker
	run      run
	Selected string
}

func NewModel(opts Options) Model {
	m := Model{
		opts:     opts,
		screen:   screenPicker,
		picker:   newPicker(opts.BaseURL),
		Selected: opts.Model,
	}
	if opts.BaseURL == "" {
		m.screen = screenBaseURL
		m.url = newBaseURLPrompt()
		return m
	}
	if opts.Model != "" {
		m.screen = screenRun
		m.run = newRun(opts, opts.Model, 0)
	}
	return m
}

func (m Model) Init() tea.Cmd {
	switch m.screen {
	case screenBaseURL:
		return m.url.Init()
	case screenRun:
		var cmd tea.Cmd
		m.run, cmd = m.run.start()
		return cmd
	}
	return m.picker.Init()
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" && m.screen == screenRun && m.run.state != nil && m.run.state.running {
			m.run.state.cancel()
			return m, tea.Quit
		}
	}

	switch m.screen {
	case screenBaseURL:
		return m.updateBaseURL(msg)
	case screenPicker:
		return m.updatePicker(msg)
	case screenRun:
		return m.updateRun(msg)
	case screenSummary:
		return m.updateSummary(msg)
	}
	return m, nil
}

func (m Model) updateBaseURL(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.url, cmd = m.url.Update(msg)
	if m.url.quit {
		return m, tea.Quit
	}
	if m.url.chosen == "" {
		return m, cmd
	}
	if m.opts.ConfigPath != "" {
		if err := config.Save(m.opts.ConfigPath, config.Config{BaseURL: m.url.chosen, Runs: m.opts.Runs}); err != nil {
			m.url.chosen = ""
			m.url.err = err
			return m, cmd
		}
	}
	m.opts.BaseURL = m.url.chosen
	m.picker = newPicker(m.opts.BaseURL)
	if m.opts.Model != "" {
		m.Selected = m.opts.Model
		m.run = newRun(m.opts, m.opts.Model, 0)
		m.screen = screenRun
		var startCmd tea.Cmd
		m.run, startCmd = m.run.start()
		return m, tea.Batch(cmd, startCmd)
	}
	m.screen = screenPicker
	return m, tea.Batch(cmd, m.picker.Init())
}

func (m Model) updatePicker(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.picker, cmd = m.picker.Update(msg)
	if m.picker.chosen != "" {
		m.Selected = m.picker.chosen
		m.run = newRun(m.opts, m.Selected, m.picker.contextLength(m.Selected))
		m.screen = screenRun
		var startCmd tea.Cmd
		m.run, startCmd = m.run.start()
		return m, tea.Batch(cmd, startCmd)
	}
	return m, cmd
}

func (m Model) updateRun(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.run, cmd = m.run.update(msg)
	if m.run.state == nil {
		return m, cmd
	}
	if !m.run.state.running && m.run.state.result != nil {
		m.screen = screenSummary
		return m, nil
	}
	if !m.run.state.running && m.run.state.err != nil {
		m.screen = screenPicker
		m.picker = newPicker(m.opts.BaseURL)
		return m, tea.Batch(cmd, m.picker.Init())
	}
	return m, cmd
}

func (m Model) updateSummary(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch key.String() {
	case "q", "esc", "ctrl+c":
		return m, tea.Quit
	case "enter":
		m.screen = screenPicker
		m.Selected = ""
		m.picker = newPicker(m.opts.BaseURL)
		return m, m.picker.Init()
	}
	return m, nil
}

func (m Model) View() string {
	switch m.screen {
	case screenBaseURL:
		return m.url.View()
	case screenRun:
		return m.run.view()
	case screenSummary:
		return summaryView(m.run.state.result)
	}
	return m.picker.View()
}
