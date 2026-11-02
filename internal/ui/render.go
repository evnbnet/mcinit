package ui

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/evanb/mcinit/internal/buildinfo"
)

// Banner prints the one-line header.
func Banner() {
	fmt.Printf("\n%s%s %s\n\n",
		indent,
		boldAccent.Render("mcinit"),
		mutedStyle.Render(buildinfo.Version),
	)
}

// Ok marks a finished step.
func Ok(format string, a ...any) {
	fmt.Printf("%s%s %s\n", indent, successStyle.Render(glyphs.ok), fmt.Sprintf(format, a...))
}

// Warn marks something that worked but is worth knowing about.
func Warn(format string, a ...any) {
	fmt.Printf("%s%s %s\n", indent, warnStyle.Render(glyphs.warn), fmt.Sprintf(format, a...))
}

// Err reports a failure. It writes to stderr so that piping stdout still shows
// the problem.
func Err(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "%s%s %s\n", indent, dangerStyle.Render(glyphs.err), fmt.Sprintf(format, a...))
}

// Hint adds an indented, dimmed follow-up line, normally the action the user
// should take next. It follows Err on stderr and Warn on stdout.
func Hint(w *os.File, format string, a ...any) {
	fmt.Fprintf(w, "%s  %s\n", indent, mutedStyle.Render(fmt.Sprintf(format, a...)))
}

// Fail prints an error and the single thing the user can do about it.
func Fail(err error, hint string) {
	Err("%s", err)
	if hint != "" {
		Hint(os.Stderr, "%s", hint)
	}
}

// Info prints a dimmed, unmarked line.
func Info(format string, a ...any) {
	fmt.Printf("%s  %s\n", indent, mutedStyle.Render(fmt.Sprintf(format, a...)))
}

// Blank prints an empty line.
func Blank() { fmt.Println() }

// Field is one label/value pair in the final summary.
type Field struct {
	Label string
	Value string
	// Dim marks a value the user did not choose — a default worth showing but
	// not worth highlighting.
	Dim bool
}

// Summary prints aligned label/value pairs.
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

// Done prints the closing block: a highlighted "Done" and the commands to run,
// aligned under it.
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
