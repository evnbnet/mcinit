package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/evanb/mcinit/internal/config"
)

func serveFixtures(t *testing.T, routes map[string]string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Errorf("request to %s carried no User-Agent", r.URL.Path)
		}
		file, ok := routes[r.URL.Path]
		if !ok {
			t.Errorf("unexpected request path %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		body, err := os.ReadFile(filepath.Join("testdata", file))
		if err != nil {
			t.Fatalf("read fixture: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestPaperParsing(t *testing.T) {
	srv := serveFixtures(t, map[string]string{
		"/":                            "paper_versions.json",
		"/versions/26.3/builds/latest": "paper_build.json",
	})
	old := paperAPI
	paperAPI = srv.URL
	t.Cleanup(func() { paperAPI = old })

	p, err := For(config.Paper)
	if err != nil {
		t.Fatal(err)
	}

	versions, err := p.Versions(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(versions) == 0 {
		t.Fatal("no versions parsed; the map-of-groups shape may have changed")
	}

	if got := versions[0].Raw; got != "26.3" {
		t.Errorf("newest version = %q, want 26.3", got)
	}
	for _, v := range versions {
		if !v.Stable() {
			t.Errorf("Versions returned pre-release %q", v.Raw)
		}
	}

	art, err := p.Resolve(context.Background(), "26.3")
	if err != nil {
		t.Fatal(err)
	}
	if art.Algo != SHA256 {
		t.Errorf("Algo = %q, want %q", art.Algo, SHA256)
	}
	if len(art.Checksum) != 64 {
		t.Errorf("sha256 digest = %q, want 64 hex chars", art.Checksum)
	}
	if art.Filename == "" || !strings.HasSuffix(art.Filename, ".jar") {
		t.Errorf("Filename = %q, want a .jar", art.Filename)
	}
	if art.URL == "" {
		t.Error("URL is empty")
	}
	if art.Size <= 0 {
		t.Errorf("Size = %d, want a positive byte count", art.Size)
	}
	if art.Build == "" {
		t.Error("Build is empty")
	}
	if art.Experimental() {
		t.Errorf("Channel = %q, want the fixture's STABLE build", art.Channel)
	}
}

func TestPurpurParsing(t *testing.T) {
	srv := serveFixtures(t, map[string]string{
		"/":            "purpur_versions.json",
		"/26.3/latest": "purpur_build.json",
	})
	old := purpurAPI
	purpurAPI = srv.URL
	t.Cleanup(func() { purpurAPI = old })

	p, err := For(config.Purpur)
	if err != nil {
		t.Fatal(err)
	}

	versions, err := p.Versions(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(versions) == 0 {
		t.Fatal("no versions parsed")
	}
	if got := versions[0].Raw; got != "26.3" {
		t.Errorf("newest version = %q, want 26.3", got)
	}

	art, err := p.Resolve(context.Background(), "26.3")
	if err != nil {
		t.Fatal(err)
	}

	if art.Algo != MD5 {
		t.Errorf("Algo = %q, want %q", art.Algo, MD5)
	}
	if len(art.Checksum) != 32 {
		t.Errorf("md5 digest = %q, want 32 hex chars", art.Checksum)
	}
	if want := srv.URL + "/26.3/2655/download"; art.URL != want {
		t.Errorf("URL = %q, want %q", art.URL, want)
	}
	if art.Filename != "purpur-26.3-2655.jar" {
		t.Errorf("Filename = %q", art.Filename)
	}
}

func TestPurpurRejectsFailedBuild(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"version":"26.3","build":"2655","result":"FAILURE","md5":"abc"}`))
	}))
	defer srv.Close()
	old := purpurAPI
	purpurAPI = srv.URL
	t.Cleanup(func() { purpurAPI = old })

	if _, err := (purpur{}).Resolve(context.Background(), "26.3"); err == nil {
		t.Fatal("Resolve accepted a build whose result was FAILURE")
	}
}

func TestForRejectsUnknownSoftware(t *testing.T) {
	if _, err := For(config.Software("spigot")); err == nil {
		t.Fatal("For accepted unknown software")
	}
}

func TestLiveAPIs(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping live API check in -short mode")
	}
	for _, sw := range []config.Software{config.Paper, config.Purpur} {
		t.Run(string(sw), func(t *testing.T) {
			p, err := For(sw)
			if err != nil {
				t.Fatal(err)
			}
			latest, err := LatestStable(context.Background(), p)
			if err != nil {
				t.Fatalf("LatestStable: %v", err)
			}
			art, err := p.Resolve(context.Background(), latest)
			if err != nil {
				t.Fatalf("Resolve(%s): %v", latest, err)
			}
			if art.URL == "" || art.Checksum == "" || art.Filename == "" {
				t.Fatalf("incomplete artifact for %s %s: %+v", sw, latest, art)
			}
			t.Logf("%s %s build %s → %s (%s:%s)", sw, latest, art.Build, art.Filename, art.Algo, art.Checksum)
		})
	}
}
