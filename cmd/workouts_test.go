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

// Representative cases per ExerciseType so a regression in the shared
// fitdownSetLine surfaces in CI. The bodyweight cases (BR/AB with zero
// weight component) verify the @+0/@-0 simplification — the spec is silent
// on these so the rule lives in code, not docs.
func TestRenderWorkoutsFitdown_SetNotation(t *testing.T) {
	cases := []struct {
		name     string
		exType   string
		one, two string
		want     string
		notWant  string // optional negative assertion
	}{
		{"WR weighted", "WR", "100", "5", "5@100", ""},
		{"WR zero weight simplifies", "WR", "0", "5", "\n5\n", "5@0"},
		{"BR with extra weight", "BR", "12", "5", "5@+12", ""},
		{"BR zero added simplifies", "BR", "0", "10", "\n10\n", "10@+0"},
		{"AB with assistance", "AB", "20", "10", "10@-20", ""},
		{"AB zero assistance simplifies", "AB", "0", "10", "\n10\n", "10@-0"},
		{"ND duration", "ND", "0", "495", "8:15", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			w := bareWorkout()
			w.ExerciseData[0].ExerciseTypes = c.exType
			w.ExerciseData[0].SetsData[0] = SetData{
				InputOne: json.Number(c.one),
				InputTwo: json.Number(c.two),
			}
			var buf bytes.Buffer
			renderWorkoutsFitdown(&buf, []Post{w})
			out := buf.String()
			if !strings.Contains(out, c.want) {
				t.Errorf("expected %q in output; got:\n%s", c.want, out)
			}
			if c.notWant != "" && strings.Contains(out, c.notWant) {
				t.Errorf("did NOT want %q in output (should have been simplified); got:\n%s", c.notWant, out)
			}
		})
	}
}
