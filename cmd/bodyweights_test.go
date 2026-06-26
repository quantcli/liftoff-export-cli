package cmd

import (
	"sort"
	"testing"
	"time"
)

// Two workouts on the same calendar day must collapse to a single
// bodyweight row, keeping the latest measurement that day. (#30)
func TestDedupeByDay_KeepsLatestPerDay(t *testing.T) {
	mk := func(ts string, w float64) bodyweightEntry {
		d, err := time.Parse(time.RFC3339, ts)
		if err != nil {
			t.Fatalf("bad test timestamp %q: %v", ts, err)
		}
		return bodyweightEntry{date: d, weight: w}
	}

	in := []bodyweightEntry{
		mk("2026-05-02T07:00:00Z", 175.0), // morning workout
		mk("2026-05-02T18:00:00Z", 177.0), // evening workout, same day
		mk("2026-05-03T08:00:00Z", 176.0),
	}

	out := dedupeByDay(in)
	sort.Slice(out, func(i, j int) bool { return out[i].date.Before(out[j].date) })

	if len(out) != 2 {
		t.Fatalf("expected 2 rows after dedupe (one per day), got %d: %+v", len(out), out)
	}
	if got := out[0].date.Format("2006-01-02"); got != "2026-05-02" {
		t.Errorf("first row date = %s, want 2026-05-02", got)
	}
	if out[0].weight != 177.0 {
		t.Errorf("same-day collision should keep the latest measurement (177.0), got %v", out[0].weight)
	}
	if out[1].weight != 176.0 {
		t.Errorf("second day weight = %v, want 176.0", out[1].weight)
	}
}

// A single entry per day passes through unchanged.
func TestDedupeByDay_NoDuplicates(t *testing.T) {
	d1, _ := time.Parse(time.RFC3339, "2026-01-01T10:00:00Z")
	d2, _ := time.Parse(time.RFC3339, "2026-01-02T10:00:00Z")
	in := []bodyweightEntry{{date: d1, weight: 180}, {date: d2, weight: 181}}
	if got := len(dedupeByDay(in)); got != 2 {
		t.Errorf("expected 2 rows, got %d", got)
	}
}
