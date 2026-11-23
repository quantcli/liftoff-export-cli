package cmd

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// CONTRACT §4 record shape: "An empty collection is `[]`, never `null`."
//
// Both `exerciseData` fields in this CLI are declared `[]T` with no
// allocation, so the Go default applied: a nil slice marshals as `null`,
// and nothing in the code had to decide that it should. A routine saved
// with no exercises emitted `"exerciseData": null`, and an exercise
// added without target sets emitted `"setsData": null`.
//
// The consumer-side cost is out of proportion to the cause, which is why
// these tests assert the shape rather than trusting the struct tags: a
// consumer that recognizes one of our records by duck-typing the field
// (`Array.isArray(exerciseData)`) fails recognition rather than one
// value, and one that validates the whole array before accepting it then
// rejects the entire export over a single empty element.

// emptyCollectionPosts is the fixture that reproduces the defect. `[]`
// in the upstream payload does not — JSON `[]` decodes to an allocated
// empty slice, so it was never the broken case. Only an omitted key or
// an explicit `null` leaves the slice nil, so both appear here, next to
// a populated record.
//
// That mix is deliberate and load-bearing twice over. It is the form the
// §4 rule fails in — one record's array against another's `null` in the
// same export — and it is the shape compat/parity's opportunistic
// JSONCollectionsAreNeverNull subtest needs in order to fire at all.
//
// bodyweight is recorded on the first post only, so the `bodyweights`
// surfaces see exactly one entry and the dedupe-by-day path is not
// dragged into a test about collections.
var emptyCollectionPosts = json.RawMessage(`[
  {
    "id": "empty-populated",
    "startedAt": "2026-06-08T12:00:00.000Z",
    "postedAt": "2026-06-08T13:00:00.000Z",
    "sessionDuration": "45 minutes 00 seconds",
    "sessionNotes": "",
    "bodyweight": "175",
    "caloriesBurned": 300,
    "prCount": 0,
    "exerciseData": [
      {
        "exerciseIndex": 0,
        "exerciseName": "Bench Press",
        "exerciseId": "stub-ex-1",
        "exerciseTypes": "WR",
        "exerciseNotes": "",
        "setsData": [
          {"setIndex": 0, "setType": "normal", "inputOne": 100, "inputTwo": 5}
        ]
      },
      {
        "exerciseIndex": 1,
        "exerciseName": "Stretching",
        "exerciseId": "stub-ex-2",
        "exerciseTypes": "ND",
        "exerciseNotes": ""
      }
    ]
  },
  {
    "id": "empty-key-omitted",
    "startedAt": "2026-06-08T12:30:00.000Z",
    "postedAt": "2026-06-08T12:40:00.000Z",
    "sessionDuration": "10 minutes 00 seconds",
    "sessionNotes": "",
    "bodyweight": "",
    "caloriesBurned": 0,
    "prCount": 0
  },
  {
    "id": "empty-key-null",
    "startedAt": "2026-06-08T13:00:00.000Z",
    "postedAt": "2026-06-08T13:05:00.000Z",
    "sessionDuration": "5 minutes 00 seconds",
    "sessionNotes": "",
    "bodyweight": "",
    "caloriesBurned": 0,
    "prCount": 0,
    "exerciseData": null
  }
]`)

// Every workouts JSON surface must emit `[]` for an exercise-less
// workout and for a set-less exercise, on both the omitted-key and the
// explicit-null decode path.
func TestWorkoutsJSONEmptyCollectionsAreArrays(t *testing.T) {
	withPosts(t, emptyCollectionPosts)
	for _, args := range [][]string{
		{"workouts", "list"},
		{"workouts", "show", "2026-06-08"},
	} {
		name := strings.Join(args, " ")
		t.Run(name, func(t *testing.T) {
			stdout, _, err := run(t, append(args, "--format", "json")...)
			if err != nil {
				t.Fatalf("%s --format json failed: %v", name, err)
			}
			records := decodeJSONObjects(t, stdout)
			if len(records) != 3 {
				t.Fatalf("got %d records, want the fixture's 3:\n%s", len(records), stdout)
			}
			for _, rec := range records {
				where := fmt.Sprintf("%s record %v", name, rec["id"])
				for _, ex := range assertJSONArray(t, rec, "exerciseData", where) {
					exObj, ok := ex.(map[string]any)
					if !ok {
						t.Fatalf("%s: exerciseData element is %T, want an object", where, ex)
					}
					assertJSONArray(t, exObj, "setsData",
						fmt.Sprintf("%s exercise %v", where, exObj["exerciseName"]))
				}
			}
		})
	}
}

// The fixture has to keep carrying both states for the test above to
// mean anything — and for compat/parity's JSONCollectionsAreNeverNull to
// fire rather than pass vacuously. A later "simplification" that leaves
// every collection populated would turn both green without testing
// anything, so assert the mix itself.
func TestWorkoutsJSONFixtureCarriesBothCollectionStates(t *testing.T) {
	withPosts(t, emptyCollectionPosts)
	stdout, _, err := run(t, "workouts", "list", "--format", "json")
	if err != nil {
		t.Fatalf("workouts list --format json failed: %v", err)
	}
	var populated, empty int
	for _, rec := range decodeJSONObjects(t, stdout) {
		if len(assertJSONArray(t, rec, "exerciseData", fmt.Sprintf("record %v", rec["id"]))) == 0 {
			empty++
		} else {
			populated++
		}
	}
	if populated == 0 || empty == 0 {
		t.Errorf("fixture must carry both a populated and an empty exerciseData; got %d populated, %d empty",
			populated, empty)
	}
}

// emptyCollectionPresets is the routines-side fixture: the same two
// decode paths (key omitted, explicit null) at every depth the preset
// tree has — a folder's presets and presetsOrder, a routine's
// exerciseData, an exercise's setsData.
var emptyCollectionPresets = json.RawMessage(`{
  "folders": [
    {
      "id": "folder-1",
      "createdAt": "2026-06-01T00:00:00.000Z",
      "userId": "u1",
      "name": "Legs",
      "presetsOrder": null,
      "presets": [
        {
          "id": "preset-foldered",
          "createdAt": "2026-06-01T00:00:00.000Z",
          "userId": "u1",
          "name": "Valley Creek 1",
          "image": null,
          "marketPresetId": null,
          "bookmarkId": "",
          "completed": 0,
          "avgDuration": 0,
          "isFavorite": false,
          "folderId": "folder-1",
          "exerciseData": [
            {
              "id": "ped-1",
              "presetCuid": "preset-foldered",
              "exerciseIndex": 0,
              "exerciseName": "Back Squat",
              "exerciseId": "ex-1",
              "exerciseTypes": "WR",
              "superset": null,
              "overrideWeightUnit": null,
              "exerciseCuid": null,
              "marketExerciseCuid": null,
              "exerciseNotes": null
            }
          ]
        }
      ]
    },
    {
      "id": "folder-2",
      "createdAt": "2026-06-01T00:00:00.000Z",
      "userId": "u1",
      "name": "Empty Folder",
      "presetsOrder": []
    }
  ],
  "presetsWithoutFolder": [
    {
      "id": "preset-unfiled",
      "createdAt": "2026-06-01T00:00:00.000Z",
      "userId": "u1",
      "name": "Push",
      "image": null,
      "marketPresetId": null,
      "bookmarkId": "",
      "completed": 0,
      "avgDuration": 0,
      "isFavorite": true,
      "folderId": null
    }
  ]
}`)

// The routines half, asserted at the decode → normalize → marshal
// boundary rather than end to end: the in-process stub serves
// post.getMyPosts only, so `routines list` has no stub route yet.
// (liftoff-export-cli#80 adds one; when it lands, this is worth
// re-expressing as a `run(t, "routines", "list", ...)` case alongside
// its own tests.) The path under test is the one that was broken — the
// decoder leaves the nil, and json.Marshal turns it into `null`.
func TestPresetsJSONEmptyCollectionsAreArrays(t *testing.T) {
	var resp presetsResponse
	if err := json.Unmarshal(emptyCollectionPresets, &resp); err != nil {
		t.Fatalf("fixture does not decode: %v", err)
	}
	normalizePresetsResponse(&resp)

	encoded, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var envelope map[string]any
	if err := json.Unmarshal(encoded, &envelope); err != nil {
		t.Fatalf("re-decode: %v", err)
	}

	for _, p := range assertJSONArray(t, envelope, "presetsWithoutFolder", "envelope") {
		assertPresetCollections(t, p, "unfiled")
	}
	for _, f := range assertJSONArray(t, envelope, "folders", "envelope") {
		folder, ok := f.(map[string]any)
		if !ok {
			t.Fatalf("folders element is %T, want an object", f)
		}
		where := fmt.Sprintf("folder %v", folder["name"])
		assertJSONArray(t, folder, "presetsOrder", where)
		for _, p := range assertJSONArray(t, folder, "presets", where) {
			assertPresetCollections(t, p, where)
		}
	}
}

// assertPresetCollections checks one routine object and the exercises
// under it.
func assertPresetCollections(t *testing.T, preset any, where string) {
	t.Helper()
	obj, ok := preset.(map[string]any)
	if !ok {
		t.Fatalf("%s: preset is %T, want an object", where, preset)
	}
	where = fmt.Sprintf("%s routine %v", where, obj["name"])
	for _, ex := range assertJSONArray(t, obj, "exerciseData", where) {
		exObj, ok := ex.(map[string]any)
		if !ok {
			t.Fatalf("%s: exerciseData element is %T, want an object", where, ex)
		}
		assertJSONArray(t, exObj, "setsData",
			fmt.Sprintf("%s exercise %v", where, exObj["exerciseName"]))
	}
}

// --- helpers ---------------------------------------------------------------

func decodeJSONObjects(t *testing.T, stdout string) []map[string]any {
	t.Helper()
	var records []map[string]any
	if err := json.Unmarshal([]byte(stdout), &records); err != nil {
		t.Fatalf("stdout is not an array of objects (§4): %v; stdout=%q", err, stdout)
	}
	return records
}

// assertJSONArray fails unless obj[key] is present and holds a JSON
// array. A `null` there is the defect: §4 wants the key present and
// `[]`, not dropped and not null.
func assertJSONArray(t *testing.T, obj map[string]any, key, where string) []any {
	t.Helper()
	v, present := obj[key]
	if !present {
		t.Fatalf("%s: %q is absent; CONTRACT §4 record shape keeps the key and empties it", where, key)
	}
	arr, ok := v.([]any)
	if !ok {
		got := fmt.Sprintf("%T (%v)", v, v)
		if v == nil {
			got = "null"
		}
		t.Fatalf("%s: %q is %s, want a JSON array — CONTRACT §4: "+
			"\"An empty collection is `[]`, never `null`\"", where, key, got)
	}
	return arr
}
