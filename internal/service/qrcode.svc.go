package service

import (
	"fmt"
	"strings"

	"github.com/skip2/go-qrcode"
)

func QRCodeSVG(content string) (string, error) {
	if strings.TrimSpace(content) == "" {
		return "", fmt.Errorf("qr content is required")
	}
	code, err := qrcode.New(content, qrcode.Medium)
	if err != nil {
		return "", fmt.Errorf("generate qr code: %w", err)
	}
	bitmap := code.Bitmap()
	if len(bitmap) == 0 || len(bitmap[0]) == 0 {
		return "", fmt.Errorf("qr bitmap is empty")
	}
	const moduleSize = 8
	width := len(bitmap[0]) * moduleSize
	height := len(bitmap) * moduleSize
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d" width="%d" height="%d" shape-rendering="crispEdges"><rect width="100%%" height="100%%" fill="#fff"/>`, width, height, width, height))
	for y, row := range bitmap {
		if len(row) != len(bitmap[0]) {
			return "", fmt.Errorf("qr bitmap row size mismatch")
		}
		for x, dark := range row {
			if dark {
				builder.WriteString(fmt.Sprintf(`<rect x="%d" y="%d" width="%d" height="%d" fill="#000"/>`, x*moduleSize, y*moduleSize, moduleSize, moduleSize))
			}
		}
	}
	builder.WriteString("</svg>")
	return builder.String(), nil
}
