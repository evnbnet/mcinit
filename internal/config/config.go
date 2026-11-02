// Package config holds the settings that describe a server to create. It is
// filled either by the interactive wizard or by command-line flags, and is the
// only thing the creation step needs.
package config

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/evanb/mcinit/internal/mcver"
)

// Software identifies a server distribution.
type Software string

const (
	Paper  Software = "paper"
	Purpur Software = "purpur"
)

// Display returns the name as it is written in the project's own branding.
func (s Software) Display() string {
	switch s {
	case Paper:
		return "Paper"
	case Purpur:
		return "Purpur"
	}
	return string(s)
}

// ParseSoftware accepts the flag spelling of a distribution.
func ParseSoftware(s string) (Software, error) {
	switch Software(strings.ToLower(strings.TrimSpace(s))) {
	case Paper:
		return Paper, nil
	case Purpur:
		return Purpur, nil
	}
	return "", fmt.Errorf("unknown server software %q (use paper or purpur)", s)
}

// Latest asks the provider for its newest stable version instead of naming one.
const Latest = "latest"

// RAM bounds. The floor is what a vanilla world needs to not thrash; the
// ceiling only guards against typos like --ram 400.
const (
	MinRAM = 1
	MaxRAM = 256
)

// Config fully describes the server to create.
type Config struct {
	Name     string   // project name, used for the default directory and MOTD
	Software Software //
	Version  string   // Minecraft version, or Latest before resolution
	RAM      int      // heap size in gigabytes
	Dir      string   // target directory

	// AcceptedEULA records an explicit yes. eula.txt is only written when this
	// is true; there is no path that accepts Mojang's terms on the user's
	// behalf.
	AcceptedEULA bool

	// Defaults baked into server.properties. The wizard does not ask for
	// these; they exist so the template has one source of truth and the final
	// summary can show what was chosen.
	MOTD       string
	Gamemode   string
	Difficulty string
	MaxPlayers int
	Port       int
	OnlineMode bool
}

// Default returns a Config with everything but the user's choices filled in.
func Default() Config {
	return Config{
		Version:    Latest,
		RAM:        4,
		MOTD:       "A server created with mcinit",
		Gamemode:   "survival",
		Difficulty: "normal",
		MaxPlayers: 20,
		Port:       25565,
		OnlineMode: true,
	}
}

// Validate reports the first problem that would make creation fail, phrased for
// the user rather than for a log.
func (c Config) Validate() error {
	if c.Name == "" {
		return fmt.Errorf("the server needs a name")
	}
	if c.Software != Paper && c.Software != Purpur {
		return fmt.Errorf("unknown server software %q (use paper or purpur)", c.Software)
	}
	if c.Version == "" {
		return fmt.Errorf("the Minecraft version is missing")
	}
	if c.Version != Latest {
		if _, err := mcver.Parse(c.Version); err != nil {
			return err
		}
	}
	if c.RAM < MinRAM || c.RAM > MaxRAM {
		return fmt.Errorf("RAM must be between %d and %d GB (you asked for %d)", MinRAM, MaxRAM, c.RAM)
	}
	if c.Dir == "" {
		return fmt.Errorf("the destination folder is missing")
	}
	if c.Port < 1 || c.Port > 65535 {
		return fmt.Errorf("invalid port %d", c.Port)
	}
	return nil
}

// AbsDir resolves Dir against the working directory.
func (c Config) AbsDir() (string, error) { return filepath.Abs(c.Dir) }
