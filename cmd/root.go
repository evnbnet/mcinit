package cmd

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"runtime"

	"github.com/evanb/mcinit/internal/buildinfo"
	"github.com/evanb/mcinit/internal/server"
	"github.com/evanb/mcinit/internal/ui"
	"github.com/spf13/cobra"
)

const (
	exitOK      = 0
	exitUser    = 1
	exitEnviron = 2
)

type failure struct {
	err  error
	hint string
	code int
}

func (f *failure) Error() string { return f.err.Error() }
func (f *failure) Unwrap() error { return f.err }

func userError(err error, hint string) error {
	return &failure{err: err, hint: hint, code: exitUser}
}

func environError(err error, hint string) error {
	return &failure{err: err, hint: hint, code: exitEnviron}
}

var quietExit = &failure{err: errors.New("cancelled"), code: exitUser}

var flagNoColor bool

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "mcinit",
		Short: "Set up a Minecraft server on localhost with one command",
		Long: "mcinit creates a Minecraft server to play on localhost:\n" +
			"it picks the software and version, installs Java if missing, downloads\n" +
			"a verified jar and writes eula.txt, server.properties and a start script.",
		Version:       buildinfo.Version,
		SilenceErrors: true,
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

func Execute() int {

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

	ui.Fail(err, "")
	return exitUser
}

func startCommand() string {
	if runtime.GOOS == "windows" {
		return `.\` + server.StartScript()
	}
	return "./" + server.StartScript()
}
