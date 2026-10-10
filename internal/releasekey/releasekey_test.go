package releasekey

import "testing"

func TestPublicKey(t *testing.T) {
	if _, err := PublicKey(); err != nil {
		t.Fatalf("embedded release public key is unusable: %v", err)
	}
}
