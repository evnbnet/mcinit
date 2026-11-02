// Package backup saves and restores a server's worlds and configuration.
//
// The server jar is deliberately not part of a backup: it is a few dozen
// megabytes that mcinit can download again, while the world is the only thing
// in the directory that cannot be replaced.
package backup

import (
	"path"
	"strings"

	"github.com/evanb/mcinit/internal/server"
)

// BackupsDir is where backups live, inside the server directory so that moving
// the server folder moves its history with it.
const BackupsDir = "backups"

// manifestName is the entry inside each zip that records what it is.
const manifestName = "mcinit-backup.json"

// configFiles are the root files worth keeping: they hold who is an operator,
// who is allowed in, who is banned, and how the server is set up.
var configFiles = []string{
	"server.properties",
	"ops.json",
	"whitelist.json",
	"banned-players.json",
	"banned-ips.json",
	"usercache.json",
	"bukkit.yml",
	"spigot.yml",
	"purpur.yml",
	"permissions.yml",
	"commands.yml",
	"help.yml",
}

// configDirs are directories kept whole. plugins holds both the plugin jars and
// their configuration; mcinit cannot re-download plugins, so they are backed up
// rather than skipped the way the server jar is.
var configDirs = []string{
	"plugins",
	"config",
}

// skipDirs are directories that are either re-downloadable or pure noise.
var skipDirs = []string{
	BackupsDir, // a backup must never contain the other backups
	"libraries",
	"versions",
	"cache",
	"logs",
	"crash-reports",
	"debug",
}

// Selection decides what goes into a backup of one particular server.
type Selection struct {
	worlds map[string]bool
}

// NewSelection builds the rules for a server, which depend on its level-name:
// a server with "level-name=survival" keeps survival, survival_nether and
// survival_the_end rather than the default world directories.
func NewSelection(info server.Info) Selection {
	worlds := map[string]bool{}
	for _, name := range []string{
		info.LevelName,
		info.LevelName + "_nether",
		info.LevelName + "_the_end",
	} {
		worlds[name] = true
	}
	return Selection{worlds: worlds}
}

// Include reports whether a path relative to the server directory belongs in
// the backup. rel always uses forward slashes, matching zip entry names.
//
// The rule that is easy to get wrong: the server jar is excluded, but only at
// the root. Plugin jars live under plugins/ and must be kept.
func (s Selection) Include(rel string, isDir bool) bool {
	rel = path.Clean(strings.ReplaceAll(rel, `\`, "/"))
	if rel == "." || rel == "" {
		return false
	}

	// Dot-directories are tool internals, never user data. Paper keeps a
	// regenerable cache in plugins/.paper-remapped that is larger than the
	// world itself, and mcinit's own restore staging directory lives under a
	// dot name too.
	for _, part := range strings.Split(rel, "/") {
		if strings.HasPrefix(part, ".") {
			return false
		}
	}

	top, rest, nested := strings.Cut(rel, "/")

	for _, skip := range skipDirs {
		if top == skip {
			return false
		}
	}

	// Worlds and kept directories come along whole, with everything inside.
	if s.worlds[top] || contains(configDirs, top) {
		return true
	}

	if nested {
		// Anything else nested belongs to a directory we did not opt into.
		_ = rest
		return false
	}

	if isDir {
		return false
	}
	// A root-level jar is the server itself: re-downloadable, and the single
	// biggest file in the directory.
	if strings.HasSuffix(strings.ToLower(top), ".jar") {
		return false
	}
	return contains(configFiles, top)
}

func contains(list []string, want string) bool {
	for _, item := range list {
		if item == want {
			return true
		}
	}
	return false
}
