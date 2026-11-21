package upgrade

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func tarball(t *testing.T, name string, body []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(body)), Typeflag: tar.TypeReg}); err != nil {
		t.Fatal(err)
	}
	tw.Write(body)
	tw.Close()
	gz.Close()
	return buf.Bytes()
}

// fakeGitHub serves one Release, v0.2.0, with a linux_amd64 archive holding entry.
// A non-empty sum replaces the archive's real checksum.
func fakeGitHub(t *testing.T, entry, sum string) Source {
	t.Helper()
	const asset = "squirrel_0.2.0_linux_amd64.tar.gz"
	archive := tarball(t, entry, []byte("new binary"))
	if sum == "" {
		sum = fmt.Sprintf("%x", sha256.Sum256(archive))
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/o/r/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/o/r/releases/tag/v0.2.0", http.StatusFound)
	})
	mux.HandleFunc("/o/r/releases/tag/v0.2.0", func(w http.ResponseWriter, r *http.Request) {
		t.Error("Latest followed the redirect")
	})
	mux.HandleFunc("/o/r/releases/download/v0.2.0/checksums.txt", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "%s  squirrel_0.2.0_darwin_arm64.tar.gz\n%s  %s\n", strings.Repeat("0", 64), sum, asset)
	})
	mux.HandleFunc("/o/r/releases/download/v0.2.0/"+asset, func(w http.ResponseWriter, r *http.Request) {
		w.Write(archive)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return Source{BaseURL: srv.URL, Repo: "o/r", Client: srv.Client()}
}

func TestTag(t *testing.T) {
	for _, in := range []string{"0.1.0", "v0.1.0"} {
		if got := Tag(in); got != "v0.1.0" {
			t.Errorf("Tag(%q) = %q", in, got)
		}
	}
}

func TestLatest(t *testing.T) {
	tag, err := fakeGitHub(t, "squirrel", "").Latest(context.Background())
	if err != nil || tag != "v0.2.0" {
		t.Fatalf("Latest() = %q, %v", tag, err)
	}
}

func TestFetch(t *testing.T) {
	bin, err := fakeGitHub(t, "squirrel", "").Fetch(context.Background(), "v0.2.0", "linux", "amd64")
	if err != nil || string(bin) != "new binary" {
		t.Fatalf("Fetch() = %q, %v", bin, err)
	}
}

func TestFetchErrors(t *testing.T) {
	tests := []struct {
		name, entry, sum, tag, goos, want string
	}{
		{"unknown tag", "squirrel", "", "v9.9.9", "linux", "no release v9.9.9"},
		{"missing platform", "squirrel", "", "v0.2.0", "freebsd", "no release for freebsd/amd64"},
		{"bad checksum", "squirrel", strings.Repeat("a", 64), "v0.2.0", "linux", "squirrel_0.2.0_linux_amd64.tar.gz does not match its checksum"},
		{"no binary", "README.md", "", "v0.2.0", "linux", "squirrel_0.2.0_linux_amd64.tar.gz: no squirrel binary inside"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := fakeGitHub(t, tt.entry, tt.sum).Fetch(context.Background(), tt.tag, tt.goos, "amd64")
			if err == nil || err.Error() != tt.want {
				t.Fatalf("got %v, want %q", err, tt.want)
			}
		})
	}
}
