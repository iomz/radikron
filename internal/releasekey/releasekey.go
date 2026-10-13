// Package releasekey provides the public key that verifies signed release assets.
//
// The release workflow signs every asset with the matching private key and
// lists the DER-encoded ECDSA P-256 signature over each asset's SHA-256
// digest in the SignaturesAsset release asset. Updaters must verify against
// this embedded key and never against a key supplied by the release source.
package releasekey

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/x509"
	_ "embed"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"strings"
)

// SignaturesAsset is the release asset that lists the signature of every
// other asset, one "<base64 signature>  <asset name>" line each, in the
// layout of checksums.txt. It needs no signature of its own: a changed line
// fails verification against the public key.
const SignaturesAsset = "signatures.txt"

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

// Signature returns the DER-encoded signature of the asset named name from
// the contents of SignaturesAsset.
func Signature(signatures []byte, name string) ([]byte, error) {
	for line := range strings.Lines(string(signatures)) {
		sig, asset, ok := strings.Cut(strings.TrimRight(line, "\r\n"), "  ")
		if !ok || asset != name {
			continue
		}
		der, err := base64.StdEncoding.DecodeString(sig)
		if err != nil || len(der) == 0 {
			return nil, fmt.Errorf("releasekey: malformed signature for %s", name)
		}
		return der, nil
	}
	return nil, fmt.Errorf("releasekey: %s lists no signature for %s", SignaturesAsset, name)
}
