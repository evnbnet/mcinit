package ui

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/evanb/mcinit/internal/buildinfo"
)

func Banner() {
	fmt.Printf("\n%s%s %s\n\n",
		indent,
		boldAccent.Render("mcinit"),
		mutedStyle.Render(buildinfo.Version),
	)
}

func Ok(format string, a ...any) {
	fmt.Printf("%s%s %s\n", indent, successStyle.Render(glyphs.ok), fmt.Sprintf(format, a...))
}

func Warn(format string, a ...any) {
	fmt.Printf("%s%s %s\n", indent, warnStyle.Render(glyphs.warn), fmt.Sprintf(format, a...))
}

func Err(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "%s%s %s\n", indent, dangerStyle.Render(glyphs.err), fmt.Sprintf(format, a...))
}

func Hint(w *os.File, format string, a ...any) {
	fmt.Fprintf(w, "%s  %s\n", indent, mutedStyle.Render(fmt.Sprintf(format, a...)))
}

func Fail(err error, hint string) {
	Err("%s", err)
	if hint != "" {
		Hint(os.Stderr, "%s", hint)
	}
}

func Info(format string, a ...any) {
	fmt.Printf("%s  %s\n", indent, mutedStyle.Render(fmt.Sprintf(format, a...)))
}

func Blank() { fmt.Println() }

type Field struct {
	Label string
	Value string

	Dim bool
}

func Summary(fields []Field) {
	width := 0
	for _, f := range fields {
		if n := lipgloss.Width(f.Label); n > width {
			width = n
		}
	}
	for _, f := range fields {
		label := mutedStyle.Render(f.Label + strings.Repeat(" ", width-lipgloss.Width(f.Label)))
		value := accentStyle.Render(f.Value)
		if f.Dim {
			value = mutedStyle.Render(f.Value)
		}
		fmt.Printf("%s  %s  %s\n", indent, label, value)
	}
}

func Done(commands ...string) {
	head := boldAccent.Render("Done")
	arrow := mutedStyle.Render(glyphs.arrow)
	pad := strings.Repeat(" ", lipgloss.Width("Done")+1)

	for i, cmd := range commands {
		switch i {
		case 0:
			fmt.Printf("\n%s%s %s  %s\n", indent, head, arrow, accentStyle.Render(cmd))
		default:
			fmt.Printf("%s%s   %s\n", indent, pad, accentStyle.Render(cmd))
		}
	}
	fmt.Println()
}
