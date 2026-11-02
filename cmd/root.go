// Package cmd wires the command line to the rest of mcinit and owns the
// process exit codes.
package cmd

import (
	"context"
	"errors"
	"os"
	"os/signal"

	"github.com/evanb/mcinit/internal/buildinfo"
	"github.com/evanb/mcinit/internal/ui"
	"github.com/spf13/cobra"
)

// Exit codes. They are distinct so that a script can tell "you asked for
// something impossible" from "the machine or the network got in the way".
const (
	exitOK      = 0
	exitUser    = 1 // bad flags, EULA declined, destination already in use
	exitEnviron = 2 // network, checksum mismatch, Java install failed
)

// failure carries an exit code and the one action the user can take, so every
// error leaves through the same renderer.
type failure struct {
	err  error
	hint string
	code int
}

func (f *failure) Error() string { return f.err.Error() }
func (f *failure) Unwrap() error { return f.err }

// userError is something the user can fix by asking for something else.
func userError(err error, hint string) error {
	return &failure{err: err, hint: hint, code: exitUser}
}

// environError is something outside the user's request: the network, winget,
// a corrupted download.
func environError(err error, hint string) error {
	return &failure{err: err, hint: hint, code: exitEnviron}
}

// quietExit ends the run with no error output. Used when the user cancels.
var quietExit = &failure{err: errors.New("cancelled"), code: exitUser}

var flagNoColor bool

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "mcinit",
		Short: "Set up a Minecraft server on localhost with one command",
		Long: "mcinit creates a Minecraft server to play on localhost:\n" +
			"it picks the software and version, installs Java if missing, downloads\n" +
			"a verified jar and writes eula.txt, server.properties and start.bat.",
		Version:       buildinfo.Version,
		SilenceErrors: true, // printed by Execute, through ui
		SilenceUsage:  true,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			ui.Configure(flagNoColor)
		},
	}

	root.PersistentFlags().BoolVar(&flagNoColor, "no-color", false,
		"no colors or Unicode glyphs (also honours NO_COLOR)")
	root.SetVersionTemplate("mcinit {{.Version}}\n")

	root.AddCommand(newNewCmd())
	root.AddCommand(newBackupCmd())
	root.AddCommand(newRestoreCmd())
	return root
}

// Execute runs the CLI and returns the process exit code.
func Execute() int {
	// Ctrl+C cancels the context, which aborts an in-flight download instead
	// of leaving a half-written file behind.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	err := newRootCmd().ExecuteContext(ctx)
	if err == nil {
		return exitOK
	}

	var f *failure
	if errors.As(err, &f) {
		if f == quietExit {
			ui.Blank()
			return f.code
		}
		ui.Fail(f.err, f.hint)
		return f.code
	}

	// Anything unclassified is treated as the user's: almost always a bad
	// flag, which cobra has already described precisely.
	ui.Fail(err, "")
	return exitUser
}
