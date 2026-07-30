package ulidx

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"time"
)

const alphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

// New returns a canonical 26-character ULID using the current millisecond
// timestamp and cryptographically secure randomness.
func New() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[6:]); err != nil {
		return "", fmt.Errorf("generate ulid randomness: %w", err)
	}
	timestamp := time.Now().UnixMilli()
	if timestamp < 0 || timestamp >= 1<<48 {
		return "", fmt.Errorf("ulid timestamp is out of range")
	}
	var timestampBytes [8]byte
	binary.BigEndian.PutUint64(timestampBytes[:], uint64(timestamp))
	copy(raw[:6], timestampBytes[2:])
	return encode(raw), nil
}

func encode(raw [16]byte) string {
	var output [26]byte
	for charIndex := range output {
		value := 0
		for bitIndex := 0; bitIndex < 5; bitIndex++ {
			sourceBit := charIndex*5 + bitIndex - 2
			value <<= 1
			if sourceBit >= 0 && sourceBit < 128 {
				value |= int((raw[sourceBit/8] >> (7 - sourceBit%8)) & 1)
			}
		}
		output[charIndex] = alphabet[value]
	}
	return string(output[:])
}
