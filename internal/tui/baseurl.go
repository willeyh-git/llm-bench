package tui

import (
	"errors"
	"net/url"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/willyhetland/llm-bencher/internal/config"
)

var errBaseURL = errors.New("enter a full http:// or https:// URL, for example http://host:8080/v1")

type baseURLPrompt struct {
	input  textinput.Model
	chosen string
	err    error
	quit   bool
}

func newBaseURLPrompt() baseURLPrompt {
	ti := textinput.New()
	ti.Placeholder = "http://host:8080/v1"
	ti.Focus()
	ti.CharLimit = 256
	ti.Width = 60
	return baseURLPrompt{input: ti}
}

func validBaseURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return errBaseURL
	}
	return nil
}

func (p baseURLPrompt) Init() tea.Cmd {
	return textinput.Blink
}

func (p baseURLPrompt) Update(msg tea.Msg) (baseURLPrompt, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "esc", "ctrl+c":
			p.quit = true
			return p, nil
		case "enter":
			raw := strings.TrimSpace(p.input.Value())
			if err := validBaseURL(raw); err != nil {
				p.err = err
				return p, nil
			}
			p.chosen = strings.TrimRight(raw, "/")
			p.err = nil
			return p, nil
		}
	}
	var cmd tea.Cmd
	p.input, cmd = p.input.Update(msg)
	return p, cmd
}

func (p baseURLPrompt) View() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("API base URL") + "\n")
	b.WriteString(subtleStyle.Render("No base URL is saved yet. It will be written to "+config.FileName+".") + "\n\n")
	b.WriteString(p.input.View() + "\n")
	if p.err != nil {
		b.WriteString("\n" + errorStyle.Render(p.err.Error()) + "\n")
	}
	b.WriteString("\n" + subtleStyle.Render("enter save · esc quit"))
	return b.String()
}
