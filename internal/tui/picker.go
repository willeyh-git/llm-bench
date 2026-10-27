package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/willyhetland/llm-bencher/internal/server"
)

const listTimeout = 10 * time.Second

var (
	titleStyle  = lipgloss.NewStyle().Bold(true)
	subtleStyle = lipgloss.NewStyle().Faint(true)
	errorStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
	loadedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	selectStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Bold(true)
)

type modelsLoadedMsg struct {
	models []server.ModelInfo
}

type modelsFailedMsg struct {
	err error
}

type picker struct {
	base    string
	models  []server.ModelInfo
	cursor  int
	loading bool
	err     error
	chosen  string
}

func newPicker(base string) picker {
	return picker{base: base, loading: true}
}

func (p picker) contextLength(id string) int {
	for _, m := range p.models {
		if m.ID == id {
			return m.ContextLength
		}
	}
	return 0
}

func (p picker) Init() tea.Cmd {
	return p.fetch()
}

func (p picker) fetch() tea.Cmd {
	base := p.base
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), listTimeout)
		defer cancel()
		models, err := server.ListModels(ctx, base)
		if err != nil {
			return modelsFailedMsg{err: err}
		}
		return modelsLoadedMsg{models: models}
	}
}

func (p picker) Update(msg tea.Msg) (picker, tea.Cmd) {
	switch msg := msg.(type) {
	case modelsLoadedMsg:
		p.loading = false
		p.err = nil
		p.models = msg.models
		p.cursor = 0
	case modelsFailedMsg:
		p.loading = false
		p.err = msg.err
		p.models = nil
	case tea.KeyMsg:
		return p.handleKey(msg)
	}
	return p, nil
}

func (p picker) handleKey(msg tea.KeyMsg) (picker, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if p.cursor > 0 {
			p.cursor--
		}
	case "down", "j":
		if p.cursor < len(p.models)-1 {
			p.cursor++
		}
	case "enter":
		if p.loading || p.err != nil || len(p.models) == 0 {
			return p, nil
		}
		p.chosen = p.models[p.cursor].ID
		return p, nil
	case "r":
		if p.err == nil {
			return p, nil
		}
		p.loading = true
		p.err = nil
		return p, p.fetch()
	case "esc", "q", "ctrl+c":
		return p, tea.Quit
	}
	return p, nil
}

func (p picker) View() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Select a model") + "\n")
	b.WriteString(subtleStyle.Render("server: "+p.base) + "\n\n")

	switch {
	case p.loading:
		b.WriteString("Loading models...\n")
	case p.err != nil:
		b.WriteString(errorStyle.Render("Could not load models") + "\n")
		b.WriteString(p.err.Error() + "\n\n")
		b.WriteString(subtleStyle.Render("r retry · q quit"))
		return b.String()
	default:
		width := 0
		for _, m := range p.models {
			if len(m.ID) > width {
				width = len(m.ID)
			}
		}
		for i, m := range p.models {
			cursor := "  "
			name := fmt.Sprintf("%-*s", width, m.ID)
			if i == p.cursor {
				cursor = "> "
				name = selectStyle.Render(name)
			}
			status := m.Status
			if status == "loaded" {
				status = loadedStyle.Render(status)
			} else {
				status = subtleStyle.Render(status)
			}
			b.WriteString(fmt.Sprintf("%s%s  %s\n", cursor, name, status))
		}
		b.WriteString("\n")
	}
	b.WriteString(subtleStyle.Render("↑/↓ or j/k move · enter select · esc/q quit"))
	return b.String()
}
