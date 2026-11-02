package cmd

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/evanb/mcinit/internal/backup"
	"github.com/evanb/mcinit/internal/server"
	"github.com/evanb/mcinit/internal/ui"
	"github.com/evanb/mcinit/internal/wizard"
	"github.com/spf13/cobra"
)

type restoreOptions struct {
	dir    string
	pick   string
	latest bool
	yes    bool
}

func newRestoreCmd() *cobra.Command {
	var opts restoreOptions

	cmd := &cobra.Command{
		Use:   "restore",
		Short: "Put a saved world back",
		Long: "Replace the current world and configuration with a saved backup.\n\n" +
			"The state being replaced is saved first, so restoring the wrong\n" +
			"backup is always recoverable.",
		Example: "  mcinit restore\n" +
			"  mcinit restore --dir my-server\n" +
			"  mcinit restore --latest --yes",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRestore(opts)
		},
	}

	f := cmd.Flags()
	f.StringVar(&opts.dir, "dir", ".", "the server folder")
	f.StringVar(&opts.pick, "backup", "", "restore this backup by name instead of choosing from a list")
	f.BoolVar(&opts.latest, "latest", false, "restore the newest backup without choosing")
	f.BoolVarP(&opts.yes, "yes", "y", false, "skip the typed confirmation")

	return cmd
}

func chooseArchive(archives []backup.Archive, opts restoreOptions) (backup.Archive, error) {
	switch {
	case opts.pick != "":
		for _, a := range archives {
			if strings.EqualFold(a.Name, opts.pick) || strings.EqualFold(filepath.Base(a.Path), opts.pick) {
				return a, nil
			}
		}
		return backup.Archive{}, userError(
			fmt.Errorf("there is no backup called %q", opts.pick),
			"Run mcinit restore with no flags to see the list.")
	case opts.latest:
		return archives[0], nil
	}

	chosen, err := wizard.PickBackup(archives)
	if err != nil {
		return backup.Archive{}, classifyWizardError(err)
	}
	return chosen, nil
}

func runRestore(opts restoreOptions) error {
	dir := opts.dir
	ui.Banner()

	info, err := server.Inspect(dir)
	if err != nil {
		return userError(err, "Run this from inside a server folder, or pass --dir.")
	}

	if running, why := info.IsRunning(); running {
		return userError(
			fmt.Errorf("the server is running (%s)", why),
			"Stop it first. A world that Minecraft has open cannot be replaced.")
	}

	archives, err := backup.List(info)
	if err != nil {
		return environError(err, "")
	}
	if len(archives) == 0 {
		return userError(
			errors.New("there are no backups for this server"),
			"Create one first with: mcinit backup")
	}

	chosen, err := chooseArchive(archives, opts)
	if err != nil {
		return err
	}

	if opts.yes {

		ui.Warn("Replacing the current world (--yes)")
	} else if err := wizard.ConfirmByName(info.Name); err != nil {
		return classifyWizardError(err)
	}

	safety, err := backup.Restore(info, chosen)
	if err != nil {
		hint := ""
		if safety.Path != "" {
			hint = "The previous state is safe in " + relativeTo(safety.Path)
		}
		return environError(err, hint)
	}

	ui.Ok("Previous state saved as %s", relativeTo(safety.Path))
	ui.Ok("Restored %s", chosen.Created.Format("2006-01-02 15:04"))

	ui.Blank()
	ui.Summary([]ui.Field{
		{Label: "Server", Value: info.Name},
		{Label: "Restored", Value: chosen.Manifest.Describe()},
		{Label: "Undo", Value: "mcinit restore, and pick the pre-restore copy", Dim: true},
	})
	ui.Done("cd "+relativeTo(info.Dir), `.\start.bat`)
	return nil
}
