package ui

import (
	"os"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-isatty"
	"github.com/muesli/termenv"
)

const (
	colAccent  = lipgloss.Color("#7D56F4")
	colMuted   = lipgloss.Color("#6C6C6C")
	colSuccess = lipgloss.Color("#04B575")
	colWarn    = lipgloss.Color("#FFB454")
	colDanger  = lipgloss.Color("#FF5F87")
)

const indent = "  "

var (
	accentStyle  = lipgloss.NewStyle().Foreground(colAccent)
	boldAccent   = lipgloss.NewStyle().Foreground(colAccent).Bold(true)
	mutedStyle   = lipgloss.NewStyle().Foreground(colMuted)
	successStyle = lipgloss.NewStyle().Foreground(colSuccess)
	warnStyle    = lipgloss.NewStyle().Foreground(colWarn)
	dangerStyle  = lipgloss.NewStyle().Foreground(colDanger)
)

type glyphSet struct {
	ok, warn, err, arrow string
	spinner              []string
	barFull, barEmpty    string
}

var (
	unicodeGlyphs = glyphSet{
		ok: "✓", warn: "!", err: "✗", arrow: "→",
		spinner: []string{"⣾", "⣽", "⣻", "⢿", "⡿", "⣟", "⣯", "⣷"},
		barFull: "█", barEmpty: "░",
	}
	asciiGlyphs = glyphSet{
		ok: "+", warn: "!", err: "x", arrow: ">",
		spinner: []string{"|", "/", "-", "\\"},
		barFull: "#", barEmpty: ".",
	}
)

var (
	glyphs      = unicodeGlyphs
	interactive = true
)

func Configure(noColor bool) {
	tty := isatty.IsTerminal(os.Stdout.Fd()) || isatty.IsCygwinTerminal(os.Stdout.Fd())
	interactive = tty

	plain := noColor || os.Getenv("NO_COLOR") != "" || !tty
	if plain {
		lipgloss.SetColorProfile(termenv.Ascii)
		glyphs = asciiGlyphs
	}
}

func Interactive() bool { return interactive }

func FormTheme() *huh.Theme {
	t := huh.ThemeBase()

	t.Focused.Base = t.Focused.Base.BorderForeground(colAccent)
	t.Focused.Title = t.Focused.Title.Foreground(colAccent).Bold(true)
	t.Focused.Description = t.Focused.Description.Foreground(colMuted)
	t.Focused.SelectSelector = t.Focused.SelectSelector.Foreground(colAccent)
	t.Focused.SelectedOption = t.Focused.SelectedOption.Foreground(colAccent)
	t.Focused.SelectedPrefix = t.Focused.SelectedPrefix.Foreground(colAccent)
	t.Focused.UnselectedOption = t.Focused.UnselectedOption.Foreground(lipgloss.NoColor{})
	t.Focused.TextInput.Prompt = t.Focused.TextInput.Prompt.Foreground(colAccent)
	t.Focused.TextInput.Cursor = t.Focused.TextInput.Cursor.Foreground(colAccent)
	t.Focused.TextInput.Placeholder = t.Focused.TextInput.Placeholder.Foreground(colMuted)
	t.Focused.ErrorIndicator = t.Focused.ErrorIndicator.Foreground(colDanger)
	t.Focused.ErrorMessage = t.Focused.ErrorMessage.Foreground(colDanger)
	t.Focused.FocusedButton = t.Focused.FocusedButton.Background(colAccent).Foreground(lipgloss.Color("#FFFFFF"))

	t.Blurred.Title = t.Blurred.Title.Foreground(colMuted)
	t.Blurred.Description = t.Blurred.Description.Foreground(colMuted)

	t.Help.ShortKey = t.Help.ShortKey.Foreground(colMuted)
	t.Help.ShortDesc = t.Help.ShortDesc.Foreground(colMuted)
	t.Help.ShortSeparator = t.Help.ShortSeparator.Foreground(colMuted)

	return t
}
