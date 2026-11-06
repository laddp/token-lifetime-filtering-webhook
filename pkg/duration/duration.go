package duration

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode"
)

const (
	// DefaultMaxExpiration is the default max lifetime string.
	DefaultMaxExpiration = "1y"

	secondsPerMinute = 60
	secondsPerHour   = 60 * secondsPerMinute
	secondsPerDay    = 24 * secondsPerHour
	secondsPerWeek   = 7 * secondsPerDay
	secondsPerYear   = 365 * secondsPerDay
)

var unitSeconds = map[rune]int64{
	's': 1,
	'm': secondsPerMinute,
	'h': secondsPerHour,
	'd': secondsPerDay,
	'w': secondsPerWeek,
	'y': secondsPerYear,
}

// Parse converts a human-readable duration into seconds.
// Empty input defaults to DefaultMaxExpiration ("1y").
// Supported units (case-insensitive): s, m, h, d, w, y.
// Composites like "1h30m" and "2d12h" are accepted.
func Parse(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		s = DefaultMaxExpiration
	}

	var total int64
	var num strings.Builder
	sawUnit := false

	for _, r := range s {
		if unicode.IsDigit(r) {
			num.WriteRune(r)
			continue
		}
		if unicode.IsSpace(r) {
			continue
		}

		unit := unicode.ToLower(r)
		mult, ok := unitSeconds[unit]
		if !ok {
			return 0, fmt.Errorf("invalid duration %q: unknown unit %q", s, string(r))
		}
		if num.Len() == 0 {
			return 0, fmt.Errorf("invalid duration %q: missing number before unit %q", s, string(r))
		}
		n, err := strconv.ParseInt(num.String(), 10, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid duration %q: %w", s, err)
		}
		if n > (math.MaxInt64-total)/mult {
			return 0, fmt.Errorf("invalid duration %q: exceeds maximum int64 seconds", s)
		}
		total += n * mult
		num.Reset()
		sawUnit = true
	}

	if num.Len() > 0 {
		return 0, fmt.Errorf("invalid duration %q: number without unit", s)
	}
	if !sawUnit {
		return 0, fmt.Errorf("invalid duration %q: no units", s)
	}
	if total < 0 {
		return 0, fmt.Errorf("invalid duration %q: negative result", s)
	}
	return total, nil
}
