package utils

import (
	"testing"
	"time"
)

func TestFormatTimestamp_ProducesExpectedFormat(t *testing.T) {
	ts := time.Date(2024, 3, 15, 14, 30, 45, 0, time.UTC)
	got := FormatTimestamp(ts)
	want := "20240315_143045"
	if got != want {
		t.Errorf("FormatTimestamp() = %q, want %q", got, want)
	}
}

func TestFormatDatePath_ProducesExpectedFormat(t *testing.T) {
	ts := time.Date(2024, 3, 15, 14, 30, 45, 0, time.UTC)
	got := FormatDatePath(ts)
	want := "2024/03/15"
	if got != want {
		t.Errorf("FormatDatePath() = %q, want %q", got, want)
	}
}

func TestParseTimestamp_RoundTrips(t *testing.T) {
	original := time.Date(2024, 12, 1, 9, 5, 0, 0, time.UTC)
	formatted := FormatTimestamp(original)
	parsed, err := ParseTimestamp(formatted)
	if err != nil {
		t.Fatalf("ParseTimestamp() error = %v", err)
	}
	if !parsed.Equal(original) {
		t.Errorf("Round-trip failed: got %v, want %v", parsed, original)
	}
}

func TestParseTimestamp_InvalidInput_ReturnsError(t *testing.T) {
	_, err := ParseTimestamp("not-a-timestamp")
	if err == nil {
		t.Error("ParseTimestamp() should return error for invalid input")
	}
}

func TestParseTimestamp_EmptyInput_ReturnsError(t *testing.T) {
	_, err := ParseTimestamp("")
	if err == nil {
		t.Error("ParseTimestamp() should return error for empty input")
	}
}
