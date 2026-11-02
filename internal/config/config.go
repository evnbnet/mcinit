package config

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/evanb/mcinit/internal/mcver"
)

type Software string

const (
	Paper  Software = "paper"
	Purpur Software = "purpur"
)

func (s Software) Display() string {
	switch s {
	case Paper:
		return "Paper"
	case Purpur:
		return "Purpur"
	}
	return string(s)
}

func ParseSoftware(s string) (Software, error) {
	switch Software(strings.ToLower(strings.TrimSpace(s))) {
	case Paper:
		return Paper, nil
	case Purpur:
		return Purpur, nil
	}
	return "", fmt.Errorf("unknown server software %q (use paper or purpur)", s)
}

const Latest = "latest"

const (
	MinRAM = 1
	MaxRAM = 256
)

type Config struct {
	Name     string
	Software Software
	Version  string
	RAM      int
	Dir      string

	AcceptedEULA bool

	MOTD       string
	Gamemode   string
	Difficulty string
	MaxPlayers int
	Port       int
	OnlineMode bool
}

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

func (c Config) AbsDir() (string, error) { return filepath.Abs(c.Dir) }
