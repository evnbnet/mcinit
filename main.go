// Command mcinit creates a ready-to-run Minecraft server on localhost.
package main

import (
	"os"

	"github.com/evanb/mcinit/cmd"
)

func main() {
	os.Exit(cmd.Execute())
}
