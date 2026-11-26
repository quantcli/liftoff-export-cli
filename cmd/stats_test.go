package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestWorkoutsStatsMeasuredZeroAndNull(t *testing.T) {
	cases := []struct {
		name, exerciseType string
		inputOne, inputTwo string
		want               [5]string
	}{
		{"zero weight", "WR", "0", "10", [5]string{"0", "10", "0", "null", "null"}},
		{"zero reps", "WR", "50", "0", [5]string{"50", "0", "0", "null", "null"}},
		{"full assist", "AB", "175", "10", [5]string{"0", "10", "0", "null", "null"}},
		{"bodyweight plus resistance", "BR", "2.5", "4", [5]string{"177.5", "4", "710", "null", "null"}},
		{"zero distance", "DD", "0", "300", [5]string{"null", "null", "null", "300", "0"}},
		{"zero duration", "DD", "5", "0", [5]string{"null", "null", "null", "0", "5"}},
		{"non-measuring type", "ND", "0", "90", [5]string{"null", "null", "null", "null", "null"}},
		{"unknown type", "ZZ", "7", "8", [5]string{"null", "null", "null", "null", "null"}},
	}
	jsonKeys := []string{"bestWeight", "bestReps", "volume", "duration", "distance"}
	csvColumns := []string{"best_weight", "best_reps", "volume", "duration", "distance"}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			withPosts(t, json.RawMessage(fmt.Sprintf(`[{
				"id":"stats-zero", "startedAt":"2026-06-08T12:00:00Z", "bodyweight":"175",
				"exerciseData":[{"exerciseName":"Exercise", "exerciseTypes":%q,
				"setsData":[{"setType":"normal", "inputOne":%s, "inputTwo":%s}]}]
			}]`, c.exerciseType, c.inputOne, c.inputTwo)))

			t.Run("json", func(t *testing.T) {
				stdout, stderr, err := run(t, "workouts", "stats", "--format", "json")
				if err != nil || stderr != "" {
					t.Fatalf("json: err=%v, stderr=%q", err, stderr)
				}
				var summaries []struct {
					Name     string            `json:"name"`
					Type     string            `json:"type"`
					Sessions []json.RawMessage `json:"sessions"`
				}
				if err := json.Unmarshal([]byte(stdout), &summaries); err != nil {
					t.Fatalf("decode JSON: %v", err)
				}
				if len(summaries) != 1 || len(summaries[0].Sessions) != 1 {
					t.Fatalf("want one summary with one session, got %s", stdout)
				}
				if summaries[0].Name != "Exercise" || summaries[0].Type != c.exerciseType {
					t.Errorf("unexpected summary: %s", stdout)
				}
				var fields map[string]json.RawMessage
				if err := json.Unmarshal(summaries[0].Sessions[0], &fields); err != nil {
					t.Fatalf("decode session: %v", err)
				}
				for i, key := range jsonKeys {
					raw, present := fields[key]
					if !present {
						t.Errorf("missing key %s (want %s)", key, c.want[i])
						continue
					}
					if c.want[i] == "null" {
						if string(raw) != "null" {
							t.Errorf("%s = %s, want null", key, raw)
						}
						continue
					}
					var got, want *float64
					if err := json.Unmarshal(raw, &got); err != nil {
						t.Fatalf("decode %s: %v", key, err)
					}
					if err := json.Unmarshal([]byte(c.want[i]), &want); err != nil {
						t.Fatalf("decode expected %s: %v", key, err)
					}
					if got == nil || *got != *want {
						t.Errorf("%s = %s, want number %s", key, raw, c.want[i])
					}
				}

				// Pin the complete session key set and its existing order.
				var compact bytes.Buffer
				if err := json.Compact(&compact, summaries[0].Sessions[0]); err != nil {
					t.Fatal(err)
				}
				prefix := fmt.Sprintf(`{"date":%s,"bodyweight":175,"sets":1,"reps":%s`, fields["date"], fields["reps"])
				var suffix strings.Builder
				for i, key := range jsonKeys {
					fmt.Fprintf(&suffix, `,%q:%s`, key, c.want[i])
				}
				if want := prefix + suffix.String() + "}"; compact.String() != want {
					t.Errorf("session shape/order = %s, want %s", compact.String(), want)
				}
			})

			t.Run("csv", func(t *testing.T) {
				stdout, stderr, err := run(t, "workouts", "stats", "--format", "csv")
				if err != nil || stderr != "" {
					t.Fatalf("csv: err=%v, stderr=%q", err, stderr)
				}
				header, rows := parseCSV(t, stdout)
				if want := "name,type,date,bodyweight,sets,reps,best_weight,best_reps,volume,duration,distance\n"; !strings.HasPrefix(stdout, want) {
					t.Errorf("CSV header changed: %q", stdout)
				}
				if len(rows) != 1 {
					t.Fatalf("want one session row, got %d", len(rows))
				}
				for i, column := range csvColumns {
					want := c.want[i]
					if want == "null" {
						want = ""
					}
					if got := col(t, header, rows[0], column); got != want {
						t.Errorf("%s = %q, want %q", column, got, want)
					}
				}
			})
		})
	}
}
