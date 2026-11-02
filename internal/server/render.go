package server

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/evanb/mcinit/internal/buildinfo"
	"github.com/evanb/mcinit/internal/config"
	"github.com/evanb/mcinit/internal/provider"
	"github.com/evanb/mcinit/templates"
)

// largeHeapGB is where Aikar's flags switch to the tuning for big heaps.
const largeHeapGB = 12

// templateData is everything the templates can reference. It is deliberately
// flat: the templates stay readable and the values are computed once, here.
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
	// VectorModule gates --add-modules=jdk.incubator.vector, which does not
	// exist before Java 9 and would make the server refuse to start.
	VectorModule bool

	// Aikar's G1 tuning, which differs above and below a 12 GB heap.
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

// escapeProperty makes a value safe for the Java properties format that
// server.properties uses: a stray backslash would otherwise be read as an
// escape sequence, and a newline would split the entry in two.
func escapeProperty(s string) string {
	s = strings.ReplaceAll(s, backslash, doubleBackslash)
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.ReplaceAll(s, "\n", `\n`)
	return s
}

// generated maps each template to the file it becomes, and records which ones
// need Windows line endings.
var generated = []struct {
	template string
	output   string
	crlf     bool
}{
	{"eula.txt.tmpl", "eula.txt", false},
	{"server.properties.tmpl", "server.properties", false},
	// cmd.exe tolerates LF, but a batch file with mixed endings has bitten
	// enough people that it is not worth the risk.
	{"start.bat.tmpl", "start.bat", true},
}

// renderAll writes every generated file into dir.
func renderAll(dir string, data templateData) error {
	for _, g := range generated {
		body, err := render(g.template, data)
		if err != nil {
			return err
		}
		if g.crlf {
			body = toCRLF(body)
		}
		if err := os.WriteFile(filepath.Join(dir, g.output), body, 0o644); err != nil {
			return fmt.Errorf("could not write %s: %w", g.output, err)
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
