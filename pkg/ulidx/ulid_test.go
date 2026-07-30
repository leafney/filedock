package ulidx

import (
	"regexp"
	"testing"
)

func TestNewReturnsCanonicalULID(t *testing.T) {
	pattern := regexp.MustCompile(`^[0-9A-HJKMNP-TV-Z]{26}$`)
	first, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	second, err := New()
	if err != nil {
		t.Fatalf("second New() error = %v", err)
	}
	if !pattern.MatchString(first) || !pattern.MatchString(second) {
		t.Fatalf("invalid ULID values %q and %q", first, second)
	}
	if first == second {
		t.Fatalf("two ULIDs are equal: %q", first)
	}
}
