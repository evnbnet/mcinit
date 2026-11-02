package wizard

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/evanb/mcinit/internal/backup"
)

// PickBackup asks which backup to restore.
//
// Like every other list in the wizard it passes no Height: huh scrolls a select
// only when it is given one, and a backup sliding out of view while you choose
// which one to restore is exactly the wrong moment for that.
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

// ConfirmByName makes the user type the server's name before something
// destructive happens.
//
// This is deliberately more work than a yes/no: a stray enter on a confirm
// dialog should not be able to replace a world, and typing the name proves the
// user knows which server they are pointing at.
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
