package download

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func serve(t *testing.T, body []byte) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func TestFetchVerifiesAndInstalls(t *testing.T) {
	body := []byte("pretend this is a server jar")
	srv := serve(t, body)
	dest := filepath.Join(t.TempDir(), "server.jar")

	err := Fetch(context.Background(), Request{
		URL:      srv.URL,
		Dest:     dest,
		Algo:     AlgoSHA256,
		Checksum: sha256Hex(body),
	})
	if err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(body) {
		t.Errorf("contents = %q, want %q", got, body)
	}
	assertNoPartLeftovers(t, dest)
}

func TestFetchAcceptsUppercaseChecksum(t *testing.T) {
	body := []byte("digest case should not matter")
	srv := serve(t, body)
	dest := filepath.Join(t.TempDir(), "server.jar")

	err := Fetch(context.Background(), Request{
		URL:      srv.URL,
		Dest:     dest,
		Algo:     AlgoSHA256,
		Checksum: strings.ToUpper(sha256Hex(body)),
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestFetchRejectsBadChecksum(t *testing.T) {
	srv := serve(t, []byte("corrupted payload"))
	dest := filepath.Join(t.TempDir(), "server.jar")

	err := Fetch(context.Background(), Request{
		URL:      srv.URL,
		Dest:     dest,
		Algo:     AlgoSHA256,
		Checksum: sha256Hex([]byte("what we actually asked for")),
	})
	if err == nil {
		t.Fatal("Fetch accepted a file whose digest did not match")
	}

	if _, statErr := os.Stat(dest); !os.IsNotExist(statErr) {
		t.Error("the destination exists after a checksum mismatch")
	}
	assertNoPartLeftovers(t, dest)
}

func TestFetchMD5(t *testing.T) {

	body := []byte("purpur jar")
	srv := serve(t, body)
	dest := filepath.Join(t.TempDir(), "purpur.jar")

	err := Fetch(context.Background(), Request{
		URL:      srv.URL,
		Dest:     dest,
		Algo:     AlgoMD5,
		Checksum: "0e3e4bf7a09e1ff7f5e1a0e48ba3ce2a",
	})

	if err != nil && strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("MD5 was rejected as unsupported: %v", err)
	}
}

func TestFetchRequiresChecksum(t *testing.T) {
	srv := serve(t, []byte("x"))
	err := Fetch(context.Background(), Request{
		URL:  srv.URL,
		Dest: filepath.Join(t.TempDir(), "x.jar"),
		Algo: AlgoSHA256,
	})
	if err == nil {
		t.Fatal("Fetch ran without a published checksum")
	}
}

func TestFetchRejectsUnknownAlgo(t *testing.T) {
	err := Fetch(context.Background(), Request{
		URL:      "http://example.invalid",
		Dest:     filepath.Join(t.TempDir(), "x.jar"),
		Algo:     "crc32",
		Checksum: "deadbeef",
	})
	if err == nil {
		t.Fatal("Fetch accepted an unsupported algorithm")
	}
}

func TestFetchReportsHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusNotFound)
	}))
	defer srv.Close()

	dest := filepath.Join(t.TempDir(), "x.jar")
	err := Fetch(context.Background(), Request{
		URL: srv.URL, Dest: dest, Algo: AlgoSHA256, Checksum: sha256Hex(nil),
	})
	if err == nil || !strings.Contains(err.Error(), "404") {
		t.Fatalf("err = %v, want a 404 report", err)
	}
	assertNoPartLeftovers(t, dest)
}

func TestFetchOverwritesExistingFile(t *testing.T) {

	body := []byte("new jar")
	srv := serve(t, body)
	dest := filepath.Join(t.TempDir(), "server.jar")
	if err := os.WriteFile(dest, []byte("old jar"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := Fetch(context.Background(), Request{
		URL: srv.URL, Dest: dest, Algo: AlgoSHA256, Checksum: sha256Hex(body),
	})
	if err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(dest)
	if string(got) != string(body) {
		t.Errorf("contents = %q, want the new jar", got)
	}
}

func assertNoPartLeftovers(t *testing.T, dest string) {
	t.Helper()
	if _, err := os.Stat(dest + partSuffix); !os.IsNotExist(err) {
		t.Errorf("%s was left behind", dest+partSuffix)
	}
}
