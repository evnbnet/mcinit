package wizard

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/evanb/mcinit/internal/config"
	"github.com/evanb/mcinit/internal/mcver"
	"github.com/evanb/mcinit/internal/provider"
	"github.com/evanb/mcinit/internal/ui"
)

var ErrAborted = errors.New("cancelled")

var ErrEULARejected = errors.New("you did not accept the Minecraft EULA")

type Provided struct {
	Software bool
	Version  bool
	RAM      bool
	Dir      bool
}

const versionChoices = 8

const otherSentinel = "\x00other"

func Run(ctx context.Context, cfg config.Config, provided Provided) (config.Config, error) {
	if !provided.Software {
		if err := askSoftware(&cfg); err != nil {
			return cfg, err
		}
	}
	if !provided.Version {
		if err := askVersion(ctx, &cfg); err != nil {
			return cfg, err
		}
	}
	if !provided.RAM {
		if err := askRAM(&cfg); err != nil {
			return cfg, err
		}
	}
	if !provided.Dir {
		if err := askDir(&cfg); err != nil {
			return cfg, err
		}
	}
	return cfg, nil
}

func askSoftware(cfg *config.Config) error {
	choice := string(config.Paper)
	if cfg.Software != "" {
		choice = string(cfg.Software)
	}

	form := newForm(huh.NewSelect[string]().
		Title("Server software").
		Options(
			huh.NewOption("Paper", string(config.Paper)),
			huh.NewOption("Purpur", string(config.Purpur)),
		).
		Value(&choice))

	if err := run(form); err != nil {
		return err
	}
	software, err := config.ParseSoftware(choice)
	if err != nil {
		return err
	}
	cfg.Software = software
	return nil
}

func askVersion(ctx context.Context, cfg *config.Config) error {
	p, err := provider.For(cfg.Software)
	if err != nil {
		return err
	}

	var versions []mcver.Version
	label := fmt.Sprintf("Looking up %s versions", cfg.Software.Display())
	if err := ui.Spin(label, func() error {
		versions, err = p.Versions(ctx)
		return err
	}); err != nil {
		return err
	}
	if len(versions) == 0 {
		return fmt.Errorf("%s has no stable versions available", cfg.Software.Display())
	}

	if len(versions) > versionChoices {
		versions = versions[:versionChoices]
	}

	choice := versions[0].Raw
	form := versionForm(versions, &choice)
	if err := run(form); err != nil {
		return err
	}

	if choice != otherSentinel {
		cfg.Version = choice
		return nil
	}

	typed := ""
	form = newForm(huh.NewInput().
		Title("Minecraft version").
		Description("For example 1.21.4 or 26.3.").
		Prompt("› ").
		Value(&typed).
		Validate(func(s string) error {
			_, err := mcver.Parse(s)
			return err
		}))
	if err := run(form); err != nil {
		return err
	}
	cfg.Version = strings.TrimSpace(typed)
	return nil
}

func askRAM(cfg *config.Config) error {
	choice := strconv.Itoa(cfg.RAM)
	form := newForm(huh.NewSelect[string]().
		Title("RAM").
		Description("How much memory to reserve for the server.").
		Options(
			huh.NewOption("2 GB    a few players, no plugins", "2"),
			huh.NewOption("4 GB    recommended", "4"),
			huh.NewOption("6 GB", "6"),
			huh.NewOption("8 GB    lots of plugins or players", "8"),
			huh.NewOption("another…", otherSentinel),
		).
		Value(&choice))
	if err := run(form); err != nil {
		return err
	}

	if choice != otherSentinel {
		ram, err := strconv.Atoi(choice)
		if err != nil {
			return err
		}
		cfg.RAM = ram
		return nil
	}

	typed := strconv.Itoa(cfg.RAM)
	form = newForm(huh.NewInput().
		Title("RAM in GB").
		Description(fmt.Sprintf("A number between %d and %d.", config.MinRAM, config.MaxRAM)).
		Prompt("› ").
		Value(&typed).
		Validate(validateRAM))
	if err := run(form); err != nil {
		return err
	}
	ram, err := strconv.Atoi(strings.TrimSpace(typed))
	if err != nil {
		return err
	}
	cfg.RAM = ram
	return nil
}

func validateRAM(s string) error {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return fmt.Errorf("type a number of GB")
	}
	if n < config.MinRAM || n > config.MaxRAM {
		return fmt.Errorf("between %d and %d GB", config.MinRAM, config.MaxRAM)
	}
	return nil
}

func askDir(cfg *config.Config) error {
	dir := cfg.Dir
	if dir == "" {
		dir = "./" + cfg.Name
	}

	form := newForm(huh.NewInput().
		Title("Folder").
		Description("Where the server gets created. It must be empty or not exist yet.").
		Prompt("› ").
		Value(&dir).
		Validate(func(s string) error {
			if strings.TrimSpace(s) == "" {
				return fmt.Errorf("a folder is required")
			}
			return nil
		}))
	if err := run(form); err != nil {
		return err
	}
	cfg.Dir = strings.TrimSpace(dir)
	return nil
}

func ConfirmEULA() error {
	accepted := false
	form := newForm(huh.NewConfirm().
		Title("Minecraft EULA").
		Description("A Minecraft server only starts if you accept Mojang's terms.\nhttps://aka.ms/MinecraftEULA").
		Affirmative("I accept").
		Negative("No").
		Value(&accepted))

	if err := run(form); err != nil {
		return err
	}
	if !accepted {
		return ErrEULARejected
	}
	return nil
}

const formWidth = 64

func newForm(field huh.Field) *huh.Form {

	return huh.NewForm(huh.NewGroup(field)).
		WithWidth(formWidth).
		WithShowHelp(true).
		WithTheme(ui.FormTheme())
}

func run(form *huh.Form) error {
	if err := form.Run(); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return ErrAborted
		}
		return err
	}
	return nil
}

func AskName() (string, error) {
	name := ""
	form := newForm(huh.NewInput().
		Title("Server name").
		Description("Used for the folder and the MOTD.").
		Prompt("› ").
		Placeholder("my-server").
		Value(&name).
		Validate(validateName))

	if err := run(form); err != nil {
		return "", err
	}
	return strings.TrimSpace(name), nil
}

func validateName(s string) error {
	s = strings.TrimSpace(s)
	if s == "" {
		return fmt.Errorf("a name is required")
	}
	if strings.ContainsAny(s, forbiddenInName) {
		return fmt.Errorf("cannot contain any of these characters: %s", forbiddenInName)
	}
	return nil
}

const forbiddenInName = `\/:*?"<>|`

func versionOptions(versions []mcver.Version) []huh.Option[string] {
	options := make([]huh.Option[string], 0, len(versions)+1)
	for i, v := range versions {
		label := v.Raw
		if i == 0 {
			label += "   newest"
		}
		options = append(options, huh.NewOption(label, v.Raw))
	}
	return append(options, huh.NewOption("another…", otherSentinel))
}

func versionForm(versions []mcver.Version, choice *string) *huh.Form {
	description := ""
	if len(versions) > 0 {
		description = fmt.Sprintf("Needs Java %d.", versions[0].RequiredJava())
	}

	return newForm(huh.NewSelect[string]().
		Title("Minecraft version").
		Description(description).
		Options(versionOptions(versions)...).
		Value(choice))
}
