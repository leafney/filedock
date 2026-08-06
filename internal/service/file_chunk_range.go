package service

import (
	"fmt"
	"strconv"
	"strings"
)

func ParseUploadContentRange(value string) (int64, int64, int64, error) {
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(value, "bytes ") {
		return 0, 0, 0, fmt.Errorf("upload content range must use bytes")
	}
	value = strings.TrimSpace(strings.TrimPrefix(value, "bytes "))
	segments := strings.Split(value, "/")
	if len(segments) != 2 || segments[0] == "" || segments[1] == "" {
		return 0, 0, 0, fmt.Errorf("invalid upload content range")
	}
	rangeParts := strings.Split(segments[0], "-")
	if len(rangeParts) != 2 || rangeParts[0] == "" || rangeParts[1] == "" {
		return 0, 0, 0, fmt.Errorf("invalid upload content range bounds")
	}
	start, err := strconv.ParseInt(rangeParts[0], 10, 64)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("parse upload range start: %w", err)
	}
	end, err := strconv.ParseInt(rangeParts[1], 10, 64)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("parse upload range end: %w", err)
	}
	total, err := strconv.ParseInt(segments[1], 10, 64)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("parse upload range total: %w", err)
	}
	if start < 0 || end < start || total <= 0 || end >= total {
		return 0, 0, 0, fmt.Errorf("upload content range is outside the file")
	}
	return start, end, total, nil
}
