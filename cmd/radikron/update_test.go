package main

import (
	"bytes"
	"context"
	"errors"
	"log"
	"strings"
	"testing"

	"github.com/iomz/radikron/internal/update"
)

type fakeReleaseSource struct {
	release    *update.Release
	latestErr  error
	applyErr   error
	appliedTo  string
	applyCalls int
}

func (f *fakeReleaseSource) Latest(context.Context) (*update.Release, error) {
	return f.release, f.latestErr
}

func (f *fakeReleaseSource) Apply(_ context.Context, _ *update.Release, exePath string) error {
	f.applyCalls++
	f.appliedTo = exePath
	return f.applyErr
}

func newFakeSource(v string) *fakeReleaseSource {
	return &fakeReleaseSource{release: &update.Release{
		Version: v,
		URL:     "https://github.com/iomz/radikron/releases/tag/v" + v,
	}}
}

func setSelfUpdate(t *testing.T, v string) {
	t.Helper()
	old := selfUpdate
	selfUpdate = v
	t.Cleanup(func() { selfUpdate = old })
}

func TestCheckUpdate(t *testing.T) {
	tests := []struct {
		name, current, latest, mode string
		want                        []string
	}{
		{"newer", "0.10.0", "0.11.0", "enabled", []string{"0.11.0 is available (current: 0.10.0)", "radikron --self-update"}},
		{"newer, disabled build", "0.10.0", "0.11.0", "disabled", []string{"is available", "Self-update is disabled"}},
		{"up to date", "0.11.0", "0.11.0", "enabled", []string{"0.11.0 is up to date"}},
		{"devel build", "(devel)", "0.11.0", "enabled", []string{"not a release build; the latest release is 0.11.0"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setSelfUpdate(t, tt.mode)
			var out bytes.Buffer
			if err := checkUpdate(context.Background(), &out, newFakeSource(tt.latest), tt.current); err != nil {
				t.Fatal(err)
			}
			for _, w := range tt.want {
				if !strings.Contains(out.String(), w) {
					t.Errorf("output %q does not contain %q", out.String(), w)
				}
			}
		})
	}
}

func TestRunSelfUpdate(t *testing.T) {
	t.Run("installs newer release", func(t *testing.T) {
		setSelfUpdate(t, "enabled")
		src := newFakeSource("0.11.0")
		var out bytes.Buffer
		if err := runSelfUpdate(context.Background(), &out, src, "0.10.0", "/opt/radikron"); err != nil {
			t.Fatal(err)
		}
		if src.appliedTo != "/opt/radikron" {
			t.Errorf("applied to %q, want /opt/radikron", src.appliedTo)
		}
		if !strings.Contains(out.String(), "Restart radikron") {
			t.Errorf("output %q does not ask for a restart", out.String())
		}
	})

	t.Run("up to date does nothing", func(t *testing.T) {
		setSelfUpdate(t, "enabled")
		src := newFakeSource("0.11.0")
		var out bytes.Buffer
		if err := runSelfUpdate(context.Background(), &out, src, "0.11.0", "/opt/radikron"); err != nil {
			t.Fatal(err)
		}
		if src.applyCalls != 0 {
			t.Errorf("Apply called %d times for an up-to-date build", src.applyCalls)
		}
	})

	t.Run("disabled build refuses", func(t *testing.T) {
		setSelfUpdate(t, "disabled")
		src := newFakeSource("0.11.0")
		err := runSelfUpdate(context.Background(), &bytes.Buffer{}, src, "0.10.0", "/opt/radikron")
		if err == nil || src.applyCalls != 0 {
			t.Fatalf("disabled build: err = %v, Apply calls = %d", err, src.applyCalls)
		}
	})

	t.Run("devel build refuses", func(t *testing.T) {
		setSelfUpdate(t, "enabled")
		src := newFakeSource("0.11.0")
		err := runSelfUpdate(context.Background(), &bytes.Buffer{}, src, "(devel)", "/opt/radikron")
		if err == nil || src.applyCalls != 0 {
			t.Fatalf("devel build: err = %v, Apply calls = %d", err, src.applyCalls)
		}
	})

	t.Run("asset not ready", func(t *testing.T) {
		setSelfUpdate(t, "enabled")
		src := newFakeSource("0.11.0")
		src.applyErr = update.ErrAssetNotReady
		err := runSelfUpdate(context.Background(), &bytes.Buffer{}, src, "0.10.0", "/opt/radikron")
		if err == nil || !strings.Contains(err.Error(), "try again in a few minutes") {
			t.Fatalf("err = %v, want a retry hint", err)
		}
	})
}

func TestLogAvailableUpdate(t *testing.T) {
	var buf bytes.Buffer
	orig := log.Writer()
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(orig) })

	logAvailableUpdate(context.Background(), newFakeSource("0.11.0"), "0.10.0", false)
	if !strings.Contains(buf.String(), "0.11.0 is available") {
		t.Errorf("log %q does not announce the update", buf.String())
	}

	buf.Reset()
	failing := &fakeReleaseSource{latestErr: errors.New("offline")}
	logAvailableUpdate(context.Background(), failing, "0.10.0", false)
	if buf.Len() != 0 {
		t.Errorf("non-debug run logged a failed check: %q", buf.String())
	}
	logAvailableUpdate(context.Background(), failing, "0.10.0", true)
	if !strings.Contains(buf.String(), "update check failed: offline") {
		t.Errorf("debug run did not log the failed check: %q", buf.String())
	}
}
