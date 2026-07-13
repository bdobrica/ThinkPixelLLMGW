package httpapi

import (
	"net/url"
	"testing"
	"time"
)

func TestUsageRangeIsBounded(t *testing.T) {
	now := time.Date(2026, 7, 12, 12, 0, 0, 0, time.UTC)
	start, end, err := usageRange(url.Values{}, now)
	if err != nil || !end.Equal(now) || !start.Equal(now.Add(-24*time.Hour)) {
		t.Fatalf("unexpected default range: %v %v %v", start, end, err)
	}
	_, _, err = usageRange(url.Values{"start": {"2026-01-01T00:00:00Z"}, "end": {"2026-07-01T00:00:00Z"}}, now)
	if err == nil {
		t.Fatal("expected ranges over 90 days to be rejected")
	}
}

func TestBoundedPage(t *testing.T) {
	page, size := boundedPage(url.Values{"page": {"2"}, "page_size": {"1000"}})
	if page != 2 || size != 100 {
		t.Fatalf("unexpected pagination: %d %d", page, size)
	}
}
