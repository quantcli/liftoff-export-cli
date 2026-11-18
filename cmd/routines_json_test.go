package cmd

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/quantcli/liftoff-export-cli/internal/stubapi"
)

// `routines list --format json` used to hand printJSON the upstream
// fetchUserPresetsWithFolders envelope verbatim, so stdout was a
// `{folders, presetsWithoutFolder}` object. CONTRACT §4 says
// `--format json` is an "array on stdout". The compat/formats bundle
// cannot catch it — the bundle walks subcommands via --since/--until and
// `routines` takes neither — so these in-process tests are the only
// machine check this surface has. (liftoff-export-cli#73)
//
// Same harness as cli_contract_test.go: the real cobra tree against the
// in-process tRPC stub, which now serves presets as well as posts.

// withPresets swaps the stub's fetchUserPresetsWithFolders payload for
// one test and restores the sample account afterwards.
func withPresets(t *testing.T, presets json.RawMessage) {
	t.Helper()
	stubSrv.SetPresets(presets)
	t.Cleanup(func() { stubSrv.SetPresets(stubapi.SamplePresets) })
}

// routineJSONElement is the subset of an emitted element the shape tests
// assert on. Pointers so "absent" is distinguishable from "empty".
type routineJSONElement struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	FolderID   *string `json:"folderId"`
	FolderName *string `json:"folderName"`
}

func decodeRoutineElements(t *testing.T, stdout string) []routineJSONElement {
	t.Helper()
	var got []routineJSONElement
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("stdout is not a JSON array (§4): %v; stdout=%q", err, stdout)
	}
	return got
}

// --- §4 "json is an array on stdout" ---------------------------------------

func TestRoutinesJSONSurfacesAreArrays(t *testing.T) {
	for _, sub := range [][]string{
		{"routines", "list"},
		{"routines", "show", "Push"},
	} {
		name := strings.Join(sub, " ")
		t.Run(name, func(t *testing.T) {
			stdout, _, err := run(t, append(sub, "--format", "json")...)
			if err != nil {
				t.Fatalf("%s --format json failed: %v", name, err)
			}
			var rows []any
			if jsonErr := json.Unmarshal([]byte(stdout), &rows); jsonErr != nil {
				t.Fatalf("%s --format json is not an array (§4): %v; stdout=%q", name, jsonErr, stdout)
			}
			if len(rows) == 0 {
				t.Errorf("%s --format json returned no elements for an account with routines: %q", name, stdout)
			}
		})
	}
}

// The upstream envelope keys must not reach stdout at all: a consumer
// reaching for `.presetsWithoutFolder[]` should get a clean jq failure,
// not a half-migrated shape.
func TestRoutinesListJSONDropsUpstreamEnvelope(t *testing.T) {
	stdout, _, err := run(t, "routines", "list", "--format", "json")
	if err != nil {
		t.Fatalf("routines list --format json failed: %v", err)
	}
	var envelope map[string]any
	if json.Unmarshal([]byte(stdout), &envelope) == nil {
		t.Fatalf("stdout unmarshalled as a JSON object; §4 requires an array:\n%s", stdout)
	}
	for _, key := range []string{"presetsWithoutFolder", "presetsOrder", `"folders"`} {
		if strings.Contains(stdout, key) {
			t.Errorf("envelope/presentation key %s leaked into the flattened array:\n%s", key, stdout)
		}
	}
}

// Flattening walks unfiled routines first and then each folder's — the
// same order the markdown and csv renderers use, so the three codecs
// agree on row order.
func TestRoutinesListJSONFlattensEveryRoutineInRenderOrder(t *testing.T) {
	stdout, _, err := run(t, "routines", "list", "--format", "json")
	if err != nil {
		t.Fatalf("routines list --format json failed: %v", err)
	}
	got := decodeRoutineElements(t, stdout)
	if len(got) != 2 {
		t.Fatalf("got %d elements, want 2 (1 unfiled + 1 foldered):\n%s", len(got), stdout)
	}
	if got[0].Name != "Push" {
		t.Errorf("element 0 = %q, want the unfiled routine %q", got[0].Name, "Push")
	}
	if got[1].Name != "Valley Creek 1" {
		t.Errorf("element 1 = %q, want the foldered routine %q", got[1].Name, "Valley Creek 1")
	}
}

// Flattening must not lose folder membership. folderId is canonical and
// was already on every Preset; folderName is the additive
// human-readable label, and is null — not "" — when the routine is
// unfiled (§4 scalar typing: an absent value is null).
func TestRoutinesListJSONKeepsFolderLinkage(t *testing.T) {
	stdout, _, err := run(t, "routines", "list", "--format", "json")
	if err != nil {
		t.Fatalf("routines list --format json failed: %v", err)
	}
	got := decodeRoutineElements(t, stdout)
	if len(got) != 2 {
		t.Fatalf("got %d elements, want 2:\n%s", len(got), stdout)
	}

	unfiled, foldered := got[0], got[1]
	if unfiled.FolderID != nil {
		t.Errorf("unfiled folderId = %q, want null", *unfiled.FolderID)
	}
	if unfiled.FolderName != nil {
		t.Errorf("unfiled folderName = %q, want null", *unfiled.FolderName)
	}
	if foldered.FolderID == nil || *foldered.FolderID != "stub-folder-1" {
		t.Errorf("foldered folderId = %v, want stub-folder-1", foldered.FolderID)
	}
	if foldered.FolderName == nil || *foldered.FolderName != "Valley Creek" {
		t.Errorf("foldered folderName = %v, want \"Valley Creek\"", foldered.FolderName)
	}

	// The nested exercise/set payload has to survive the wrapper too —
	// it is what prime's documented jq recipe reads.
	var withExercises []struct {
		ExerciseData []struct {
			ExerciseName string `json:"exerciseName"`
		} `json:"exerciseData"`
	}
	if err := json.Unmarshal([]byte(stdout), &withExercises); err != nil {
		t.Fatalf("unmarshal exerciseData: %v", err)
	}
	if len(withExercises[0].ExerciseData) != 1 || withExercises[0].ExerciseData[0].ExerciseName != "Bench Press" {
		t.Errorf("exerciseData did not survive flattening: %+v", withExercises[0].ExerciseData)
	}
}

// `routines show` already emitted []Preset; it gains the same
// folderName field so both subcommands emit one element shape (§4: a
// logical field has one JSON type across every subcommand that emits it).
func TestRoutinesShowJSONCarriesFolderName(t *testing.T) {
	for _, c := range []struct {
		arg  string
		want *string
	}{
		{"Push", nil},
		{"Valley Creek 1", stringPtr("Valley Creek")},
	} {
		t.Run(c.arg, func(t *testing.T) {
			stdout, _, err := run(t, "routines", "show", c.arg, "--format", "json")
			if err != nil {
				t.Fatalf("routines show %q --format json failed: %v", c.arg, err)
			}
			got := decodeRoutineElements(t, stdout)
			if len(got) != 1 {
				t.Fatalf("got %d elements, want 1:\n%s", len(got), stdout)
			}
			switch {
			case c.want == nil && got[0].FolderName != nil:
				t.Errorf("folderName = %q, want null", *got[0].FolderName)
			case c.want != nil && (got[0].FolderName == nil || *got[0].FolderName != *c.want):
				t.Errorf("folderName = %v, want %q", got[0].FolderName, *c.want)
			}
		})
	}
}

// §4 empty-result rule on a surface the compat bundle never reaches.
func TestRoutinesListJSONEmptyAccountIsEmptyArray(t *testing.T) {
	withPresets(t, stubapi.EmptyPresets)
	stdout, _, err := run(t, "routines", "list", "--format", "json")
	if err != nil {
		t.Fatalf("exit non-zero on empty result (§4 says exit 0): %v", err)
	}
	if strings.TrimSpace(stdout) != "[]" {
		t.Errorf("empty account should emit `[]`, got %q", stdout)
	}
}

// --- json-only: markdown and csv are byte-identical ------------------------

// These two goldens were captured from the pre-fix binary. They are here
// to prove the array change is confined to the json codec: if a future
// edit to the flattening path leaks into either renderer, these fail.

const routinesListMarkdownGolden = `# My Routines

## Routine: Push ★

Bench Press
2x5@100

---

# Valley Creek

## Routine: Valley Creek 1

Kettlebell Swing (Left Only)
10@30
`

const routinesListCSVGolden = `folder_id,folder_name,id,name,created_at,is_favorite,completed,avg_duration,exercise_index,exercise_name,exercise_id,exercise_types,superset,override_weight_unit,exercise_notes,set_index,set_type,input_one,input_two
,,stub-preset-1,Push,2026-05-01T10:00:00Z,true,7,3600,0,Bench Press,stub-ex-1,WR,,,,0,normal,100,5
,,stub-preset-1,Push,2026-05-01T10:00:00Z,true,7,3600,0,Bench Press,stub-ex-1,WR,,,,1,normal,100,5
stub-folder-1,Valley Creek,stub-preset-2,Valley Creek 1,2026-05-03T10:00:00Z,false,4,2700,0,Kettlebell Swing,stub-ex-2,WR,,,Left Only,0,normal,30,10
`

func TestRoutinesListMarkdownAndCSVUnaffected(t *testing.T) {
	useZone(t, "TEST", 0) // created_at renders in local time
	for _, c := range []struct {
		name string
		args []string
		want string
	}{
		{"markdown", []string{"routines", "list"}, routinesListMarkdownGolden},
		{"csv", []string{"routines", "list", "--format", "csv"}, routinesListCSVGolden},
	} {
		t.Run(c.name, func(t *testing.T) {
			stdout, _, err := run(t, c.args...)
			if err != nil {
				t.Fatalf("%v failed: %v", c.args, err)
			}
			if stdout != c.want {
				t.Errorf("%s output changed; this was meant to be a json-only change\n got:\n%s\nwant:\n%s",
					c.name, stdout, c.want)
			}
		})
	}
}

// §4 default-is-markdown, extended to the two routines subcommands the
// compat bundle cannot walk.
func TestRoutinesDefaultFormatMatchesMarkdown(t *testing.T) {
	for _, sub := range [][]string{
		{"routines", "list"},
		{"routines", "show", "Push"},
	} {
		name := strings.Join(sub, " ")
		t.Run(name, func(t *testing.T) {
			bare, _, err := run(t, sub...)
			if err != nil {
				t.Fatalf("%s failed: %v", name, err)
			}
			explicit, _, err := run(t, append(sub, "--format", "markdown")...)
			if err != nil {
				t.Fatalf("%s --format markdown failed: %v", name, err)
			}
			if bare != explicit {
				t.Errorf("%s: default output differs from --format markdown\n default:  %q\n markdown: %q",
					name, bare, explicit)
			}
		})
	}
}
