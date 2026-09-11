package api

import (
	"errors"
	"strconv"
)

// parsePositiveInt parses a non-negative integer, rejecting anything else.
func parsePositiveInt(raw string) (int, error) {
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, err
	}
	if n < 0 {
		return 0, errors.New("must not be negative")
	}
	return n, nil
}
