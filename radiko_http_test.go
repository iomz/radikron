package radikron

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCurrentAreaID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<html><span id="JP13">Tokyo</span></html>`))
	}))
	defer server.Close()

	got, err := currentAreaIDWithClient(server.Client(), server.URL)
	if err != nil {
		t.Fatal(err)
	}
	if got != "JP13" {
		t.Fatalf("CurrentAreaID() = %q, want JP13", got)
	}
}

func TestNewRadikoHTTPClientDefaults(t *testing.T) {
	got, err := NewRadikoHTTPClient()
	if err != nil {
		t.Fatal(err)
	}
	if got.Transport != http.DefaultTransport || got.Timeout != radikoHTTPTimeout {
		t.Fatalf("client settings = (%v, %v), want (%v, %v)", got.Transport, got.Timeout, http.DefaultTransport, radikoHTTPTimeout)
	}
	if got.Jar == nil {
		t.Fatal("new client has no cookie jar")
	}
}

func TestTokyoTimezoneAvailable(t *testing.T) {
	location, err := time.LoadLocation(TZTokyo)
	if err != nil {
		t.Fatal(err)
	}
	_, offset := time.Date(2026, time.January, 1, 0, 0, 0, 0, location).Zone()
	if offset != 9*60*60 {
		t.Fatalf("Tokyo UTC offset = %d, want %d", offset, 9*60*60)
	}
}
