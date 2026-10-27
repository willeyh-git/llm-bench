package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

type Options struct {
	BaseURL string
	Out     string
	Model   string
}

type screen int

const (
	screenPicker screen = iota
	screenRun
	screenSummary
)

type Model struct {
	opts     Options
	screen   screen
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
	if opts.Model != "" {
		m.screen = screenRun
		m.run = newRun(opts, opts.Model, 0)
	}
	return m
}

func (m Model) Init() tea.Cmd {
	if m.screen == screenRun {
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
	case screenPicker:
		return m.updatePicker(msg)
	case screenRun:
		return m.updateRun(msg)
	case screenSummary:
		return m.updateSummary(msg)
	}
	return m, nil
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
	case screenRun:
		return m.run.view()
	case screenSummary:
		return summaryView(m.run.state.result)
	}
	return m.picker.View()
}
