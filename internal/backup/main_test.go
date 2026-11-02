package backup

import (
	"os"
	"testing"

	"github.com/evanb/mcinit/internal/ui"
)

func TestMain(m *testing.M) {
	ui.Configure(true)
	os.Exit(m.Run())
}
