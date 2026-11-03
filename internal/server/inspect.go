package server

import (
	"bufio"
	"fmt"
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

	values, err := readProperties(filepath.Join(abs, propertiesFile))
	if err != nil {
		return Info{}, err
	}

	info := Info{
		Dir:       abs,
		Name:      filepath.Base(abs),
		LevelName: defaultLevelName,
		Port:      25565,
	}
	if v := strings.TrimSpace(values["level-name"]); v != "" {
		info.LevelName = v
	}
	if n, err := strconv.Atoi(strings.TrimSpace(values["server-port"])); err == nil && n > 0 && n <= 65535 {
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

// readProperties parses the subset of the Java properties format that
// server.properties uses: key=value lines, # comments, blank lines.
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

	values := map[string]string{}
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
		values[strings.TrimSpace(key)] = unescapeProperty(value)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return values, nil
}

// unescapeProperty reverses escapeProperty.
//
// Minecraft escapes more than a naive implementation expects — it writes
// level-type as "minecraft\:normal" and non-ASCII as \uXXXX — so the cases
// below are the ones a real server.properties actually contains.
func unescapeProperty(s string) string {
	if !strings.Contains(s, backslash) {
		return s
	}

	var b strings.Builder
	b.Grow(len(s))

	runes := []rune(s)
	for i := 0; i < len(runes); i++ {
		if runes[i] != '\\' || i == len(runes)-1 {
			b.WriteRune(runes[i])
			continue
		}

		i++
		switch runes[i] {
		case 'n':
			b.WriteRune('\n')
		case 't':
			b.WriteRune('\t')
		case 'r':
			b.WriteRune('\r')
		case 'u':
			// Malformed escapes are kept as written rather than silently
			// turned into a replacement character.
			if i+4 < len(runes) {
				if code, err := strconv.ParseUint(string(runes[i+1:i+5]), 16, 32); err == nil {
					b.WriteRune(rune(code))
					i += 4
					continue
				}
			}
			b.WriteString(`\u`)
		default:
			// Covers \\, \:, \= and \#, all of which stand for themselves.
			b.WriteRune(runes[i])
		}
	}
	return b.String()
}

// Spelled out as constants because an inline `\\` literal is easy to mangle,
// and a mangled one turns the escaping into a silent no-op.
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

func portInUse(port int) bool {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), 300*time.Millisecond)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}
