package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"runtime"
	"runtime/debug"
	"time"

	"github.com/iomz/radikron/internal/update"
)

// selfUpdate is "disabled" in builds whose binary is owned by an image or a
// package manager; set it with -ldflags "-X main.selfUpdate=disabled".
var selfUpdate = "enabled"

const startupCheckTimeout = 30 * time.Second

// releaseSource finds and installs releases; *update.Client implements it.
type releaseSource interface {
	Latest(ctx context.Context) (*update.Release, error)
	Apply(ctx context.Context, rel *update.Release, exePath string) error
}

// currentVersion returns the goreleaser-injected version, falling back to the
// module version recorded by "go install ...@vX.Y.Z".
func currentVersion() string {
	if version != "(devel)" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return version
}

// runUpdateCommand handles -check-update and -self-update.
func runUpdateCommand(apply bool, current string) error {
	src, err := update.NewClient(current)
	if err != nil {
		return err
	}
	ctx := context.Background()
	if !apply {
		return checkUpdate(ctx, os.Stdout, src, current)
	}
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate executable: %w", err)
	}
	return runSelfUpdate(ctx, os.Stdout, src, current, exePath)
}

// checkUpdate prints whether a newer release exists.
func checkUpdate(ctx context.Context, w io.Writer, src releaseSource, current string) error {
	rel, err := src.Latest(ctx)
	if err != nil {
		return err
	}
	newer, err := update.IsNewer(current, rel.Version)
	if err != nil {
		fmt.Fprintf(w, "radikron %s is not a release build; the latest release is %s\n  %s\n", current, rel.Version, rel.URL)
		return nil
	}
	if !newer {
		fmt.Fprintf(w, "radikron %s is up to date\n", current)
		return nil
	}
	fmt.Fprintf(w, "radikron %s is available (current: %s)\n  %s\n", rel.Version, current, rel.URL)
	if selfUpdate == "disabled" {
		fmt.Fprintln(w, "Self-update is disabled in this build; update it the way you installed it.")
	} else {
		fmt.Fprintln(w, "Run `radikron -self-update` to install it.")
	}
	return nil
}

// runSelfUpdate replaces the executable at exePath with the latest release.
func runSelfUpdate(ctx context.Context, w io.Writer, src releaseSource, current, exePath string) error {
	if selfUpdate == "disabled" {
		return errors.New("self-update is disabled in this build; update it the way you installed it")
	}
	rel, err := src.Latest(ctx)
	if err != nil {
		return err
	}
	newer, err := update.IsNewer(current, rel.Version)
	if err != nil {
		return fmt.Errorf("cannot self-update radikron %s: not a release build", current)
	}
	if !newer {
		fmt.Fprintf(w, "radikron %s is up to date\n", current)
		return nil
	}
	if err := src.Apply(ctx, rel, exePath); err != nil {
		if errors.Is(err, update.ErrAssetNotReady) {
			return fmt.Errorf("radikron %s is published, but its signed archive for %s/%s is not uploaded yet; try again in a few minutes",
				rel.Version, runtime.GOOS, runtime.GOARCH)
		}
		return err
	}
	fmt.Fprintf(w, "Updated radikron %s to %s at %s\nRestart radikron to use the new version.\n", current, rel.Version, exePath)
	return nil
}

// logAvailableUpdate logs a newer release once at startup. It never installs
// anything, and logs failures only in debug mode so offline runs stay quiet.
func logAvailableUpdate(ctx context.Context, src releaseSource, current string, debugMode bool) {
	ctx, cancel := context.WithTimeout(ctx, startupCheckTimeout)
	defer cancel()
	rel, err := src.Latest(ctx)
	if err != nil {
		if debugMode {
			log.Printf("update check failed: %v", err)
		}
		return
	}
	if newer, err := update.IsNewer(current, rel.Version); err == nil && newer {
		log.Printf("radikron %s is available (current: %s): %s", rel.Version, current, rel.URL)
	}
}
