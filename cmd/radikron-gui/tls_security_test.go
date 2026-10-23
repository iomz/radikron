package main

import (
	"context"
	"crypto/tls"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/wailsapp/wails/v3/pkg/updater"
	"github.com/wailsapp/wails/v3/pkg/updater/providers/github"
)

// Check both GUI updater HTTP paths without downloading or installing updates.
func TestTLSVerificationEMudhra(t *testing.T) {
	if os.Getenv("RADIKRON_TLS_INTEGRATION") != "1" {
		t.Skip("set RADIKRON_TLS_INTEGRATION=1 to run live certificate checks")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	for _, endpoint := range []string{
		"https://dv-g3b-emudhra.we-have-a-problem.com",
		"https://dv-g3b-aaa-emudhra.we-have-a-problem.com",
	} {
		provider, err := github.New(github.Config{Repository: releaseRepository, BaseURL: endpoint})
		if err != nil {
			t.Fatal(err)
		}
		signed := newSignedProvider(provider, nil)
		checks := map[string]func() error{
			"release-metadata": func() error {
				_, err := provider.Check(ctx, updater.CheckRequest{Platform: "darwin", Arch: "arm64"})
				return err
			},
			"signatures": func() error {
				_, err := signed.fetchSignatures(ctx, endpoint+"/")
				return err
			},
		}
		for name, check := range checks {
			var verificationError *tls.CertificateVerificationError
			err := check()
			if !errors.As(err, &verificationError) {
				t.Errorf("%s %s: expected certificate verification failure, got %v", name, endpoint, err)
				continue
			}
			t.Logf("%s %s: rejected: %v", name, endpoint, verificationError)
		}
	}
	provider, err := github.New(github.Config{Repository: releaseRepository})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.Check(ctx, updater.CheckRequest{Platform: "darwin", Arch: "arm64"}); err != nil {
		t.Fatalf("normal GitHub HTTPS failed: %v", err)
	}
	signed := newSignedProvider(provider, nil)
	if _, err := signed.fetchSignatures(ctx, "https://radiko.jp/area"); err != nil {
		t.Fatalf("normal signature-client HTTPS failed: %v", err)
	}
	t.Log("normal GitHub and Radiko HTTPS succeeded")
}
