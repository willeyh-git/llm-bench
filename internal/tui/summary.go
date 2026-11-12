package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/willyhetland/llm-bencher/internal/bench"
)

var cardStyle = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(lipgloss.Color("63")).
	Padding(0, 2)

func summaryView(res *bench.Result) string {
	var b strings.Builder
	title := "Summary: " + res.Model
	if res.Tag != "" {
		title += " · " + res.Tag
	}
	b.WriteString(titleStyle.Render(title) + "\n\n")

	for _, k := range []struct{ key, label string }{
		{"4000", "prefill 4K"},
		{"16000", "prefill 16K"},
		{"32000", "prefill 32K"},
		{"64000", "prefill 64K"},
		{"96000", "prefill 96K"},
		{"126000", "prefill 128K"},
	} {
		if reason, ok := res.Skipped[k.key]; ok {
			b.WriteString(fmt.Sprintf("%-12s %s\n", k.label, subtleStyle.Render(reason)))
			continue
		}
		pr, ok := res.Prefill[k.key]
		if !ok {
			continue
		}
		b.WriteString(fmt.Sprintf("%-12s %8.1f tok/s   TTFT %.2fs   prompt %d tokens\n",
			k.label, pr.TPS, pr.TTFTSeconds, pr.PromptTokens))
		b.WriteString(reasonLines(pr.Invalid, pr.Flags))
	}

	d := res.Decode
	b.WriteString(fmt.Sprintf("%-12s %8.1f tok/s median   min %.1f   max %.1f   (%d runs)\n",
		"decode", d.TPSMedian, d.TPSMin, d.TPSMax, len(d.Runs)))
	b.WriteString(reasonLines(d.Invalid, d.Flags))

	for _, k := range []struct{ key, label string }{
		{"4000", "decode 4K"},
		{"16000", "decode 16K"},
		{"32000", "decode 32K"},
		{"64000", "decode 64K"},
		{"96000", "decode 96K"},
		{"126000", "decode 128K"},
	} {
		if reason, ok := res.DecodeAtContextSkipped[k.key]; ok {
			b.WriteString(fmt.Sprintf("%-12s %s\n", k.label, subtleStyle.Render(reason)))
			continue
		}
		dc, ok := res.DecodeAtContext[k.key]
		if !ok {
			continue
		}
		b.WriteString(fmt.Sprintf("%-12s %8.1f tok/s median   min %.1f   max %.1f   (%d runs)\n",
			k.label, dc.TPSMedian, dc.TPSMin, dc.TPSMax, len(dc.Runs)))
		b.WriteString(reasonLines(dc.Invalid, dc.Flags))
	}

	b.WriteString("\n" + subtleStyle.Render("saved: "+res.Path))
	b.WriteString("\n" + subtleStyle.Render("enter back to picker · q quit"))
	return cardStyle.Render(b.String())
}

func reasonLines(invalid, flags []string) string {
	var b strings.Builder
	for _, r := range invalid {
		b.WriteString(errorStyle.Render("  invalid: "+r) + "\n")
	}
	for _, f := range flags {
		b.WriteString(warnStyle.Render("  flag: "+f) + "\n")
	}
	return b.String()
}
