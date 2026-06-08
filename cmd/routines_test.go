package cmd

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func stringPtr(s string) *string { return &s }

// Smallest possible preset — just a name, no exercises. Used by the
// header/separator tests where exercise rendering is incidental.
func bareRoutine(name string) Preset {
	return Preset{Name: name, ExerciseData: []PresetExerciseData{}}
}

func TestRenderRoutinesFitdown_HeadingAndSeparator(t *testing.T) {
	var buf bytes.Buffer
	if err := renderRoutinesFitdown(&buf, []Preset{bareRoutine("Push"), bareRoutine("Pull")}); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "# Routine: Push") {
		t.Errorf("missing H1 for first routine; got:\n%s", out)
	}
	if !strings.Contains(out, "# Routine: Pull") {
		t.Errorf("missing H1 for second routine; got:\n%s", out)
	}
	if !strings.Contains(out, "\n---\n") {
		t.Errorf("missing --- horizontal rule between routines; got:\n%s", out)
	}
	if strings.HasSuffix(strings.TrimRight(out, "\n"), "---") {
		t.Errorf("trailing --- should not appear after last routine; got:\n%s", out)
	}
}

func TestRenderRoutinesFitdown_SingleRoutineNoSeparator(t *testing.T) {
	var buf bytes.Buffer
	renderRoutinesFitdown(&buf, []Preset{bareRoutine("Push")})
	if strings.Contains(buf.String(), "---") {
		t.Errorf("single routine should have no --- separator; got:\n%s", buf.String())
	}
}

func TestRenderRoutinesFitdown_FavoriteStar(t *testing.T) {
	p := bareRoutine("Push")
	p.IsFavorite = true
	var buf bytes.Buffer
	renderRoutinesFitdown(&buf, []Preset{p})
	if !strings.Contains(buf.String(), "# Routine: Push ★") {
		t.Errorf("favorite routine should be starred; got:\n%s", buf.String())
	}
}

func TestRenderRoutinesFitdown_ExerciseNoteInParens(t *testing.T) {
	p := Preset{
		Name: "Test",
		ExerciseData: []PresetExerciseData{{
			ExerciseName:  "Kettlebell Swing",
			ExerciseTypes: "WR",
			ExerciseNotes: stringPtr("Left Only"),
			SetsData: []PresetSetData{{
				InputOne: json.Number("30"),
				InputTwo: json.Number("10"),
			}},
		}},
	}
	var buf bytes.Buffer
	renderRoutinesFitdown(&buf, []Preset{p})
	out := buf.String()
	if !strings.Contains(out, "Kettlebell Swing (Left Only)") {
		t.Errorf("exercise note should render in parens after name; got:\n%s", out)
	}
	// Guard against the old "# Left Only" rendering (markdown H1 collision).
	if strings.Contains(out, "\n# Left Only") {
		t.Errorf("exercise note should NOT render as markdown H1; got:\n%s", out)
	}
}

func TestRenderRoutinesFitdown_SetCompression(t *testing.T) {
	mkSet := func() PresetSetData {
		return PresetSetData{InputOne: json.Number("100"), InputTwo: json.Number("5")}
	}
	p := Preset{
		Name: "Test",
		ExerciseData: []PresetExerciseData{{
			ExerciseName:  "Squat",
			ExerciseTypes: "WR",
			SetsData:      []PresetSetData{mkSet(), mkSet(), mkSet()},
		}},
	}
	var buf bytes.Buffer
	renderRoutinesFitdown(&buf, []Preset{p})
	if !strings.Contains(buf.String(), "3x5@100") {
		t.Errorf("consecutive identical sets should compress to Nx notation; got:\n%s", buf.String())
	}
}

func TestPickPreset(t *testing.T) {
	presets := []Preset{
		{ID: "id-push", Name: "Push"},
		{ID: "id-pull", Name: "Pull"},
		{ID: "id-pull2", Name: "Pull"}, // collision
	}
	if p, err := pickPreset(presets, "Push"); err != nil || p.ID != "id-push" {
		t.Errorf("exact name match: got %+v, err=%v", p, err)
	}
	if p, err := pickPreset(presets, "push"); err != nil || p.ID != "id-push" {
		t.Errorf("case-insensitive name match: got %+v, err=%v", p, err)
	}
	if p, err := pickPreset(presets, "id-push"); err != nil || p.ID != "id-push" {
		t.Errorf("id match: got %+v, err=%v", p, err)
	}
	if _, err := pickPreset(presets, "Pull"); err == nil {
		t.Errorf("colliding name should error to force disambiguation by id")
	}
	if _, err := pickPreset(presets, "nope"); err == nil {
		t.Errorf("missing name+id should error")
	}
}
