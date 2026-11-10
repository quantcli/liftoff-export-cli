package cmd

import (
	"testing"
	"time"
)

func TestFilterByWindowIncludesEntireDSTDay(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	old := time.Local
	time.Local = loc
	t.Cleanup(func() { time.Local = old })
	for _, tc := range []struct{ day, inside, after string }{
		{"2026-03-08", "2026-03-08T23:59:59-04:00", "2026-03-09T00:00:00-04:00"},
		{"2026-11-01", "2026-11-01T23:59:59-05:00", "2026-11-02T00:00:00-05:00"},
	} {
		t.Run(tc.day, func(t *testing.T) {
			start, err := parseDateValue(tc.day)
			if err != nil {
				t.Fatal(err)
			}
			end, err := parseUntilValue(tc.day)
			if err != nil {
				t.Fatal(err)
			}
			got := filterByWindow([]Post{{StartedAt: tc.inside}, {StartedAt: tc.after}}, start, end)
			if len(got) != 1 || got[0].StartedAt != tc.inside {
				t.Fatalf("filtered posts=%+v", got)
			}
		})
	}
}
