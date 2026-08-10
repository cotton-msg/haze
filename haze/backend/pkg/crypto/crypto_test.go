package crypto

import "testing"

func TestNewRejectsEmptyKey(t *testing.T) {
	if _, err := New(""); err == nil {
		t.Fatal("expected error for empty key")
	}
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	box, err := New("super-secret-key")
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	plain := "push-auth-secret-123"
	enc, err := box.Encrypt(plain)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if enc == plain {
		t.Fatal("encrypted output must differ from plaintext")
	}

	dec, err := box.Decrypt(enc)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if dec != plain {
		t.Fatalf("round trip mismatch: got %q want %q", dec, plain)
	}
}

func TestEncryptIsRandomized(t *testing.T) {
	box, _ := New("another-key")
	a, _ := box.Encrypt("same-value")
	b, _ := box.Encrypt("same-value")
	if a == b {
		t.Fatal("encryption must use a fresh nonce per call")
	}
}

func TestDecryptWrongKeyFails(t *testing.T) {
	boxA, _ := New("key-a")
	boxB, _ := New("key-b")

	enc, err := boxA.Encrypt("value")
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if _, err := boxB.Decrypt(enc); err == nil {
		t.Fatal("expected decrypt failure with wrong key")
	}
}

func TestDecryptGarbageFails(t *testing.T) {
	box, _ := New("key-c")
	if _, err := box.Decrypt("not-base64!!!"); err == nil {
		t.Fatal("expected error for invalid base64")
	}
	if _, err := box.Decrypt("aGVsbG8="); err == nil {
		t.Fatal("expected error for too-short ciphertext")
	}
}
