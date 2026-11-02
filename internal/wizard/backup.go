package wizard

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/evanb/mcinit/internal/backup"
)

func PickBackup(archives []backup.Archive) (backup.Archive, error) {
	if len(archives) == 0 {
		return backup.Archive{}, fmt.Errorf("there are no backups yet")
	}

	options := make([]huh.Option[int], 0, len(archives))
	for i, a := range archives {
		options = append(options, huh.NewOption(a.Label(), i))
	}

	chosen := 0
	form := newForm(huh.NewSelect[int]().
		Title("Which backup?").
		Description("Newest first.").
		Options(options...).
		Value(&chosen))

	if err := run(form); err != nil {
		return backup.Archive{}, err
	}
	return archives[chosen], nil
}

func ConfirmByName(name string) error {
	typed := ""
	form := newForm(huh.NewInput().
		Title("Replace the current world?").
		Description(fmt.Sprintf(
			"The world and configuration will be overwritten.\nType %s to confirm.", name)).
		Prompt("› ").
		Placeholder(name).
		Value(&typed).
		Validate(func(s string) error {
			if strings.TrimSpace(s) != name {
				return fmt.Errorf("type %s exactly, or press esc to cancel", name)
			}
			return nil
		}))

	return run(form)
}
