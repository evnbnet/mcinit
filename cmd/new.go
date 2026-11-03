package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/evanb/mcinit/internal/config"
	"github.com/evanb/mcinit/internal/javart"
	"github.com/evanb/mcinit/internal/mcver"
	"github.com/evanb/mcinit/internal/provider"
	"github.com/evanb/mcinit/internal/server"
	"github.com/evanb/mcinit/internal/ui"
	"github.com/evanb/mcinit/internal/wizard"
	"github.com/spf13/cobra"
)

type newOptions struct {
	software   string
	version    string
	ram        int
	dir        string
	acceptEULA bool
	yes        bool
}

func newNewCmd() *cobra.Command {
	var opts newOptions

	cmd := &cobra.Command{
		Use:   "new [name]",
		Short: "Create a new server",
		Long: "Create a Minecraft server that is ready to start.\n\n" +
			"With no flags it asks for everything interactively. With flags it can\n" +
			"be fully automated; --yes requires that nothing is left to ask.",
		Example: "  mcinit new my-server\n" +
			"  mcinit new my-server --software purpur --mc 26.3 --ram 8\n" +
			"  mcinit new auto --mc latest --accept-eula --yes",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := ""
			if len(args) == 1 {
				name = args[0]
			}
			return runNew(cmd, name, opts)
		},
	}

	f := cmd.Flags()
	f.StringVar(&opts.software, "software", "", "paper or purpur")
	f.StringVar(&opts.version, "mc", "", "Minecraft version, or latest")
	f.IntVar(&opts.ram, "ram", 0, "RAM in GB")
	f.StringVar(&opts.dir, "dir", "", "destination folder (defaults to ./<name>)")
	f.BoolVar(&opts.acceptEULA, "accept-eula", false, "accept the Minecraft EULA (https://aka.ms/MinecraftEULA)")
	f.BoolVarP(&opts.yes, "yes", "y", false, "ask nothing; fail if anything is missing")

	return cmd
}

func runNew(cmd *cobra.Command, name string, opts newOptions) error {
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	ui.Banner()

	cfg, provided, err := resolveFlags(name, opts, cmd)
	if err != nil {
		return err
	}

	if !opts.yes {
		cfg, err = wizard.Run(ctx, cfg, provided)
		if err != nil {
			return classifyWizardError(err)
		}
		ui.Blank()
	}
	cfg.MOTD = fmt.Sprintf("%s - created with mcinit", cfg.Name)

	if cfg.Dir == "" {
		cfg.Dir = filepath.Join(".", cfg.Name)
	}
	if err := cfg.Validate(); err != nil {
		return userError(err, "")
	}

	dir, err := cfg.AbsDir()
	if err != nil {
		return userError(err, "")
	}
	if err := server.CheckTarget(dir); err != nil {
		return userError(err, "Pick another name, or pass --dir with an empty folder.")
	}

	if err := ensureEULA(&cfg, opts); err != nil {
		return err
	}

	art, mc, err := resolveBuild(ctx, &cfg)
	if err != nil {
		return environError(err, "Check your connection, or try another version with --mc.")
	}
	ui.Ok("%s %s build %s", cfg.Software.Display(), cfg.Version, art.Build)
	if art.Experimental() {
		ui.Warn("This is an experimental build (%s)", strings.ToLower(art.Channel))
	}

	runtime, err := ensureJava(ctx, mc)
	if err != nil {
		return environError(err, fmt.Sprintf(
			"Install Java %d yourself from https://adoptium.net and run mcinit again.",
			mc.RequiredJava()))
	}

	result, err := server.Create(ctx, server.Plan{
		Config:   cfg,
		Dir:      dir,
		Artifact: art,
		Java:     runtime,
	})
	if err != nil {
		return environError(err, "Nothing was written. Try again.")
	}
	ui.Ok("%s verified (%s)", art.Filename, art.Algo)
	ui.Ok("eula.txt · server.properties · %s", server.StartScript())

	printSummary(cfg, art, runtime, result)
	return nil
}

func resolveFlags(name string, opts newOptions, cmd *cobra.Command) (config.Config, wizard.Provided, error) {
	cfg := config.Default()
	cfg.Name = name

	changed := cmd.Flags().Changed
	provided := wizard.Provided{
		Software: changed("software"),
		Version:  changed("mc"),
		RAM:      changed("ram"),
		Dir:      changed("dir"),
	}

	if provided.Software {
		software, err := config.ParseSoftware(opts.software)
		if err != nil {
			return cfg, provided, userError(err, "")
		}
		cfg.Software = software
	}
	if provided.Version {
		cfg.Version = strings.TrimSpace(opts.version)
	}
	if provided.RAM {
		cfg.RAM = opts.ram
	}
	if provided.Dir {
		cfg.Dir = opts.dir
	}

	if opts.yes {
		if cfg.Name == "" {
			return cfg, provided, userError(
				errors.New("the server name is missing"),
				"Pass it as an argument: mcinit new my-server --yes")
		}
		if cfg.Software == "" {
			cfg.Software = config.Paper
		}
		return cfg, provided, nil
	}

	if cfg.Name == "" {
		askedName, err := wizard.AskName()
		if err != nil {
			return cfg, provided, classifyWizardError(err)
		}
		cfg.Name = askedName
	}
	return cfg, provided, nil
}

func ensureEULA(cfg *config.Config, opts newOptions) error {
	switch {
	case opts.acceptEULA:
		ui.Ok("Minecraft EULA accepted with --accept-eula")
	case opts.yes:
		return userError(
			errors.New("the Minecraft EULA has not been accepted"),
			"Add --accept-eula if you agree to https://aka.ms/MinecraftEULA")
	default:
		if err := wizard.ConfirmEULA(); err != nil {
			return classifyWizardError(err)
		}
		ui.Ok("Minecraft EULA accepted")
	}

	cfg.AcceptedEULA = true
	return nil
}

func resolveBuild(ctx context.Context, cfg *config.Config) (provider.Artifact, mcver.Version, error) {
	p, err := provider.For(cfg.Software)
	if err != nil {
		return provider.Artifact{}, mcver.Version{}, err
	}

	label := fmt.Sprintf("Looking up the latest %s build", cfg.Software.Display())
	var art provider.Artifact

	err = ui.Spin(label, func() error {
		if strings.EqualFold(cfg.Version, config.Latest) {
			latest, err := provider.LatestStable(ctx, p)
			if err != nil {
				return err
			}
			cfg.Version = latest
		}
		var err error
		art, err = p.Resolve(ctx, cfg.Version)
		return err
	})
	if err != nil {
		return provider.Artifact{}, mcver.Version{}, err
	}

	mc, err := mcver.Parse(cfg.Version)
	if err != nil {
		return provider.Artifact{}, mcver.Version{}, err
	}
	return art, mc, nil
}

func ensureJava(ctx context.Context, mc mcver.Version) (javart.Runtime, error) {
	need := mc.RequiredJava()

	if rt, ok := javart.Detect(need); ok {
		ui.Ok("Java %s found", rt.Version)
		ui.Info("%s", rt.Path)
		return rt, nil
	}

	ui.Warn("Minecraft %s needs Java %d and I could not find it", mc.Raw, need)
	rt, _, err := javart.Ensure(ctx, need)
	if err != nil {
		return javart.Runtime{}, err
	}
	ui.Ok("Java %s installed", rt.Version)
	ui.Info("%s", rt.Path)
	return rt, nil
}

func printSummary(cfg config.Config, art provider.Artifact, rt javart.Runtime, result server.Result) {
	ui.Blank()
	ui.Summary([]ui.Field{
		{Label: "Folder", Value: result.Dir},
		{Label: "Software", Value: fmt.Sprintf("%s %s (build %s)", cfg.Software.Display(), cfg.Version, art.Build)},
		{Label: "Java", Value: rt.Version},
		{Label: "RAM", Value: fmt.Sprintf("%d GB", cfg.RAM)},
		{Label: "Defaults", Value: fmt.Sprintf("%s · %s · %d slots · port %d · online-mode %v",
			cfg.Gamemode, cfg.Difficulty, cfg.MaxPlayers, cfg.Port, cfg.OnlineMode), Dim: true},
	})

	ui.Done("cd "+relativeTo(result.Dir), startCommand())
}

func relativeTo(dir string) string {
	wd, err := os.Getwd()
	if err != nil {
		return dir
	}
	rel, err := filepath.Rel(wd, dir)
	if err != nil || strings.HasPrefix(rel, "..") {
		return dir
	}
	return rel
}

func classifyWizardError(err error) error {
	switch {
	case errors.Is(err, wizard.ErrAborted):
		return quietExit
	case errors.Is(err, wizard.ErrEULARejected):
		return userError(err,
			"Without accepting the EULA the server cannot start, so nothing was created.")
	}
	return environError(err, "")
}
