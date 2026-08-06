package service

import (
	"errors"
	"strconv"
	"strings"
)

var ErrInvalidByteRange = errors.New("invalid single byte range")

type ByteRange struct {
	Start int64
	End   int64
}

func (r ByteRange) Length() int64 {
	if r.End < r.Start {
		return 0
	}
	return r.End - r.Start + 1
}

// ParseSingleByteRange parses one RFC 7233 byte range. The bool result is
// false when the request has no Range header and the whole file is requested.
func ParseSingleByteRange(header string, size int64) (ByteRange, bool, error) {
	if strings.TrimSpace(header) == "" {
		return ByteRange{Start: 0, End: size - 1}, false, nil
	}
	if size <= 0 {
		return ByteRange{}, true, ErrInvalidByteRange
	}
	header = strings.TrimSpace(header)
	if !strings.HasPrefix(header, "bytes=") {
		return ByteRange{}, true, ErrInvalidByteRange
	}
	specification := strings.TrimSpace(strings.TrimPrefix(header, "bytes="))
	if specification == "" || strings.Contains(specification, ",") {
		return ByteRange{}, true, ErrInvalidByteRange
	}
	parts := strings.Split(specification, "-")
	if len(parts) != 2 {
		return ByteRange{}, true, ErrInvalidByteRange
	}
	startText := strings.TrimSpace(parts[0])
	endText := strings.TrimSpace(parts[1])
	if startText == "" {
		suffixLength, err := parsePositiveRangeNumber(endText)
		if err != nil {
			return ByteRange{}, true, ErrInvalidByteRange
		}
		start := size - suffixLength
		if start < 0 {
			start = 0
		}
		return ByteRange{Start: start, End: size - 1}, true, nil
	}
	start, err := parseRangeNumber(startText)
	if err != nil || start >= size {
		return ByteRange{}, true, ErrInvalidByteRange
	}
	end := size - 1
	if endText != "" {
		end, err = parseRangeNumber(endText)
		if err != nil || end < start {
			return ByteRange{}, true, ErrInvalidByteRange
		}
		if end >= size {
			end = size - 1
		}
	}
	return ByteRange{Start: start, End: end}, true, nil
}

func parseRangeNumber(value string) (int64, error) {
	if value == "" || strings.HasPrefix(value, "+") || strings.HasPrefix(value, "-") {
		return 0, ErrInvalidByteRange
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed < 0 {
		return 0, ErrInvalidByteRange
	}
	return parsed, nil
}

func parsePositiveRangeNumber(value string) (int64, error) {
	parsed, err := parseRangeNumber(value)
	if err != nil || parsed <= 0 {
		return 0, ErrInvalidByteRange
	}
	return parsed, nil
}
