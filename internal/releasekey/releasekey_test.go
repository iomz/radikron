package releasekey

import (
	"bytes"
	"testing"
)

func TestPublicKey(t *testing.T) {
	if _, err := PublicKey(); err != nil {
		t.Fatalf("embedded release public key is unusable: %v", err)
	}
}

func TestSignature(t *testing.T) {
	// The release workflow writes one line per asset: base64 of the DER
	// signature, two spaces, and the asset name.
	signatures := []byte("AQID  radikron_Linux_x86_64.tar.gz\r\n" +
		"BAUG  radikron_Linux_x86_64.tar.gz.extra\n" +
		"!!!!  broken.zip\n" +
		"BwgJ  checksums.txt")
	tests := []struct {
		name    string
		want    []byte
		wantErr bool
	}{
		{"radikron_Linux_x86_64.tar.gz", []byte{1, 2, 3}, false},
		{"checksums.txt", []byte{7, 8, 9}, false},
		{"radikron_Linux", nil, true},
		{"broken.zip", nil, true},
		{"missing.exe", nil, true},
	}
	for _, tt := range tests {
		got, err := Signature(signatures, tt.name)
		if (err != nil) != tt.wantErr || !bytes.Equal(got, tt.want) {
			t.Errorf("Signature(%q) = %v, %v; want %v, error %v", tt.name, got, err, tt.want, tt.wantErr)
		}
	}
}
