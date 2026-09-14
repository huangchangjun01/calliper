package handlers

import (
	"fmt"
	"strconv"
)

// parsePageLimit parses limit/offset query strings with safe clamping.
//
// limitStr is clamped to [1,100]; offsetStr is clamped to >= 0. An empty,
// unparseable, or out-of-range value falls back to a safe default rather than
// propagating a non-numeric value into the SQL LIMIT/OFFSET clause. The
// returned error is informational; callers can ignore it and use the returned
// (already clamped) values. defaultLimit is used when limitStr is empty or
// invalid and is itself constrained to [1,100].
func parsePageLimit(limitStr, offsetStr string, defaultLimit int) (int, int, error) {
	if defaultLimit < 1 || defaultLimit > 100 {
		defaultLimit = 20
	}
	limit := defaultLimit
	offset := 0

	if limitStr != "" {
		l, err := strconv.Atoi(limitStr)
		if err != nil {
			return defaultLimit, 0, fmt.Errorf("invalid limit %q", limitStr)
		}
		if l < 1 || l > 100 {
			return defaultLimit, 0, fmt.Errorf("limit %d out of range [1,100]", l)
		}
		limit = l
	}

	if offsetStr != "" {
		o, err := strconv.Atoi(offsetStr)
		if err != nil || o < 0 {
			return limit, 0, fmt.Errorf("invalid offset %q", offsetStr)
		}
		offset = o
	}

	return limit, offset, nil
}
