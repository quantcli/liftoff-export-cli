package cmd

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// A workout the renderer hasn't been asked to apologize for: real
// timestamp, one exercise, one set. Used as the baseline; specific tests
// mutate one field at a time.
func bareWorkout() Post {
	return Post{
		StartedAt: "2026-06-08T12:00:00Z",
		ExerciseData: []ExerciseData{{
			ExerciseName:  "Bench Press",
			ExerciseTypes: "WR",
			SetsData: []SetData{{
				InputOne: json.Number("100"),
				InputTwo: json.Number("5"),
			}},
		}},
	}
}

func TestRenderWorkoutsFitdown_NoNotes(t *testing.T) {
	var buf bytes.Buffer
	renderWorkoutsFitdown(&buf, []Post{bareWorkout()})
	out := buf.String()
	if !strings.HasPrefix(out, "Workout ") {
		t.Errorf("expected Workout header prefix; got:\n%s", out)
	}
	if !strings.Contains(out, "Bench Press\n5@100") {
		t.Errorf("expected 'Bench Press' + set line; got:\n%s", out)
	}
}

func TestRenderWorkoutsFitdown_SessionNoteInParens(t *testing.T) {
	w := bareWorkout()
	w.SessionNotes = "felt great"
	var buf bytes.Buffer
	renderWorkoutsFitdown(&buf, []Post{w})
	out := buf.String()
	if !strings.Contains(out, "(felt great)") {
		t.Errorf("session note should append in parens to header; got:\n%s", out)
	}
	// Guard against the old "# felt great" rendering (markdown H1 collision).
	if strings.Contains(out, "\n# felt great") || strings.HasPrefix(out, "# felt great") {
		t.Errorf("session note should NOT render as markdown H1; got:\n%s", out)
	}
}

func TestRenderWorkoutsFitdown_ExerciseNoteInParens(t *testing.T) {
	w := bareWorkout()
	w.ExerciseData[0].ExerciseName = "Machine Seated Crunch"
	w.ExerciseData[0].ExerciseNotes = "Seat 3"
	var buf bytes.Buffer
	renderWorkoutsFitdown(&buf, []Post{w})
	out := buf.String()
	if !strings.Contains(out, "Machine Seated Crunch (Seat 3)") {
		t.Errorf("exercise note should render in parens after name; got:\n%s", out)
	}
	if strings.Contains(out, "\n# Seat 3") {
		t.Errorf("exercise note should NOT render as markdown H1; got:\n%s", out)
	}
}

// Set-line notation hasn't changed in this PR but is the load-bearing
// piece routines share with workouts (via the fitdownSetLine duplicate).
// One representative case per ExerciseType so a regression in either
// renderer surfaces in CI.
func TestRenderWorkoutsFitdown_SetNotation(t *testing.T) {
	cases := []struct {
		exType   string
		one, two string
		want     string
	}{
		{"WR", "100", "5", "5@100"},     // weight/reps
		{"BR", "0", "10", "10@+0"},      // bodyweight reps
		{"AB", "20", "10", "10@-20"},    // assisted bodyweight (minus)
		{"ND", "0", "495", "8:15"},      // no-data duration
	}
	for _, c := range cases {
		t.Run(c.exType, func(t *testing.T) {
			w := bareWorkout()
			w.ExerciseData[0].ExerciseTypes = c.exType
			w.ExerciseData[0].SetsData[0] = SetData{
				InputOne: json.Number(c.one),
				InputTwo: json.Number(c.two),
			}
			var buf bytes.Buffer
			renderWorkoutsFitdown(&buf, []Post{w})
			if !strings.Contains(buf.String(), c.want) {
				t.Errorf("%s: expected %q in output; got:\n%s", c.exType, c.want, buf.String())
			}
		})
	}
}
