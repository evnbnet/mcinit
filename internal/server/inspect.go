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

const propertiesFile = "server.properties"

const defaultLevelName = "world"

type Info struct {
	Dir       string
	Name      string
	LevelName string
	Port      int
}

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

func unescapeProperty(s string) string {
	s = strings.ReplaceAll(s, `\n`, "\n")
	return strings.ReplaceAll(s, doubleBackslash, backslash)
}

const (
	doubleBackslash = `\` + `\`
	backslash       = `\`
)

func (i Info) IsRunning() (bool, string) {
	for _, world := range i.Worlds() {
		if sessionLocked(filepath.Join(i.Dir, world, "session.lock")) {
			return true, filepath.Join(world, "session.lock") + " is locked"
		}
	}

	if portInUse(i.Port) {
		return true, fmt.Sprintf("port %d is in use", i.Port)
	}
	return false, ""
}

func sessionLocked(path string) bool {
	file, err := os.Open(path)
	if err != nil {

		return false
	}
	defer file.Close()

	_, err = file.Read(make([]byte, 1))

	return err != nil && err != io.EOF
}

func portInUse(port int) bool {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), 300*time.Millisecond)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}
