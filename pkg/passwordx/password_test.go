package passwordx

import "testing"

func TestHashAndCompare(t *testing.T) {
	hash, err := Hash("123456.com")
	if err != nil {
		t.Fatalf("Hash() error = %v", err)
	}
	if hash == "123456.com" {
		t.Fatalf("Hash() returned plain password")
	}
	if err := Compare(hash, "123456.com"); err != nil {
		t.Fatalf("Compare() error = %v", err)
	}
	if err := Compare(hash, "wrong"); err == nil {
		t.Fatalf("Compare() expected error for wrong password")
	}
}
