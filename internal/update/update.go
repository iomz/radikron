// Package update checks GitHub Releases for a newer radikron CLI and replaces
// the running executable with a verified release build.
//
// Every downloaded archive must carry a "<asset>.sig" signature that verifies
// against the key embedded by internal/releasekey; an unsigned or tampered
// archive is never installed.
package update

import (
	"context"
	"crypto/ecdsa"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strings"
	"time"

	"github.com/iomz/radikron/internal/releasekey"
)

const (
	defaultAPIBaseURL = "https://api.github.com"
	defaultRepository = "iomz/radikron"
	httpTimeout       = 5 * time.Minute

	// Limits guard against a hostile or broken server streaming forever.
	maxMetadataSize = 1 << 20
	maxArchiveSize  = 256 << 20
	maxSignatureLen = 1 << 10
)

// ErrAssetNotReady means the newest release has no archive for this
// platform yet, typically while the release workflow is still uploading.
var ErrAssetNotReady = errors.New("update: release has no signed archive for this platform yet")

// Release is the newest published release and its archive for this platform.
type Release struct {
	Version string // without the leading "v"
	URL     string // release page

	archive   asset
	signature asset
}

type asset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
}

// Client talks to the GitHub Releases API.
type Client struct {
	HTTP       *http.Client
	APIBaseURL string
	Repository string
	GOOS       string
	GOARCH     string
	PublicKey  *ecdsa.PublicKey
	UserAgent  string
}

// NewClient returns a Client for the official repository, the running
// platform, and the embedded release key.
func NewClient(currentVersion string) (*Client, error) {
	key, err := releasekey.PublicKey()
	if err != nil {
		return nil, err
	}
	return &Client{
		HTTP:       &http.Client{Timeout: httpTimeout},
		APIBaseURL: defaultAPIBaseURL,
		Repository: defaultRepository,
		GOOS:       runtime.GOOS,
		GOARCH:     runtime.GOARCH,
		PublicKey:  key,
		UserAgent:  "radikron/" + currentVersion,
	}, nil
}

// ArchiveName returns the goreleaser archive name for a platform, mirroring
// the name_template in .goreleaser.yml.
func ArchiveName(goos, goarch string) string {
	arch := goarch
	switch goarch {
	case "amd64":
		arch = "x86_64"
	case "386":
		arch = "i386"
	}
	ext := ".tar.gz"
	if goos == "windows" {
		ext = ".zip"
	}
	if goos != "" {
		goos = strings.ToUpper(goos[:1]) + goos[1:]
	}
	return "radikron_" + goos + "_" + arch + ext
}

// Latest returns the newest published release. GitHub's "latest" endpoint
// already excludes drafts and prereleases.
func (c *Client) Latest(ctx context.Context) (*Release, error) {
	url := fmt.Sprintf("%s/repos/%s/releases/latest", c.APIBaseURL, c.Repository)
	body, err := c.get(ctx, url, "application/vnd.github+json", maxMetadataSize)
	if err != nil {
		return nil, err
	}
	var payload struct {
		TagName string  `json:"tag_name"`
		HTMLURL string  `json:"html_url"`
		Assets  []asset `json:"assets"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("update: decode release: %w", err)
	}
	if _, err := parseVersion(payload.TagName); err != nil {
		return nil, err
	}
	rel := &Release{
		Version: strings.TrimPrefix(payload.TagName, "v"),
		URL:     payload.HTMLURL,
	}
	name := ArchiveName(c.GOOS, c.GOARCH)
	for _, a := range payload.Assets {
		switch a.Name {
		case name:
			rel.archive = a
		case name + ".sig":
			rel.signature = a
		}
	}
	return rel, nil
}

// Apply downloads the release archive for this platform, verifies its
// signature, and replaces the executable at exePath with the binary inside.
func (c *Client) Apply(ctx context.Context, rel *Release, exePath string) error {
	if rel.archive.URL == "" || rel.signature.URL == "" {
		return ErrAssetNotReady
	}
	archive, err := c.get(ctx, rel.archive.URL, "application/octet-stream", maxArchiveSize)
	if err != nil {
		return err
	}
	sig, err := c.get(ctx, rel.signature.URL, "application/octet-stream", maxSignatureLen)
	if err != nil {
		return err
	}
	digest := sha256.Sum256(archive)
	if !ecdsa.VerifyASN1(c.PublicKey, digest[:], sig) {
		return fmt.Errorf("update: signature of %s did not verify; refusing to install", rel.archive.Name)
	}
	binary, err := extractBinary(rel.archive.Name, archive, binaryName(c.GOOS))
	if err != nil {
		return err
	}
	return replaceExecutable(exePath, binary, c.GOOS)
}

func binaryName(goos string) string {
	if goos == "windows" {
		return "radikron.exe"
	}
	return "radikron"
}

// get fetches url and fails when the body is larger than limit.
func (c *Client) get(ctx context.Context, url, accept string, limit int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", accept)
	req.Header.Set("User-Agent", c.UserAgent)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("update: fetch %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("update: fetch %s: HTTP %d", url, resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, fmt.Errorf("update: read %s: %w", url, err)
	}
	if int64(len(body)) > limit {
		return nil, fmt.Errorf("update: %s is larger than %d bytes", url, limit)
	}
	return body, nil
}
