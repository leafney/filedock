package cryptox

import "testing"

func TestAESGCMEncryptDecrypt(t *testing.T) {
	codec := NewAESGCM("unit-test-secret")
	plain := "af_test_token_123"
	ciphertext, err := codec.Encrypt(plain)
	if err != nil {
		t.Fatalf("encrypt failed: %v", err)
	}
	if ciphertext == "" {
		t.Fatal("expected ciphertext")
	}
	if ciphertext == plain {
		t.Fatal("ciphertext should differ from plain text")
	}
	decrypted, err := codec.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("decrypt failed: %v", err)
	}
	if decrypted != plain {
		t.Fatalf("expected %q, got %q", plain, decrypted)
	}
}
