package server

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// propertiesFile is the file that marks a directory as a Minecraft server.
const propertiesFile = "server.properties"

// defaultLevelName is what Minecraft uses when server.properties says nothing.
const defaultLevelName = "world"

// Info describes an existing server directory, read back from its own files
// rather than from anything mcinit remembers.
type Info struct {
	Dir       string
	Name      string // the directory's base name
	LevelName string
	Port      int
}

// Inspect reads an existing server directory. It fails with something the user
// can act on when the directory is not a server at all, which is the common
// mistake of running a command from the wrong place.
func Inspect(dir string) (Info, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return Info{}, err
	}

	props, err := readProperties(filepath.Join(abs, propertiesFile))
	if err != nil {
		return Info{}, err
	}

	info := Info{
		Dir:       abs,
		Name:      filepath.Base(abs),
		LevelName: defaultLevelName,
		Port:      25565,
	}
	if v := strings.TrimSpace(props["level-name"]); v != "" {
		info.LevelName = v
	}
	if n, err := strconv.Atoi(strings.TrimSpace(props["server-port"])); err == nil && n > 0 && n <= 65535 {
		info.Port = n
	}
	return info, nil
}

// Worlds returns the three world directories that belong to this server, in the
// order Minecraft creates them. Directories that do not exist are skipped: a
// server that never started has none of them.
func (i Info) Worlds() []string {
	var out []string
	for _, name := range []string{
		i.LevelName,
		i.LevelName + "_nether",
		i.LevelName + "_the_end",
	} {
		if st, err := os.Stat(filepath.Join(i.Dir, name)); err == nil && st.IsDir() {
			out = append(out, name)
		}
	}
	return out
}

// readProperties parses the subset of the Java properties format that
// server.properties actually uses: key=value lines, # comments, blank lines.
func readProperties(path string) (map[string]string, error) {
	file, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil, fmt.Errorf("%s does not look like a Minecraft server (no %s)",
			filepath.Dir(path), propertiesFile)
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()

	props := map[string]string{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "!") {
			continue
		}
		key, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		props[strings.TrimSpace(key)] = unescapeProperty(value)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return props, nil
}

// unescapeProperty reverses escapeProperty in render.go.
func unescapeProperty(s string) string {
	s = strings.ReplaceAll(s, `\n`, "\n")
	return strings.ReplaceAll(s, doubleBackslash, backslash)
}

// Spelled out as constants because an inline `\\` literal is easy to mangle.
const (
	doubleBackslash = `\` + `\`
	backslash       = `\`
)

// IsRunning reports whether a server appears to be running out of this
// directory, and why.
//
// This matters for more than tidiness: a backup taken while Minecraft is
// writing region files can capture a torn world, and a restore cannot delete
// directories whose files are open.
func (i Info) IsRunning() (bool, string) {
	for _, world := range i.Worlds() {
		if sessionLocked(filepath.Join(i.Dir, world, "session.lock")) {
			return true, filepath.Join(world, "session.lock") + " is locked"
		}
	}

	// Second opinion, for a server whose world has not been created yet.
	if portInUse(i.Port) {
		return true, fmt.Sprintf("port %d is in use", i.Port)
	}
	return false, ""
}

// sessionLocked reports whether Minecraft currently holds its lock on a
// session.lock file.
//
// Java takes a byte-range lock here, which on Windows does not prevent opening
// the file — only reading or writing the locked range. Verified against a
// running Paper server: the open succeeds, and the read fails with "another
// process has locked a portion of the file". So the probe has to attempt real
// I/O, and it has to be a *read*: a write that unexpectedly succeeded would
// corrupt the file Minecraft depends on.
func sessionLocked(path string) bool {
	file, err := os.Open(path)
	if err != nil {
		// Missing or unreadable tells us nothing about the server.
		return false
	}
	defer file.Close()

	_, err = file.Read(make([]byte, 1))
	// io.EOF means an empty but unlocked file; only a genuine I/O error here
	// indicates the lock.
	return err != nil && err != io.EOF
}

// portInUse reports whether something is already listening on a local port.
func portInUse(port int) bool {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), 300*time.Millisecond)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}
