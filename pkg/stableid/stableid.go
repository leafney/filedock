package stableid

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

func Generate(prefix string, parts ...string) string {
	cleanPrefix := sanitize(prefix)

	values := make([]string, 0, len(parts))
	for _, part := range parts {
		values = append(values, strings.TrimSpace(part))
	}

	sum := sha256.Sum256([]byte(strings.Join(values, "|")))
	hash := hex.EncodeToString(sum[:])
	if cleanPrefix == "" {
		return hash[:16]
	}
	return cleanPrefix + "-" + hash[:16]
}

func sanitize(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, "_", "-")
	value = strings.ReplaceAll(value, " ", "-")
	value = strings.Trim(value, "-")
	return value
}
