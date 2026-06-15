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

func unfiledOnly(presets ...Preset) *presetsResponse {
	return &presetsResponse{
		Folders:              []Folder{},
		PresetsWithoutFolder: presets,
	}
}

func TestRenderRoutinesFitdown_UnfiledSectionHeading(t *testing.T) {
	var buf bytes.Buffer
	if err := renderRoutinesFitdown(&buf, unfiledOnly(bareRoutine("Push"), bareRoutine("Pull"))); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "# My Routines") {
		t.Errorf("missing '# My Routines' section heading; got:\n%s", out)
	}
	if !strings.Contains(out, "## Routine: Push") {
		t.Errorf("routine should render as H2 inside a section; got:\n%s", out)
	}
	if !strings.Contains(out, "## Routine: Pull") {
		t.Errorf("missing second routine; got:\n%s", out)
	}
	if !strings.Contains(out, "\n---\n") {
		t.Errorf("missing --- horizontal rule between sibling routines; got:\n%s", out)
	}
	if strings.HasSuffix(strings.TrimRight(out, "\n"), "---") {
		t.Errorf("trailing --- should not appear after last routine; got:\n%s", out)
	}
}

func TestRenderRoutinesFitdown_SingleRoutineNoSeparator(t *testing.T) {
	var buf bytes.Buffer
	renderRoutinesFitdown(&buf, unfiledOnly(bareRoutine("Push")))
	if strings.Contains(buf.String(), "---") {
		t.Errorf("single routine should have no --- separator; got:\n%s", buf.String())
	}
}

func TestRenderRoutinesFitdown_FavoriteStar(t *testing.T) {
	p := bareRoutine("Push")
	p.IsFavorite = true
	var buf bytes.Buffer
	renderRoutinesFitdown(&buf, unfiledOnly(p))
	if !strings.Contains(buf.String(), "## Routine: Push ★") {
		t.Errorf("favorite routine should be starred; got:\n%s", buf.String())
	}
}

func TestRenderRoutinesFitdown_FolderHeadingAndH2Routines(t *testing.T) {
	resp := &presetsResponse{
		PresetsWithoutFolder: []Preset{bareRoutine("Free Weights")},
		Folders: []Folder{{
			Name:    "Valley Creek",
			Presets: []Preset{bareRoutine("Valley Creek 1"), bareRoutine("Valley Creek 1 Copy")},
		}},
	}
	var buf bytes.Buffer
	if err := renderRoutinesFitdown(&buf, resp); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "# My Routines") {
		t.Errorf("missing unfiled section heading; got:\n%s", out)
	}
	if !strings.Contains(out, "# Valley Creek") {
		t.Errorf("folder name should render as H1; got:\n%s", out)
	}
	if !strings.Contains(out, "## Routine: Valley Creek 1") {
		t.Errorf("foldered routine should render as H2; got:\n%s", out)
	}
	// Unfiled section must come before folder sections so output order
	// matches the Liftoff app's UI (ungrouped on top).
	unfiledIdx := strings.Index(out, "# My Routines")
	folderIdx := strings.Index(out, "# Valley Creek")
	if unfiledIdx == -1 || folderIdx == -1 || unfiledIdx > folderIdx {
		t.Errorf("unfiled section must come before folder section; got:\n%s", out)
	}
}

func TestRenderRoutinesFitdown_EmptySectionSuppressed(t *testing.T) {
	// Account with only a folder — no unfiled presets. The "# My Routines"
	// heading must not appear over an empty section.
	resp := &presetsResponse{
		PresetsWithoutFolder: []Preset{},
		Folders: []Folder{{
			Name:    "Solo",
			Presets: []Preset{bareRoutine("Only One")},
		}},
	}
	var buf bytes.Buffer
	renderRoutinesFitdown(&buf, resp)
	out := buf.String()
	if strings.Contains(out, "# My Routines") {
		t.Errorf("empty unfiled section should not emit a heading; got:\n%s", out)
	}
	if !strings.Contains(out, "# Solo") {
		t.Errorf("folder heading missing; got:\n%s", out)
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
	renderRoutinesFitdown(&buf, unfiledOnly(p))
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
	renderRoutinesFitdown(&buf, unfiledOnly(p))
	if !strings.Contains(buf.String(), "3x5@100") {
		t.Errorf("consecutive identical sets should compress to Nx notation; got:\n%s", buf.String())
	}
}

func TestRenderOnePreset_ShowUsesH1(t *testing.T) {
	// `routines show` doesn't render inside a section, so the lone routine
	// gets an H1 ("# Routine: NAME") rather than the H2 used in list output.
	var buf bytes.Buffer
	renderOnePreset(&buf, bareRoutine("Push"), "#")
	if !strings.Contains(buf.String(), "# Routine: Push") {
		t.Errorf("show should use H1 marker; got:\n%s", buf.String())
	}
	if strings.Contains(buf.String(), "## Routine: Push") {
		t.Errorf("show should NOT use H2 marker; got:\n%s", buf.String())
	}
}

func TestPickPreset(t *testing.T) {
	resp := &presetsResponse{
		PresetsWithoutFolder: []Preset{
			{ID: "id-push", Name: "Push"},
			{ID: "id-pull", Name: "Pull"},
		},
		Folders: []Folder{{
			Name: "Valley Creek",
			Presets: []Preset{
				{ID: "id-vc1", Name: "Valley Creek 1"},
				{ID: "id-pull2", Name: "Pull"}, // collision with unfiled "Pull"
			},
		}},
	}
	if p, err := pickPreset(resp, "Push"); err != nil || p.ID != "id-push" {
		t.Errorf("exact name match on unfiled: got %+v, err=%v", p, err)
	}
	if p, err := pickPreset(resp, "Valley Creek 1"); err != nil || p.ID != "id-vc1" {
		t.Errorf("name match on foldered routine: got %+v, err=%v", p, err)
	}
	if p, err := pickPreset(resp, "valley creek 1"); err != nil || p.ID != "id-vc1" {
		t.Errorf("case-insensitive folder match: got %+v, err=%v", p, err)
	}
	if p, err := pickPreset(resp, "id-vc1"); err != nil || p.ID != "id-vc1" {
		t.Errorf("id match on foldered routine: got %+v, err=%v", p, err)
	}
	if _, err := pickPreset(resp, "Pull"); err == nil {
		t.Errorf("collision across folder boundary should error")
	}
	if _, err := pickPreset(resp, "nope"); err == nil {
		t.Errorf("missing name+id should error")
	}
}
