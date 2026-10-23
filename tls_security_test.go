package radikron

import (
	"crypto/tls"
	"errors"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/iomz/radikron/internal/update"
)

// TestTLSVerificationEMudhra is opt-in because it requires public network access.
// Exercise production clients, not a separate TLS implementation. DNS, timeout,
// proxy, and other network errors must never count as certificate rejection.
func TestTLSVerificationEMudhra(t *testing.T) {
	if os.Getenv("RADIKRON_TLS_INTEGRATION") != "1" {
		t.Skip("set RADIKRON_TLS_INTEGRATION=1 to run live certificate checks")
	}
	radiko, err := NewRadikoHTTPClient()
	if err != nil {
		t.Fatal(err)
	}
	updater, err := update.NewClient("tls-audit")
	if err != nil {
		t.Fatal(err)
	}
	clients := map[string]*http.Client{
		"radiko":     radiko,
		"download":   downloadingHTTPClient,
		"metadata":   http.DefaultClient,
		"cli-update": updater.HTTP,
	}
	for name, client := range clients {
		t.Run(name, func(t *testing.T) {
			// Copy only to bound test duration; transport, roots and verification
			// remain exactly those of the production client.
			bounded := *client
			bounded.Timeout = 30 * time.Second
			for _, endpoint := range []string{
				"https://dv-g3b-emudhra.we-have-a-problem.com/",
				"https://dv-g3b-aaa-emudhra.we-have-a-problem.com/",
			} {
				req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, endpoint, http.NoBody)
				if err != nil {
					t.Fatal(err)
				}
				resp, err := bounded.Do(req)
				if resp != nil {
					resp.Body.Close()
				}
				var verificationError *tls.CertificateVerificationError
				if !errors.As(err, &verificationError) {
					t.Errorf("%s: expected certificate verification failure, got %v", endpoint, err)
					continue
				}
				t.Logf("%s: rejected: %v", endpoint, verificationError)
			}
			req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://radiko.jp/area", http.NoBody)
			if err != nil {
				t.Fatal(err)
			}
			resp, err := bounded.Do(req)
			if err != nil {
				t.Fatalf("normal HTTPS failed: %v", err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("normal HTTPS returned %s", resp.Status)
			}
			t.Logf("normal HTTPS: %s, TLS handshake verified", resp.Status)
		})
	}
}
