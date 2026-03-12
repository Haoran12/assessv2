package handler

import (
	"strconv"
	"strings"
)

func parseOptionalUintQuery(raw string) (*uint, error) {
	text := strings.TrimSpace(raw)
	if text == "" {
		return nil, nil
	}
	parsed, err := strconv.ParseUint(text, 10, 64)
	if err != nil {
		return nil, err
	}
	value := uint(parsed)
	return &value, nil
}
