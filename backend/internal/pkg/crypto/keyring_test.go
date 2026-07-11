package crypto_test

import (
	"testing"

	"github.com/trademind-ai/trademind/backend/internal/pkg/crypto"
)

func TestKeyRingEncryptDecrypt(t *testing.T) {
	key := "0123456789abcdef0123456789abcdef"
	kr, err := crypto.NewKeyRing("v1", key, "")
	if err != nil {
		t.Fatal(err)
	}
	ct, err := kr.Encrypt([]byte("secret-value"))
	if err != nil {
		t.Fatal(err)
	}
	if !crypto.IsEncrypted(ct) {
		t.Fatal("expected versioned ciphertext")
	}
	pt, err := kr.Decrypt(ct)
	if err != nil {
		t.Fatal(err)
	}
	if string(pt) != "secret-value" {
		t.Fatalf("got %q", pt)
	}
}
