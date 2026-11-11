package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func typeString(p baseURLPrompt, s string) baseURLPrompt {
	for _, r := range s {
		p, _ = p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	return p
}

func TestBaseURLPromptRejectsInvalidURL(t *testing.T) {
	p := typeString(newBaseURLPrompt(), "gaming:8080/v1")
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if p.chosen != "" {
		t.Fatalf("chosen = %q, want empty for a URL without a scheme", p.chosen)
	}
	if p.err == nil {
		t.Fatal("expected a validation error")
	}
}

func TestBaseURLPromptAcceptsAndTrimsURL(t *testing.T) {
	p := typeString(newBaseURLPrompt(), "http://gaming:8080/v1/")
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if p.chosen != "http://gaming:8080/v1" {
		t.Fatalf("chosen = %q, want trailing slash removed", p.chosen)
	}
	if p.err != nil {
		t.Fatalf("unexpected error: %v", p.err)
	}
}

func TestBaseURLPromptEscQuits(t *testing.T) {
	p, _ := newBaseURLPrompt().Update(tea.KeyMsg{Type: tea.KeyEsc})
	if !p.quit {
		t.Fatal("esc should set quit")
	}
}

func TestNewModelAsksForURLWhenNoneSaved(t *testing.T) {
	m := NewModel(Options{})
	if m.screen != screenBaseURL {
		t.Fatalf("screen = %v, want base URL prompt", m.screen)
	}
}

func TestNewModelSkipsURLPromptWhenConfigured(t *testing.T) {
	m := NewModel(Options{BaseURL: "http://gaming:8080/v1"})
	if m.screen != screenPicker {
		t.Fatalf("screen = %v, want picker", m.screen)
	}
}
