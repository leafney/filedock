package service

import (
	"errors"
	"testing"
)

func TestParseSingleByteRange(t *testing.T) {
	tests := []struct {
		name      string
		header    string
		size      int64
		want      ByteRange
		hasRange  bool
		wantError bool
	}{
		{name: "no range", size: 100, want: ByteRange{Start: 0, End: 99}},
		{name: "prefix", header: "bytes=0-49", size: 100, want: ByteRange{Start: 0, End: 49}, hasRange: true},
		{name: "open ended", header: "bytes=50-", size: 100, want: ByteRange{Start: 50, End: 99}, hasRange: true},
		{name: "suffix", header: "bytes=-20", size: 100, want: ByteRange{Start: 80, End: 99}, hasRange: true},
		{name: "suffix larger than file", header: "bytes=-200", size: 100, want: ByteRange{Start: 0, End: 99}, hasRange: true},
		{name: "end clamped", header: "bytes=90-200", size: 100, want: ByteRange{Start: 90, End: 99}, hasRange: true},
		{name: "spaces", header: " bytes= 10 - 20 ", size: 100, want: ByteRange{Start: 10, End: 20}, hasRange: true},
		{name: "empty file", header: "bytes=0-", size: 0, wantError: true, hasRange: true},
		{name: "multi range", header: "bytes=0-10,20-30", size: 100, wantError: true, hasRange: true},
		{name: "wrong unit", header: "items=0-10", size: 100, wantError: true, hasRange: true},
		{name: "invalid start", header: "bytes=x-10", size: 100, wantError: true, hasRange: true},
		{name: "invalid end", header: "bytes=10-x", size: 100, wantError: true, hasRange: true},
		{name: "reversed", header: "bytes=30-10", size: 100, wantError: true, hasRange: true},
		{name: "outside", header: "bytes=100-", size: 100, wantError: true, hasRange: true},
		{name: "zero suffix", header: "bytes=-0", size: 100, wantError: true, hasRange: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, hasRange, err := ParseSingleByteRange(test.header, test.size)
			if hasRange != test.hasRange {
				t.Fatalf("hasRange = %v, want %v", hasRange, test.hasRange)
			}
			if test.wantError {
				if !errors.Is(err, ErrInvalidByteRange) {
					t.Fatalf("error = %v, want ErrInvalidByteRange", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Fatalf("range = %+v, want %+v", got, test.want)
			}
		})
	}
}

func TestByteRangeLength(t *testing.T) {
	if got := (ByteRange{Start: 10, End: 19}).Length(); got != 10 {
		t.Fatalf("length = %d, want 10", got)
	}
	if got := (ByteRange{Start: 20, End: 19}).Length(); got != 0 {
		t.Fatalf("empty length = %d, want 0", got)
	}
}
