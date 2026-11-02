package backup

import (
	"path"
	"strings"

	"github.com/evanb/mcinit/internal/server"
)

const BackupsDir = "backups"

const manifestName = "mcinit-backup.json"

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

var configDirs = []string{
	"plugins",
	"config",
}

var skipDirs = []string{
	BackupsDir,
	"libraries",
	"versions",
	"cache",
	"logs",
	"crash-reports",
	"debug",
}

type Selection struct {
	worlds map[string]bool
}

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

func (s Selection) Include(rel string, isDir bool) bool {
	rel = path.Clean(strings.ReplaceAll(rel, `\`, "/"))
	if rel == "." || rel == "" {
		return false
	}

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

	if s.worlds[top] || contains(configDirs, top) {
		return true
	}

	if nested {

		_ = rest
		return false
	}

	if isDir {
		return false
	}

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
