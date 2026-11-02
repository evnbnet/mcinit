package cmd

import (
	"fmt"

	"github.com/evanb/mcinit/internal/backup"
	"github.com/evanb/mcinit/internal/server"
	"github.com/evanb/mcinit/internal/ui"
	"github.com/spf13/cobra"
)

func newBackupCmd() *cobra.Command {
	var (
		dir   string
		force bool
	)

	cmd := &cobra.Command{
		Use:   "backup",
		Short: "Save the world and configuration into a zip",
		Long: "Save the server's worlds and configuration into backups/.\n\n" +
			"The server jar is left out: mcinit can download that again, while\n" +
			"the world is the only thing in the folder that cannot be replaced.",
		Example: "  mcinit backup\n" +
			"  mcinit backup --dir my-server",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBackup(dir, force)
		},
	}

	cmd.Flags().StringVar(&dir, "dir", ".", "the server folder")
	cmd.Flags().BoolVar(&force, "force", false,
		"back up even while the server is running (the world may be captured mid-write)")

	return cmd
}

func runBackup(dir string, force bool) error {
	ui.Banner()

	info, err := server.Inspect(dir)
	if err != nil {
		return userError(err, "Run this from inside a server folder, or pass --dir.")
	}

	if running, why := info.IsRunning(); running {
		if !force {
			return userError(
				fmt.Errorf("the server is running (%s)", why),
				"Stop it first so the world is saved consistently, or pass --force.")
		}
		// Worth saying out loud: a world copied while Minecraft is writing to
		// it can contain half-written region files.
		ui.Warn("The server is running (%s); this backup may be inconsistent", why)
	}

	result, err := backup.Create(info, backup.ReasonManual)
	if err != nil {
		return environError(err, "")
	}

	ui.Ok("%s", result.Manifest.Describe())
	ui.Ok("%s  (%s)", relativeTo(result.Path), backup.HumanSize(result.Size))

	ui.Blank()
	ui.Summary([]ui.Field{
		{Label: "Server", Value: info.Name},
		{Label: "Worlds", Value: fmt.Sprintf("%v", result.Manifest.Worlds)},
		{Label: "Files", Value: fmt.Sprintf("%d  (%s uncompressed)",
			result.Manifest.Files, backup.HumanSize(result.Manifest.Bytes))},
		{Label: "Skipped", Value: "server jar, libraries, cache, logs", Dim: true},
	})
	ui.Done("mcinit restore")
	return nil
}
