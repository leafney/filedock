package stableid

import "testing"

func TestGenerateStable(t *testing.T) {
	a := Generate("device", "agent-001", "token-001")
	b := Generate("device", "agent-001", "token-001")
	c := Generate("device", "agent-001", "token-002")

	if a != b {
		t.Fatalf("same input should generate same id: %q != %q", a, b)
	}
	if a == c {
		t.Fatalf("different input should generate different id")
	}
	if a[:7] != "device-" {
		t.Fatalf("unexpected prefix: %q", a)
	}
}

func TestGenerateWithoutPrefix(t *testing.T) {
	a := Generate("", "agent-001", "token-001")
	b := Generate("", "agent-001", "token-001")

	if a != b {
		t.Fatalf("same input should generate same id: %q != %q", a, b)
	}
	if len(a) != 16 {
		t.Fatalf("unexpected id length: %d", len(a))
	}
	if a == "" {
		t.Fatal("id should not be empty")
	}
}
