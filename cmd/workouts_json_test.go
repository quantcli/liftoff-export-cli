package cmd

import (
	"encoding/json"
	"strings"
	"testing"
)

// workouts list/show JSON must emit bodyweight as a number (matching
// `workouts stats`) and add a numeric sessionDurationSeconds alongside the
// human sessionDuration string. (#33, #36)
func TestPostMarshalJSON_NumericFields(t *testing.T) {
	p := Post{
		ID:              "abc",
		StartedAt:       "2026-06-11T03:14:12.665Z",
		SessionDuration: "01 hours 06 minutes 01 seconds",
		Bodyweight:      "175",
		CaloriesBurned:  56,
	}

	b, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	raw := string(b)

	// bodyweight must be a bare number, not a quoted string.
	if strings.Contains(raw, `"bodyweight":"175"`) {
		t.Errorf("bodyweight should be a number, got quoted string in:\n%s", raw)
	}

	var got struct {
		Bodyweight             *float64 `json:"bodyweight"`
		SessionDuration        string   `json:"sessionDuration"`
		SessionDurationSeconds *int     `json:"sessionDurationSeconds"`
	}
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Bodyweight == nil || *got.Bodyweight != 175 {
		t.Errorf("bodyweight = %v, want 175", got.Bodyweight)
	}
	if got.SessionDuration != "01 hours 06 minutes 01 seconds" {
		t.Errorf("human sessionDuration should be preserved, got %q", got.SessionDuration)
	}
	if got.SessionDurationSeconds == nil || *got.SessionDurationSeconds != 3961 {
		t.Errorf("sessionDurationSeconds = %v, want 3961 (1h06m01s)", got.SessionDurationSeconds)
	}
}

// An absent bodyweight marshals as null, not "".
func TestPostMarshalJSON_EmptyBodyweightIsNull(t *testing.T) {
	b, err := json.Marshal(Post{StartedAt: "2026-06-11T03:14:12Z"})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(b), `"bodyweight":null`) {
		t.Errorf("empty bodyweight should marshal as null, got:\n%s", string(b))
	}
}

func TestParseDurationSeconds(t *testing.T) {
	cases := []struct {
		in   string
		want *int
	}{
		{"01 hours 06 minutes 01 seconds", intp(3961)},
		{"45 minutes 30 seconds", intp(2730)},
		{"2 hours", intp(7200)},
		{"", nil},
		{"a while", nil},      // non-numeric
		{"5 fortnights", nil}, // unknown unit
		{"10 minutes 5", nil}, // odd field count
	}
	for _, c := range cases {
		got := parseDurationSeconds(c.in)
		switch {
		case c.want == nil && got != nil:
			t.Errorf("parseDurationSeconds(%q) = %d, want nil", c.in, *got)
		case c.want != nil && (got == nil || *got != *c.want):
			t.Errorf("parseDurationSeconds(%q) = %v, want %d", c.in, got, *c.want)
		}
	}
}

func intp(n int) *int { return &n }
