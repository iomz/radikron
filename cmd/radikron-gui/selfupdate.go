package main

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/updater"
	"github.com/wailsapp/wails/v3/pkg/updater/providers/github"
	"gopkg.in/yaml.v3"

	"github.com/iomz/radikron/internal/releasekey"
)

const (
	releaseRepository = "iomz/radikron"
	guiAssetPrefix    = "radikron-gui_"

	// updateCheckInterval spaces background checks; the GUI often runs for
	// days while it monitors stations.
	updateCheckInterval = 24 * time.Hour

	signatureTimeout   = 30 * time.Second
	maxSignaturesBytes = 64 << 10
)

// buildConfig is the Wails build configuration. Its info.version is the GUI
// version: the release process bumps it before tagging, and the update
// check compares it with the latest release tag.
//
//go:embed build/config.yml
var buildConfig []byte

// guiVersion returns info.version from build/config.yml.
func guiVersion() (string, error) {
	var cfg struct {
		Info struct {
			Version string `yaml:"version"`
		} `yaml:"info"`
	}
	if err := yaml.Unmarshal(buildConfig, &cfg); err != nil {
		return "", fmt.Errorf("parse build/config.yml: %w", err)
	}
	if cfg.Info.Version == "" {
		return "", errors.New("build/config.yml has no info.version")
	}
	return cfg.Info.Version, nil
}

// guiAssetName returns the release asset that updates the GUI on goos/goarch,
// or "" when releases ship no GUI update for that platform. Releases ship the
// GUI for Apple Silicon Macs and 64-bit x86 Windows only; the names follow
// release.yml, and installer packages (-installer.dmg) are for first installs
// only.
func guiAssetName(goos, goarch string) string {
	switch goos + "/" + goarch {
	case "darwin/arm64":
		return guiAssetPrefix + goos + "_" + goarch + ".app.zip"
	case "windows/amd64":
		return guiAssetPrefix + goos + "_" + goarch + ".exe"
	}
	return ""
}

// matchGUIAsset selects the GUI update asset by exact name. The Wails default
// matcher picks the first name containing the platform and architecture,
// which would select a CLI archive such as radikron_Darwin_x86_64.tar.gz.
func matchGUIAsset(req updater.CheckRequest, assets []github.ReleaseAsset) int {
	name := guiAssetName(req.Platform, req.Arch)
	if name == "" {
		return -1
	}
	for i, a := range assets {
		if a.Name == name {
			return i
		}
	}
	return -1
}

// signedProvider adds signature verification to the Wails GitHub provider,
// which only understands checksum sidecars. The release's signatures.txt
// lists every asset's signature (ECDSA P-256 over SHA-256, DER encoded);
// Check attaches the update asset's signature so the Updater verifies the
// download against the embedded release key. A release without a signature
// is reported as an error, never as an unverified update.
type signedProvider struct {
	*github.Provider
	client *http.Client
}

func newSignedProvider(gh *github.Provider, client *http.Client) *signedProvider {
	if client == nil {
		client = &http.Client{Timeout: signatureTimeout}
	}
	return &signedProvider{Provider: gh, client: client}
}

// Check implements updater.Provider.
func (p *signedProvider) Check(ctx context.Context, req updater.CheckRequest) (*updater.Release, error) {
	rel, err := p.Provider.Check(ctx, req)
	if err != nil || rel == nil {
		return rel, err
	}
	assetURL, _ := rel.Metadata["github.asset.url"].(string)
	if assetURL == "" {
		return nil, errors.New("github release metadata has no asset URL")
	}
	// Release assets share one download directory, so signatures.txt sits
	// next to the update asset.
	signaturesURL := assetURL[:strings.LastIndex(assetURL, "/")+1] + releasekey.SignaturesAsset
	signatures, err := p.fetchSignatures(ctx, signaturesURL)
	if err != nil {
		return nil, fmt.Errorf("signature for %s: %w", rel.Artifact.Filename, err)
	}
	sig, err := releasekey.Signature(signatures, rel.Artifact.Filename)
	if err != nil {
		return nil, fmt.Errorf("signature for %s: %w", rel.Artifact.Filename, err)
	}
	rel.Verification = &updater.Verification{
		SignatureAlgo: "ecdsa-p256",
		Signature:     sig,
	}
	return rel, nil
}

func (p *signedProvider) fetchSignatures(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/octet-stream")
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		// A missing signature usually means the release workflow is still
		// uploading assets.
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	signatures, err := io.ReadAll(io.LimitReader(resp.Body, maxSignaturesBytes+1))
	if err != nil {
		return nil, err
	}
	if len(signatures) > maxSignaturesBytes {
		return nil, fmt.Errorf("%s is larger than %d bytes", releasekey.SignaturesAsset, maxSignaturesBytes)
	}
	return signatures, nil
}

// canSelfUpdate reports whether this build can replace itself with a release
// asset: releases ship GUI updates for Apple Silicon Macs and x86-64 Windows
// only, and on macOS the Updater swaps the whole .app bundle, so a bare
// binary (for example bin/radikron-gui from a development build) is not
// updated.
func canSelfUpdate(goos, goarch, exe string) bool {
	if guiAssetName(goos, goarch) == "" {
		return false
	}
	if goos == "darwin" {
		return strings.Contains(filepath.ToSlash(exe), ".app/Contents/MacOS/")
	}
	return true
}

// setupUpdater configures wailsApp.Updater against GitHub Releases. It returns
// false without an error when this build cannot update itself.
func setupUpdater(wailsApp *application.App) (bool, error) {
	exe, err := os.Executable()
	if err != nil {
		return false, fmt.Errorf("resolve executable: %w", err)
	}
	if !canSelfUpdate(runtime.GOOS, runtime.GOARCH, exe) {
		return false, nil
	}
	version, err := guiVersion()
	if err != nil {
		return false, err
	}
	// Fail early on a broken embedded key rather than at the first update.
	if _, err := releasekey.PublicKey(); err != nil {
		return false, err
	}
	gh, err := github.New(github.Config{
		Repository:   releaseRepository,
		AssetMatcher: matchGUIAsset,
	})
	if err != nil {
		return false, err
	}
	err = wailsApp.Updater.Init(updater.Config{
		CurrentVersion: version,
		Providers:      []updater.Provider{newSignedProvider(gh, nil)},
		PublicKey:      releasekey.PublicKeyPEM,
	})
	if err != nil {
		return false, err
	}
	return true, nil
}

// UpdatesSupported reports whether "Check for Updates" is available in this
// build.
func (a *App) UpdatesSupported() bool {
	return a.updatesEnabled
}

// CheckForUpdates opens the update window, which checks for a newer release
// and offers to download, install, and restart into it.
func (a *App) CheckForUpdates() {
	if !a.updatesEnabled || a.wails == nil {
		return
	}
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	go func() {
		// The window shows check and install errors to the user.
		if err := a.wails.Updater.CheckAndInstall(ctx); err != nil {
			a.logger.Warn(fmt.Sprintf("Update failed: %v", err))
		}
	}()
}

// startBackgroundUpdateChecks checks for a newer release now and then every
// updateCheckInterval until ctx ends. A found release is announced with an
// "update-available" event; nothing is downloaded until the user asks.
func (a *App) startBackgroundUpdateChecks(ctx context.Context) {
	if !a.updatesEnabled || a.wails == nil {
		return
	}
	go func() {
		ticker := time.NewTicker(updateCheckInterval)
		defer ticker.Stop()
		for {
			a.checkForUpdateQuietly(ctx)
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}

func (a *App) checkForUpdateQuietly(ctx context.Context) {
	u := a.wails.Updater
	switch u.State() {
	case updater.StateIdle, updater.StateUpToDate, updater.StateAvailable, updater.StateError:
	default:
		// Leave a check, download, or pending restart the user started alone.
		return
	}
	rel, err := u.Check(ctx)
	if err != nil {
		a.logger.Warn(fmt.Sprintf("Update check failed: %v", err))
		return
	}
	version := ""
	if rel != nil {
		version = rel.Version
	}
	a.mu.Lock()
	a.availableUpdate = version
	a.mu.Unlock()
	if rel == nil {
		return
	}
	a.logger.Info(fmt.Sprintf("Radikron %s is available", rel.Version))
	a.emit("update-available", map[string]any{"version": rel.Version})
}

// AvailableUpdate returns the newest release version found by a background
// check, or "" when none is known. The frontend reads it on load because the
// first check can finish before the page subscribes to "update-available".
func (a *App) AvailableUpdate() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.availableUpdate
}
