package service

import (
	"strings"
	"testing"
)

func TestQRCodeSVG(t *testing.T) {
	svg, err := QRCodeSVG("http://127.0.0.1:8195/rooms/1234")
	if err != nil {
		t.Fatalf("QRCodeSVG() error = %v", err)
	}
	if !strings.HasPrefix(svg, "<svg ") || !strings.Contains(svg, "<rect") || !strings.HasSuffix(svg, "</svg>") {
		t.Fatalf("unexpected SVG output: %q", svg[:min(len(svg), 120)])
	}
}

func min(left, right int) int {
	if left < right {
		return left
	}
	return right
}
