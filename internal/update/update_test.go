package update

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

type testFile struct {
	name string
	body []byte
}

func makeTarGz(t *testing.T, files ...testFile) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, f := range files {
		hdr := &tar.Header{Name: f.name, Mode: 0o755, Size: int64(len(f.body)), Typeflag: tar.TypeReg}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(f.body); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func makeZip(t *testing.T, files ...testFile) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, f := range files {
		w, err := zw.Create(f.name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(f.body); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func newKey(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return key
}

// signatures returns a signatures.txt that signs each named asset with key.
func signatures(t *testing.T, key *ecdsa.PrivateKey, assets map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	for name, data := range assets {
		digest := sha256.Sum256(data)
		sig, err := ecdsa.SignASN1(rand.Reader, key, digest[:])
		if err != nil {
			t.Fatal(err)
		}
		fmt.Fprintf(&buf, "%s  %s\n", base64.StdEncoding.EncodeToString(sig), name)
	}
	return buf.Bytes()
}

// fakeGitHub serves a "latest release" with the given assets.
func fakeGitHub(t *testing.T, tag string, assets map[string][]byte) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	type apiAsset struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
	}
	var list []apiAsset
	for name, body := range assets {
		list = append(list, apiAsset{Name: name, URL: srv.URL + "/download/" + name})
		mux.HandleFunc("/download/"+name, func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write(body)
		})
	}
	mux.HandleFunc("/repos/iomz/radikron/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			http.Error(w, "missing User-Agent", http.StatusForbidden)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"tag_name": tag,
			"html_url": "https://github.com/iomz/radikron/releases/tag/" + tag,
			"assets":   list,
		})
	})
	return srv
}

func testClient(srv *httptest.Server, key *ecdsa.PrivateKey, goos string) *Client {
	return &Client{
		HTTP:       srv.Client(),
		APIBaseURL: srv.URL,
		Repository: "iomz/radikron",
		GOOS:       goos,
		GOARCH:     "amd64",
		PublicKey:  &key.PublicKey,
		UserAgent:  "radikron/test",
	}
}

func writeExe(t *testing.T, name string) string {
	t.Helper()
	exe := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(exe, []byte("old binary"), 0o755); err != nil { //nolint:gosec // needs exec bit
		t.Fatal(err)
	}
	return exe
}

func readFile(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestArchiveName(t *testing.T) {
	tests := []struct{ goos, goarch, want string }{
		{"darwin", "arm64", "radikron_Darwin_arm64.tar.gz"},
		{"darwin", "amd64", "radikron_Darwin_x86_64.tar.gz"},
		{"linux", "arm64", "radikron_Linux_arm64.tar.gz"},
		{"windows", "amd64", "radikron_Windows_x86_64.zip"},
		{"windows", "arm64", "radikron_Windows_arm64.zip"},
	}
	for _, tt := range tests {
		if got := ArchiveName(tt.goos, tt.goarch); got != tt.want {
			t.Errorf("ArchiveName(%q, %q) = %q, want %q", tt.goos, tt.goarch, got, tt.want)
		}
	}
}

func TestLatestAndApplyTarGz(t *testing.T) {
	key := newKey(t)
	archive := makeTarGz(t,
		testFile{"README.md", []byte("readme")},
		testFile{"radikron", []byte("new binary")},
	)
	name := ArchiveName("linux", "amd64")
	srv := fakeGitHub(t, "v0.11.0", map[string][]byte{
		name:             archive,
		"signatures.txt": signatures(t, key, map[string][]byte{name: archive}),
		// GUI assets in the same release must not be picked up.
		"radikron-gui_windows_amd64.exe": []byte("gui"),
	})
	c := testClient(srv, key, "linux")

	rel, err := c.Latest(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if rel.Version != "0.11.0" || rel.archive.Name != name {
		t.Fatalf("Latest() = version %q archive %q", rel.Version, rel.archive.Name)
	}

	exe := writeExe(t, "radikron")
	before, err := os.Stat(exe)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Apply(context.Background(), rel, exe); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, exe); got != "new binary" {
		t.Errorf("executable = %q, want new binary", got)
	}
	after, err := os.Stat(exe)
	if err != nil {
		t.Fatal(err)
	}
	// Compare with the original mode; Windows has no Unix permission bits.
	if after.Mode().Perm() != before.Mode().Perm() {
		t.Errorf("mode = %v, want %v", after.Mode().Perm(), before.Mode().Perm())
	}
}

func TestApplyWindowsZip(t *testing.T) {
	key := newKey(t)
	archive := makeZip(t, testFile{"radikron.exe", []byte("new binary")})
	name := ArchiveName("windows", "amd64")
	srv := fakeGitHub(t, "v0.11.0", map[string][]byte{
		name:             archive,
		"signatures.txt": signatures(t, key, map[string][]byte{name: archive}),
	})
	c := testClient(srv, key, "windows")
	rel, err := c.Latest(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	exe := writeExe(t, "radikron.exe")
	// A leftover from the previous update must not block this one.
	if err := os.WriteFile(exe+".old", []byte("older"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := c.Apply(context.Background(), rel, exe); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, exe); got != "new binary" {
		t.Errorf("executable = %q, want new binary", got)
	}
	if _, err := os.Stat(exe + ".old"); !os.IsNotExist(err) {
		t.Errorf(".old file should be removed when the old binary is not running, got %v", err)
	}
}

func TestApplyRejectsWrongSignature(t *testing.T) {
	key := newKey(t)
	otherKey := newKey(t)
	archive := makeTarGz(t, testFile{"radikron", []byte("evil binary")})
	name := ArchiveName("linux", "amd64")
	srv := fakeGitHub(t, "v0.11.0", map[string][]byte{
		name:             archive,
		"signatures.txt": signatures(t, otherKey, map[string][]byte{name: archive}),
	})
	c := testClient(srv, key, "linux")
	rel, err := c.Latest(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	exe := writeExe(t, "radikron")
	if err := c.Apply(context.Background(), rel, exe); err == nil {
		t.Fatal("Apply() accepted an archive signed by another key")
	}
	if got := readFile(t, exe); got != "old binary" {
		t.Errorf("executable changed to %q after a rejected update", got)
	}
	entries, err := os.ReadDir(filepath.Dir(exe))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Errorf("rejected update left files behind: %v", entries)
	}
}

func TestApplyWithoutSignatureIsNotReady(t *testing.T) {
	key := newKey(t)
	archive := makeTarGz(t, testFile{"radikron", []byte("new binary")})
	srv := fakeGitHub(t, "v0.11.0", map[string][]byte{
		ArchiveName("linux", "amd64"): archive,
	})
	c := testClient(srv, key, "linux")
	rel, err := c.Latest(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	exe := writeExe(t, "radikron")
	if err := c.Apply(context.Background(), rel, exe); !errors.Is(err, ErrAssetNotReady) {
		t.Fatalf("Apply() error = %v, want ErrAssetNotReady", err)
	}
}

func TestApplyWithoutArchiveSignatureFails(t *testing.T) {
	key := newKey(t)
	archive := makeTarGz(t, testFile{"radikron", []byte("new binary")})
	name := ArchiveName("linux", "amd64")
	srv := fakeGitHub(t, "v0.11.0", map[string][]byte{
		name: archive,
		// signatures.txt exists but does not list this archive.
		"signatures.txt": signatures(t, key, map[string][]byte{"checksums.txt": []byte("sums")}),
	})
	c := testClient(srv, key, "linux")
	rel, err := c.Latest(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	exe := writeExe(t, "radikron")
	if err := c.Apply(context.Background(), rel, exe); err == nil {
		t.Fatal("Apply() installed an archive without a signature")
	}
	if got := readFile(t, exe); got != "old binary" {
		t.Errorf("executable changed to %q after a rejected update", got)
	}
}

func TestApplyFollowsSymlink(t *testing.T) {
	key := newKey(t)
	archive := makeTarGz(t, testFile{"radikron", []byte("new binary")})
	name := ArchiveName("linux", "amd64")
	srv := fakeGitHub(t, "v0.11.0", map[string][]byte{
		name:             archive,
		"signatures.txt": signatures(t, key, map[string][]byte{name: archive}),
	})
	c := testClient(srv, key, "linux")
	rel, err := c.Latest(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	target := writeExe(t, "radikron")
	link := filepath.Join(t.TempDir(), "radikron")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := c.Apply(context.Background(), rel, link); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, target); got != "new binary" {
		t.Errorf("symlink target = %q, want new binary", got)
	}
	if fi, err := os.Lstat(link); err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Errorf("symlink was replaced instead of its target: %v", err)
	}
}

func TestLatestRejectsNonSemverTag(t *testing.T) {
	key := newKey(t)
	srv := fakeGitHub(t, "nightly", nil)
	c := testClient(srv, key, "linux")
	if _, err := c.Latest(context.Background()); err == nil {
		t.Fatal("Latest() accepted a non-SemVer tag")
	}
}

func TestExtractBinaryIgnoresNestedPaths(t *testing.T) {
	archive := makeTarGz(t, testFile{"nested/radikron", []byte("wrong")})
	if _, err := extractBinary("x.tar.gz", archive, "radikron"); err == nil {
		t.Fatal("extractBinary() returned a nested file")
	}
}
