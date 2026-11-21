package cli

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/orange-dinosaur/squirrel/internal/upgrade"
)

// fakeReleases serves v0.1.0 and v0.2.0 (the latest) for this platform, swaps the running
// binary for a file in a temp folder, and sets the running version. It returns the file.
func fakeReleases(t *testing.T, running string) string {
	t.Helper()
	cliEnv(t)
	mux := http.NewServeMux()
	mux.HandleFunc("/o/r/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/o/r/releases/tag/v0.2.0", http.StatusFound)
	})
	for _, v := range []string{"0.1.0", "0.2.0"} {
		var buf bytes.Buffer
		gz := gzip.NewWriter(&buf)
		tw := tar.NewWriter(gz)
		body := []byte("binary " + v)
		tw.WriteHeader(&tar.Header{Name: "squirrel", Mode: 0o755, Size: int64(len(body)), Typeflag: tar.TypeReg})
		tw.Write(body)
		tw.Close()
		gz.Close()
		archive := buf.Bytes()
		asset := fmt.Sprintf("squirrel_%s_%s_%s.tar.gz", v, runtime.GOOS, runtime.GOARCH)
		dir := "/o/r/releases/download/v" + v + "/"
		mux.HandleFunc(dir+"checksums.txt", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintf(w, "%x  %s\n", sha256.Sum256(archive), asset)
		})
		mux.HandleFunc(dir+asset, func(w http.ResponseWriter, r *http.Request) { w.Write(archive) })
	}
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	bin := filepath.Join(t.TempDir(), "squirrel")
	if err := os.WriteFile(bin, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	oldReleases, oldExec, oldVersion := releases, executable, version
	t.Cleanup(func() { releases, executable, version = oldReleases, oldExec, oldVersion })
	releases = upgrade.Source{BaseURL: srv.URL, Repo: "o/r", Client: srv.Client()}
	executable = func() (string, error) { return bin, nil }
	version = running
	return bin
}

func TestUpgrade(t *testing.T) {
	tests := []struct {
		name, running string
		args          []string
		out, err      string
		binary        string
	}{
		{"source build refused", "", []string{"upgrade"}, "",
			"squirrel was built from source (unknown); pass --force to replace it with the latest release", "old"},
		{"source build refused, named", "", []string{"upgrade", "0.1.0"}, "",
			"squirrel was built from source (unknown); pass --force to replace it with v0.1.0", "old"},
		{"force", "", []string{"upgrade", "--force"}, "UPGRADED  squirrel unknown → 0.2.0\n", "", "binary 0.2.0"},
		{"already latest", "0.2.0", []string{"upgrade"}, "NOTE  squirrel 0.2.0 is the latest\n", "", "old"},
		{"already installed", "0.1.0", []string{"upgrade", "v0.1.0"}, "NOTE  squirrel 0.1.0 is already installed\n", "", "old"},
		{"newer", "0.1.0", []string{"upgrade"}, "UPGRADED  squirrel 0.1.0 → 0.2.0\n", "", "binary 0.2.0"},
		{"named older", "0.2.0", []string{"upgrade", "0.1.0"}, "UPGRADED  squirrel 0.2.0 → 0.1.0\n", "", "binary 0.1.0"},
		{"unknown tag", "0.2.0", []string{"upgrade", "9.9.9"}, "", "no release v9.9.9", "old"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bin := fakeReleases(t, tt.running)
			out, err := squirrel(t, tt.args...)
			if out != tt.out || err != tt.err {
				t.Errorf("got output %q, error %q\nwant output %q, error %q", out, err, tt.out, tt.err)
			}
			if got, _ := os.ReadFile(bin); string(got) != tt.binary {
				t.Errorf("binary holds %q, want %q", got, tt.binary)
			}
		})
	}
}

func TestUpgradeReadOnlyFolder(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can write a read-only folder")
	}
	bin := fakeReleases(t, "0.1.0")
	dir := filepath.Dir(bin)
	os.Chmod(dir, 0o555)
	t.Cleanup(func() { os.Chmod(dir, 0o755) })
	_, err := squirrel(t, "upgrade")
	if want := "can't replace " + bin + ": permission denied, run with sudo"; err != want {
		t.Errorf("got %q, want %q", err, want)
	}
}
