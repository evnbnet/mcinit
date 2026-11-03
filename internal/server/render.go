package server

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"text/template"
	"time"

	"github.com/evanb/mcinit/internal/buildinfo"
	"github.com/evanb/mcinit/internal/config"
	"github.com/evanb/mcinit/internal/provider"
	"github.com/evanb/mcinit/templates"
)

const largeHeapGB = 12

type templateData struct {
	McinitVersion string
	Generated     string

	Name      string
	Software  string
	MCVersion string
	Build     string
	JarName   string

	JavaPath  string
	JavaMajor int

	RAM int

	VectorModule bool

	G1NewSizePercent               int
	G1MaxNewSizePercent            int
	G1HeapRegionSize               string
	G1ReservePercent               int
	InitiatingHeapOccupancyPercent int

	MOTD       string
	Gamemode   string
	Difficulty string
	MaxPlayers int
	Port       int
	OnlineMode bool
}

func newTemplateData(cfg config.Config, art provider.Artifact, javaPath string, javaMajor int) templateData {
	d := templateData{
		McinitVersion: buildinfo.Version,
		Generated:     time.Now().Format("2006-01-02 15:04"),
		Name:          cfg.Name,
		Software:      cfg.Software.Display(),
		MCVersion:     cfg.Version,
		Build:         art.Build,
		JarName:       art.Filename,
		JavaPath:      javaPath,
		JavaMajor:     javaMajor,
		RAM:           cfg.RAM,
		VectorModule:  javaMajor >= 9,
		MOTD:          escapeProperty(cfg.MOTD),
		Gamemode:      cfg.Gamemode,
		Difficulty:    cfg.Difficulty,
		MaxPlayers:    cfg.MaxPlayers,
		Port:          cfg.Port,
		OnlineMode:    cfg.OnlineMode,
	}

	if cfg.RAM >= largeHeapGB {
		d.G1NewSizePercent = 40
		d.G1MaxNewSizePercent = 50
		d.G1HeapRegionSize = "16M"
		d.G1ReservePercent = 15
		d.InitiatingHeapOccupancyPercent = 20
	} else {
		d.G1NewSizePercent = 30
		d.G1MaxNewSizePercent = 40
		d.G1HeapRegionSize = "8M"
		d.G1ReservePercent = 20
		d.InitiatingHeapOccupancyPercent = 15
	}
	return d
}

func escapeProperty(s string) string {
	s = strings.ReplaceAll(s, backslash, doubleBackslash)
	s = strings.ReplaceAll(s, "\r", "")
	return strings.ReplaceAll(s, "\n", `\n`)
}

// generatedFile is one file mcinit writes into a new server directory.
type generatedFile struct {
	template string
	output   string
	// crlf marks a file that must use Windows line endings. cmd.exe tolerates
	// LF, but a batch file with mixed endings has bitten enough people that it
	// is not worth the risk.
	crlf bool
	// mode is the permission the file is written with. The start script on
	// Unix has to be executable or neither Finder nor the shell will run it.
	mode os.FileMode
}

// filesFor returns what to generate for a target operating system.
//
// The target is a parameter rather than a call to runtime.GOOS so that the
// macOS output can be rendered and tested from any machine — which matters,
// because the start script is the single most important file mcinit writes and
// the one that is hardest to check by hand on a platform you are not on.
func filesFor(goos string) []generatedFile {
	common := []generatedFile{
		{template: "eula.txt.tmpl", output: "eula.txt", mode: 0o644},
		{template: "server.properties.tmpl", output: "server.properties", mode: 0o644},
	}

	if goos == "windows" {
		return append(common, generatedFile{
			template: "start.bat.tmpl", output: "start.bat", crlf: true, mode: 0o644,
		})
	}
	return append(common, generatedFile{
		template: "start.command.tmpl", output: "start.command", mode: 0o755,
	})
}

// StartScript is the name of the launch script on the current platform.
func StartScript() string {
	for _, g := range filesFor(runtime.GOOS) {
		if strings.HasPrefix(g.output, "start.") {
			return g.output
		}
	}
	return "start.sh"
}

func renderAll(dir string, data templateData) error {
	return renderAllFor(runtime.GOOS, dir, data)
}

// renderAllFor writes the generated files for a target operating system.
func renderAllFor(goos, dir string, data templateData) error {
	for _, g := range filesFor(goos) {
		body, err := render(g.template, data)
		if err != nil {
			return err
		}
		if g.crlf {
			body = toCRLF(body)
		}
		path := filepath.Join(dir, g.output)
		if err := os.WriteFile(path, body, g.mode); err != nil {
			return fmt.Errorf("could not write %s: %w", g.output, err)
		}
		// WriteFile only applies the mode when it creates the file, so a
		// rerun over an existing script would leave it non-executable.
		if err := os.Chmod(path, g.mode); err != nil {
			return fmt.Errorf("could not set permissions on %s: %w", g.output, err)
		}
	}
	return nil
}

func render(name string, data templateData) ([]byte, error) {
	tmpl, err := template.New(name).ParseFS(templates.FS, name)
	if err != nil {
		return nil, fmt.Errorf("invalid template %s: %w", name, err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("could not render %s: %w", name, err)
	}
	return buf.Bytes(), nil
}

func toCRLF(b []byte) []byte {
	b = bytes.ReplaceAll(b, []byte("\r\n"), []byte("\n"))
	return bytes.ReplaceAll(b, []byte("\n"), []byte("\r\n"))
}
