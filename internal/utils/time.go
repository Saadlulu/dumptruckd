package utils

import "time"

// Now returns the current time. It's a variable so it can be overridden in tests.
var Now = time.Now

// FormatTimestamp formats a time as a backup-friendly timestamp
func FormatTimestamp(t time.Time) string {
	return t.Format("20060102_150405")
}

// FormatDatePath formats a time as a date-based directory path (YYYY/MM/DD)
func FormatDatePath(t time.Time) string {
	return t.Format("2006/01/02")
}

// ParseTimestamp parses a backup timestamp
func ParseTimestamp(s string) (time.Time, error) {
	return time.Parse("20060102_150405", s)
}
