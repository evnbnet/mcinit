package backup

import (
	"os"
	"testing"

	"github.com/evanb/mcinit/internal/ui"
)

// TestMain puts the UI in its non-interactive mode so the tests do not fill the
// output with repainting progress bars. This is the same path a user gets when
// piping mcinit's output to a file.
func TestMain(m *testing.M) {
	ui.Configure(true)
	os.Exit(m.Run())
}
