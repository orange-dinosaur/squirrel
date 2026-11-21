// Package upgrade finds squirrel Releases on GitHub and fetches their binaries.
package upgrade

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
)

// Source is where Releases are published.
type Source struct {
	BaseURL string // https://github.com, or a test server
	Repo    string // owner/name
	Client  *http.Client
}

// GitHub is the real Release source.
func GitHub() Source {
	return Source{BaseURL: "https://github.com", Repo: "orange-dinosaur/squirrel", Client: http.DefaultClient}
}

// Tag turns a user-given version into a tag: "0.1.0" and "v0.1.0" both give "v0.1.0".
func Tag(version string) string {
	return "v" + strings.TrimPrefix(version, "v")
}

// Latest returns the newest Release's tag. It reads the redirect GitHub sends from
// /releases/latest, which avoids the API and its rate limit.
func (s Source) Latest(ctx context.Context) (string, error) {
	client := *s.Client
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := s.get(ctx, &client, s.releaseURL("latest"))
	if err != nil {
		return "", err
	}
	resp.Body.Close()
	loc, err := resp.Location()
	if err != nil {
		return "", errors.New("can't find the latest release: GitHub sent no redirect")
	}
	dir, tag := path.Split(loc.Path)
	if !strings.HasSuffix(dir, "/releases/tag/") || tag == "" {
		return "", errors.New("no release published yet")
	}
	return tag, nil
}

// Fetch downloads the Release archive for goos/goarch, checks it against checksums.txt and
// returns the squirrel binary inside it.
func (s Source) Fetch(ctx context.Context, tag, goos, goarch string) ([]byte, error) {
	sums, err := s.download(ctx, tag, "checksums.txt")
	if errors.Is(err, errNotFound) {
		return nil, fmt.Errorf("no release %s", tag)
	}
	if err != nil {
		return nil, err
	}
	name := fmt.Sprintf("squirrel_%s_%s_%s.tar.gz", strings.TrimPrefix(tag, "v"), goos, goarch)
	want, ok := checksumFor(sums, name)
	if !ok {
		return nil, fmt.Errorf("no release for %s/%s", goos, goarch)
	}
	archive, err := s.download(ctx, tag, name)
	if err != nil {
		return nil, err
	}
	got := sha256.Sum256(archive)
	if hex.EncodeToString(got[:]) != want {
		return nil, fmt.Errorf("%s does not match its checksum", name)
	}
	bin, err := binaryFrom(archive)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	return bin, nil
}

var errNotFound = errors.New("not found")

func (s Source) releaseURL(rest string) string {
	return s.BaseURL + "/" + s.Repo + "/releases/" + rest
}

func (s Source) download(ctx context.Context, tag, file string) ([]byte, error) {
	resp, err := s.get(ctx, s.Client, s.releaseURL("download/"+url.PathEscape(tag)+"/"+file))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusNotFound:
		return nil, errNotFound
	case resp.StatusCode != http.StatusOK:
		return nil, fmt.Errorf("downloading %s: %s", file, resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("downloading %s: %w", file, err)
	}
	return body, nil
}

func (s Source) get(ctx context.Context, client *http.Client, u string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("can't reach GitHub: %w", err)
	}
	return resp, nil
}

// checksumFor finds name's sha256 in a sha256sum-format file.
func checksumFor(sums []byte, name string) (string, bool) {
	sc := bufio.NewScanner(bytes.NewReader(sums))
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		if len(f) == 2 && f[1] == name {
			return strings.ToLower(f[0]), true
		}
	}
	return "", false
}

func binaryFrom(archive []byte) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, err
	}
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return nil, errors.New("no squirrel binary inside")
		}
		if err != nil {
			return nil, err
		}
		if h.Typeflag == tar.TypeReg && h.Name == "squirrel" {
			return io.ReadAll(tr)
		}
	}
}
