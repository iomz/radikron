// Package releasekey provides the public key that verifies signed release assets.
//
// The release workflow signs every asset with the matching private key and
// publishes the DER-encoded ECDSA P-256 signature over the asset's SHA-256
// digest as "<asset>.sig". Updaters must verify against this embedded key and
// never against a key supplied by the release source.
package releasekey

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/x509"
	_ "embed"
	"encoding/pem"
	"errors"
	"fmt"
)

// PublicKeyPEM is the PKIX public key in PEM form.
//
//go:embed public_key.pem
var PublicKeyPEM []byte

// PublicKey parses PublicKeyPEM as an ECDSA P-256 public key.
func PublicKey() (*ecdsa.PublicKey, error) {
	block, _ := pem.Decode(PublicKeyPEM)
	if block == nil || block.Type != "PUBLIC KEY" {
		return nil, errors.New("releasekey: no PEM public key block")
	}
	parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("releasekey: parse public key: %w", err)
	}
	key, ok := parsed.(*ecdsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("releasekey: public key has type %T, want ECDSA", parsed)
	}
	if key.Curve != elliptic.P256() {
		return nil, fmt.Errorf("releasekey: public key uses %s, want P-256", key.Curve.Params().Name)
	}
	return key, nil
}
