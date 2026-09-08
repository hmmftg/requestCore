package httpsemantics

import (
	"net/http"
	"testing"
	"time"
)

func TestParseRetryAfter_DeltaSeconds(t *testing.T) {
	ra, err := ParseRetryAfter("120")
	if err != nil {
		t.Fatalf("ParseRetryAfter() error = %v", err)
	}
	if ra.IsDate {
		t.Error("IsDate = true, want false")
	}
	if ra.Delta != 120 {
		t.Errorf("Delta = %d, want 120", ra.Delta)
	}
}

func TestParseRetryAfter_Zero(t *testing.T) {
	ra, err := ParseRetryAfter("0")
	if err != nil {
		t.Fatalf("ParseRetryAfter() error = %v", err)
	}
	if ra.Delta != 0 {
		t.Errorf("Delta = %d, want 0", ra.Delta)
	}
}

func TestParseRetryAfter_HTTPDate(t *testing.T) {
	dateStr := "Tue, 21 Oct 2025 07:28:00 GMT"
	ra, err := ParseRetryAfter(dateStr)
	if err != nil {
		t.Fatalf("ParseRetryAfter() error = %v", err)
	}
	if !ra.IsDate {
		t.Error("IsDate = false, want true")
	}
	expected, _ := http.ParseTime(dateStr)
	if !ra.Date.Equal(expected) {
		t.Errorf("Date = %v, want %v", ra.Date, expected)
	}
}

func TestParseRetryAfter_Empty(t *testing.T) {
	_, err := ParseRetryAfter("")
	if err == nil {
		t.Error("ParseRetryAfter(\"\") should error")
	}
}

func TestParseRetryAfter_Negative(t *testing.T) {
	_, err := ParseRetryAfter("-1")
	if err == nil {
		t.Error("ParseRetryAfter(\"-1\") should error")
	}
}

func TestParseRetryAfter_Malformed(t *testing.T) {
	_, err := ParseRetryAfter("not a date or number")
	if err == nil {
		t.Error("ParseRetryAfter with malformed input should error")
	}
}

func TestRetryAfterDuration_Delta(t *testing.T) {
	ra := RetryAfter{Delta: 60}
	now := time.Now()
	d := ra.Duration(now)
	if d != 60*time.Second {
		t.Errorf("Duration() = %v, want 60s", d)
	}
}

func TestRetryAfterDuration_Date_Future(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	future := now.Add(2 * time.Minute)
	ra := RetryAfter{Date: future, IsDate: true}
	d := ra.Duration(now)
	if d != 2*time.Minute {
		t.Errorf("Duration() = %v, want 2m", d)
	}
}

func TestRetryAfterDuration_Date_Past(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	past := now.Add(-1 * time.Minute)
	ra := RetryAfter{Date: past, IsDate: true}
	d := ra.Duration(now)
	if d != 0 {
		t.Errorf("Duration() = %v, want 0 (past date clamped)", d)
	}
}

func TestFormatRetryAfterDelta(t *testing.T) {
	if got := FormatRetryAfterDelta(120); got != "120" {
		t.Errorf("FormatRetryAfterDelta() = %q, want %q", got, "120")
	}
}

func TestFormatRetryAfterDate(t *testing.T) {
	tm := time.Date(2025, 10, 21, 7, 28, 0, 0, time.UTC)
	got := FormatRetryAfterDate(tm)
	want := "Tue, 21 Oct 2025 07:28:00 GMT"
	if got != want {
		t.Errorf("FormatRetryAfterDate() = %q, want %q", got, want)
	}
}

func TestClampRetryAfter_Delta(t *testing.T) {
	ra := RetryAfter{Delta: 600}
	now := time.Now()
	clamped := ClampRetryAfter(ra, now, 60*time.Second)
	if clamped != 60 {
		t.Errorf("ClampRetryAfter() = %d, want 60", clamped)
	}
}

func TestClampRetryAfter_Delta_UnderMax(t *testing.T) {
	ra := RetryAfter{Delta: 30}
	now := time.Now()
	clamped := ClampRetryAfter(ra, now, 60*time.Second)
	if clamped != 30 {
		t.Errorf("ClampRetryAfter() = %d, want 30", clamped)
	}
}

func TestClampRetryAfter_NoMax(t *testing.T) {
	ra := RetryAfter{Delta: 600}
	now := time.Now()
	clamped := ClampRetryAfter(ra, now, 0)
	if clamped != 600 {
		t.Errorf("ClampRetryAfter() = %d, want 600 (no clamp)", clamped)
	}
}

func TestFormatRetryAfter_Delta(t *testing.T) {
	ra := RetryAfter{Delta: 120}
	now := time.Now()
	got := FormatRetryAfter(ra, now, 0)
	if got != "120" {
		t.Errorf("FormatRetryAfter() = %q, want %q", got, "120")
	}
}

func TestFormatRetryAfter_Delta_Clamped(t *testing.T) {
	ra := RetryAfter{Delta: 600}
	now := time.Now()
	got := FormatRetryAfter(ra, now, 60*time.Second)
	if got != "60" {
		t.Errorf("FormatRetryAfter() = %q, want %q (clamped)", got, "60")
	}
}

func TestFormatRetryAfter_Date(t *testing.T) {
	tm := time.Date(2025, 10, 21, 7, 28, 0, 0, time.UTC)
	ra := RetryAfter{Date: tm, IsDate: true}
	now := time.Date(2025, 10, 21, 7, 27, 0, 0, time.UTC)
	got := FormatRetryAfter(ra, now, 0)
	want := "Tue, 21 Oct 2025 07:28:00 GMT"
	if got != want {
		t.Errorf("FormatRetryAfter() = %q, want %q", got, want)
	}
}

func TestFormatRetryAfter_Date_Clamped(t *testing.T) {
	// Date is 1 hour in the future, but max is 60 seconds
	tm := time.Date(2025, 10, 21, 8, 28, 0, 0, time.UTC)
	ra := RetryAfter{Date: tm, IsDate: true}
	now := time.Date(2025, 10, 21, 7, 28, 0, 0, time.UTC)
	got := FormatRetryAfter(ra, now, 60*time.Second)
	if got != "60" {
		t.Errorf("FormatRetryAfter() = %q, want %q (clamped)", got, "60")
	}
}
