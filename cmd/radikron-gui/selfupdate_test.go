package main

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wailsapp/wails/v3/pkg/updater"
	"github.com/wailsapp/wails/v3/pkg/updater/providers/github"
)

// releaseAssetNames mirrors the assets release.yml publishes (v0.16.0).
var releaseAssetNames = []string{
	"checksums.txt",
	"signatures.txt",
	"radikron_Darwin_arm64.tar.gz",
	"radikron_Linux_arm64.tar.gz",
	"radikron_Linux_x86_64.tar.gz",
	"radikron_Windows_x86_64.zip",
	"radikron-gui_darwin_arm64-installer.dmg",
	"radikron-gui_darwin_arm64.app.zip",
	"radikron-gui_windows_amd64.exe",
}

// signaturesFile returns a signatures.txt line for each name and signature.
func signaturesFile(sigs map[string][]byte) []byte {
	var buf bytes.Buffer
	for name, sig := range sigs {
		fmt.Fprintf(&buf, "%s  %s\n", base64.StdEncoding.EncodeToString(sig), name)
	}
	return buf.Bytes()
}

func TestGUIVersionMatchesFrontendPackage(t *testing.T) {
	version, err := guiVersion()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join("frontend", "package.json"))
	if err != nil {
		t.Fatal(err)
	}
	var pkg struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		t.Fatal(err)
	}
	if version != pkg.Version {
		t.Errorf("build/config.yml info.version %q differs from frontend/package.json version %q", version, pkg.Version)
	}
}

func TestMatchGUIAsset(t *testing.T) {
	assets := make([]github.ReleaseAsset, len(releaseAssetNames))
	for i, name := range releaseAssetNames {
		assets[i] = github.ReleaseAsset{Name: name}
	}
	tests := []struct {
		goos, goarch string
		want         string
	}{
		{"darwin", "arm64", "radikron-gui_darwin_arm64.app.zip"},
		{"darwin", "amd64", ""},
		{"windows", "amd64", "radikron-gui_windows_amd64.exe"},
		{"windows", "arm64", ""},
		{"linux", "amd64", ""},
		{"windows", "386", ""},
	}
	for _, tt := range tests {
		got := matchGUIAsset(updater.CheckRequest{Platform: tt.goos, Arch: tt.goarch}, assets)
		gotName := ""
		if got >= 0 {
			gotName = assets[got].Name
		}
		if gotName != tt.want {
			t.Errorf("%s/%s: matched %q, want %q", tt.goos, tt.goarch, gotName, tt.want)
		}
	}
}

func TestCanSelfUpdate(t *testing.T) {
	tests := []struct {
		goos, goarch, exe string
		want              bool
	}{
		{"darwin", "arm64", "/Applications/Radikron.app/Contents/MacOS/radikron-gui", true},
		{"darwin", "arm64", "/Users/me/radikron/cmd/radikron-gui/bin/radikron-gui", false},
		// Releases no longer ship the GUI for Intel Macs or Windows on ARM.
		{"darwin", "amd64", "/Applications/Radikron.app/Contents/MacOS/radikron-gui", false},
		{"windows", "amd64", `C:\Users\me\radikron-gui.exe`, true},
		{"windows", "arm64", `C:\Users\me\radikron-gui.exe`, false},
		{"linux", "amd64", "/usr/local/bin/radikron-gui", false},
	}
	for _, tt := range tests {
		if got := canSelfUpdate(tt.goos, tt.goarch, tt.exe); got != tt.want {
			t.Errorf("canSelfUpdate(%s/%s, %s) = %v, want %v", tt.goos, tt.goarch, tt.exe, got, tt.want)
		}
	}
}

// releaseServer serves a GitHub "latest release" response and its assets.
// Assets missing from files are listed but return 404 when downloaded.
func releaseServer(t *testing.T, tag string, names []string, files map[string][]byte) *httptest.Server {
	t.Helper()
	var srv *httptest.Server
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/"+releaseRepository+"/releases/latest", func(w http.ResponseWriter, _ *http.Request) {
		type asset struct {
			Name string `json:"name"`
			Size int    `json:"size"`
			URL  string `json:"browser_download_url"`
		}
		assets := make([]asset, 0, len(names))
		for _, name := range names {
			assets = append(assets, asset{Name: name, Size: len(files[name]), URL: srv.URL + "/download/" + name})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"tag_name": tag, "assets": assets})
	})
	mux.HandleFunc("/download/", func(w http.ResponseWriter, r *http.Request) {
		data, ok := files[strings.TrimPrefix(r.URL.Path, "/download/")]
		if !ok {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(data)
	})
	srv = httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func testProvider(t *testing.T, srv *httptest.Server) *signedProvider {
	t.Helper()
	gh, err := github.New(github.Config{
		Repository:   releaseRepository,
		BaseURL:      srv.URL,
		AssetMatcher: matchGUIAsset,
		HTTPClient:   srv.Client(),
	})
	if err != nil {
		t.Fatal(err)
	}
	return newSignedProvider(gh, srv.Client())
}

func TestSignedProviderAttachesSignature(t *testing.T) {
	const asset = "radikron-gui_windows_amd64.exe"
	srv := releaseServer(t, "v9.0.0", releaseAssetNames, map[string][]byte{
		asset: []byte("binary"),
		"signatures.txt": signaturesFile(map[string][]byte{
			"radikron-gui_darwin_arm64.app.zip": []byte("other"),
			asset:                               []byte("signature"),
		}),
	})
	rel, err := testProvider(t, srv).Check(context.Background(), updater.CheckRequest{
		CurrentVersion: "0.12.1", Platform: "windows", Arch: "amd64",
	})
	if err != nil {
		t.Fatal(err)
	}
	if rel == nil || rel.Version != "9.0.0" || rel.Artifact.Filename != asset {
		t.Fatalf("unexpected release %+v", rel)
	}
	if v := rel.Verification; v == nil || v.SignatureAlgo != "ecdsa-p256" || string(v.Signature) != "signature" {
		t.Errorf("unexpected verification %+v", rel.Verification)
	}
}

func TestSignedProviderRejectsMissingSignature(t *testing.T) {
	const asset = "radikron-gui_windows_amd64.exe"
	for name, files := range map[string]map[string][]byte{
		"no signatures.txt": {asset: []byte("binary")},
		"asset not listed": {
			asset:            []byte("binary"),
			"signatures.txt": signaturesFile(map[string][]byte{"checksums.txt": []byte("signature")}),
		},
	} {
		t.Run(name, func(t *testing.T) {
			srv := releaseServer(t, "v9.0.0", []string{asset, "signatures.txt"}, files)
			_, err := testProvider(t, srv).Check(context.Background(), updater.CheckRequest{
				CurrentVersion: "0.12.1", Platform: "windows", Arch: "amd64",
			})
			if err == nil || !strings.Contains(err.Error(), "signature") {
				t.Fatalf("Check error = %v, want a missing signature error", err)
			}
		})
	}
}

func TestSignedProviderUpToDate(t *testing.T) {
	srv := releaseServer(t, "v0.12.1", releaseAssetNames, nil)
	rel, err := testProvider(t, srv).Check(context.Background(), updater.CheckRequest{
		CurrentVersion: "0.12.1", Platform: "windows", Arch: "amd64",
	})
	if err != nil || rel != nil {
		t.Fatalf("Check = %+v, %v; want no release", rel, err)
	}
}

// testHost is the minimal updater.Host for driving the Updater headlessly.
type testHost struct{}

func (testHost) Emit(string, ...any) bool                              { return false }
func (testHost) OnEvent(string, func(any)) func()                      { return func() {} }
func (testHost) OpenWindow(updater.WindowOptions) updater.WindowHandle { return nil }
func (testHost) Quit()                                                 {}

// TestUpdaterVerifiesReleaseSignature runs the Wails Updater with the
// signature format the release workflow produces (`openssl dgst -sha256
// -sign`: DER ECDSA over SHA-256 of the asset) and checks that it installs a
// signed download and refuses a tampered one.
func TestUpdaterVerifiesReleaseSignature(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	publicKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der})

	const asset = "radikron-gui_windows_amd64.exe"
	binary := []byte("new radikron-gui build")
	digest := sha256.Sum256(binary)
	sig, err := ecdsa.SignASN1(rand.Reader, key, digest[:])
	if err != nil {
		t.Fatal(err)
	}

	for _, tt := range []struct {
		name    string
		served  []byte
		wantErr bool
	}{
		{"signed", binary, false},
		{"tampered", []byte("tampered radikron-gui build"), true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			srv := releaseServer(t, "v9.0.0", []string{asset, "signatures.txt"}, map[string][]byte{
				asset:            tt.served,
				"signatures.txt": signaturesFile(map[string][]byte{asset: sig}),
			})
			u := updater.New(testHost{})
			err := u.Init(updater.Config{
				CurrentVersion: "0.12.1",
				Providers:      []updater.Provider{testProvider(t, srv)},
				PublicKey:      publicKeyPEM,
				Platform:       "windows",
				Arch:           "amd64",
				Window:         updater.WindowNone,
			})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := u.Check(context.Background()); err != nil {
				t.Fatal(err)
			}
			err = u.DownloadAndInstall(context.Background())
			if path := u.DownloadedPath(); path != "" {
				t.Cleanup(func() { _ = os.RemoveAll(filepath.Dir(path)) })
			}
			if tt.wantErr {
				if err == nil || !strings.Contains(err.Error(), "signature") {
					t.Fatalf("DownloadAndInstall error = %v, want a signature error", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got := u.State(); got != updater.StateReady {
				t.Errorf("state = %s, want %s", got, updater.StateReady)
			}
			staged, err := os.ReadFile(u.DownloadedPath())
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(staged, binary) {
				t.Errorf("staged %q, want %q", staged, binary)
			}
		})
	}
}
