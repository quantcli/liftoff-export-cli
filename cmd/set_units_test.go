package cmd

import (
	"encoding/json"
	"strings"
	"testing"
)

// CONTRACT §4, Units and cross-codec parity:
//
//	"Where a sibling field selects the unit, the dispatch is documented.
//	 … the dispatch field and its full mapping MUST appear in `prime`
//	 under SUBCOMMANDS, and the preferred fix is additive unit-named
//	 siblings (`weight_lb` / `distance_km`) beside the polymorphic field."
//
// liftoff's inputOne is pounds for a weight exercise and kilometres for a
// distance one, selected by the enclosing exercise's exerciseTypes, and
// neither input carried a unit. A consumer that reads a weight without a
// unit field as "already in the unit I display" imports our pound numbers
// as kilograms: every value inflated ~2.2x, nothing on their side that
// looks unusual, no error anywhere. These tests pin down the siblings
// that make a correct read possible, and the §9 boundary that the bare
// fields keep their names and their meanings.

// unitFixture is one workout carrying every live exerciseTypes code plus
// one this CLI does not know, so the dispatch is covered end to end
// rather than per-helper. The WR weight is a kilogram-origin value
// (50 kg) with the full fractional tail Liftoff returns, which is the
// case that must survive into the sibling digit for digit.
var unitFixture = json.RawMessage(`[
  {
    "id": "units-1",
    "startedAt": "2026-06-08T12:00:00.000Z",
    "postedAt": "2026-06-08T13:00:00.000Z",
    "sessionDuration": "45 minutes 00 seconds",
    "sessionNotes": "",
    "bodyweight": "175",
    "caloriesBurned": 300,
    "prCount": 0,
    "exerciseData": [
      {"exerciseIndex": 0, "exerciseName": "Cable Press", "exerciseId": "e-wr",
       "exerciseTypes": "WR", "exerciseNotes": "",
       "setsData": [{"setIndex": 0, "setType": "normal", "inputOne": 110.2311310925, "inputTwo": 5}]},
      {"exerciseIndex": 1, "exerciseName": "Assisted Pull Up", "exerciseId": "e-ab",
       "exerciseTypes": "AB", "exerciseNotes": "",
       "setsData": [{"setIndex": 0, "setType": "normal", "inputOne": 100, "inputTwo": 3}]},
      {"exerciseIndex": 2, "exerciseName": "Weighted Dip", "exerciseId": "e-br",
       "exerciseTypes": "BR", "exerciseNotes": "",
       "setsData": [{"setIndex": 0, "setType": "normal", "inputOne": 25, "inputTwo": 6}]},
      {"exerciseIndex": 3, "exerciseName": "Farmer Carry", "exerciseId": "e-wd",
       "exerciseTypes": "WD", "exerciseNotes": "",
       "setsData": [{"setIndex": 0, "setType": "normal", "inputOne": 80, "inputTwo": 0.4}]},
      {"exerciseIndex": 4, "exerciseName": "Hike", "exerciseId": "e-dd",
       "exerciseTypes": "DD", "exerciseNotes": "",
       "setsData": [{"setIndex": 0, "setType": "normal", "inputOne": 8.04672, "inputTwo": 3600}]},
      {"exerciseIndex": 5, "exerciseName": "Plank", "exerciseId": "e-nd",
       "exerciseTypes": "ND", "exerciseNotes": "",
       "setsData": [{"setIndex": 0, "setType": "normal", "inputOne": 0, "inputTwo": 90}]},
      {"exerciseIndex": 6, "exerciseName": "Something New", "exerciseId": "e-xx",
       "exerciseTypes": "ZZ", "exerciseNotes": "",
       "setsData": [{"setIndex": 0, "setType": "normal", "inputOne": 7, "inputTwo": 8}]}
    ]
  }
]`)

// jsonSet is the decoded shape of one set in `workouts list --format
// json`. The sibling fields are pointers so a present-but-null key is
// distinguishable from an absent one; `set` keeps the raw object so the
// key-presence assertion can look for the keys themselves.
type jsonSet struct {
	raw        map[string]any
	InputOne   json.Number  `json:"inputOne"`
	InputTwo   json.Number  `json:"inputTwo"`
	WeightLb   *json.Number `json:"weightLb"`
	DistanceKm *json.Number `json:"distanceKm"`
}

// setsByExerciseType runs `workouts list --format json` against the
// fixture and returns the first set of each exercise, keyed by its
// exerciseTypes code.
func setsByExerciseType(t *testing.T) map[string]jsonSet {
	t.Helper()
	withPosts(t, unitFixture)
	stdout, _, err := run(t, "workouts", "list", "--format", "json")
	if err != nil {
		t.Fatalf("workouts list --format json failed: %v", err)
	}

	var posts []struct {
		ExerciseData []struct {
			ExerciseTypes string            `json:"exerciseTypes"`
			SetsData      []json.RawMessage `json:"setsData"`
		} `json:"exerciseData"`
	}
	if err := json.Unmarshal([]byte(stdout), &posts); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, stdout)
	}
	if len(posts) != 1 {
		t.Fatalf("got %d posts, want the fixture's 1:\n%s", len(posts), stdout)
	}

	out := map[string]jsonSet{}
	for _, ex := range posts[0].ExerciseData {
		if len(ex.SetsData) == 0 {
			t.Fatalf("exercise %s lost its sets", ex.ExerciseTypes)
		}
		var s jsonSet
		if err := json.Unmarshal(ex.SetsData[0], &s); err != nil {
			t.Fatalf("decode %s set: %v", ex.ExerciseTypes, err)
		}
		if err := json.Unmarshal(ex.SetsData[0], &s.raw); err != nil {
			t.Fatalf("decode %s set as object: %v", ex.ExerciseTypes, err)
		}
		out[ex.ExerciseTypes] = s
	}
	if len(out) != 7 {
		t.Fatalf("got %d exercise types, want the fixture's 7", len(out))
	}
	return out
}

// The dispatch itself: which input each code's weight and distance
// sibling is drawn from. "" means the key must be present and null.
func TestSetUnitSiblingsDispatchOnExerciseTypes(t *testing.T) {
	sets := setsByExerciseType(t)

	cases := []struct {
		exType     string
		weightLb   string
		distanceKm string
		why        string
	}{
		{"WR", "110.2311310925", "", "inputOne is the lifted weight in pounds"},
		{"AB", "100", "", "inputOne is assistance in pounds, subtracted from bodyweight"},
		{"BR", "25", "", "inputOne is added load in pounds, on top of bodyweight"},
		{"WD", "80", "0.4", "inputOne pounds carried, inputTwo kilometres covered"},
		{"DD", "", "8.04672", "inputOne is kilometres; there is no weight"},
		{"ND", "", "", "a timed entry has neither a weight nor a distance"},
		{"ZZ", "", "", "an unknown code gets nulls, never a guess"},
	}

	for _, c := range cases {
		t.Run(c.exType, func(t *testing.T) {
			s := sets[c.exType]
			if got := numberOrEmpty(s.WeightLb); got != c.weightLb {
				t.Errorf("weightLb = %q, want %q (%s)", got, c.weightLb, c.why)
			}
			if got := numberOrEmpty(s.DistanceKm); got != c.distanceKm {
				t.Errorf("distanceKm = %q, want %q (%s)", got, c.distanceKm, c.why)
			}
		})
	}
}

// §4 record shape: "A field with no value is present and `null` — not
// omitted." A consumer that reaches for .weightLb must find the key on
// every set, including a timed one, or it cannot tell "this exercise has
// no weight" from "this exporter does not emit weights".
func TestSetUnitSiblingsAreAlwaysPresent(t *testing.T) {
	for exType, s := range setsByExerciseType(t) {
		for _, key := range []string{"weightLb", "distanceKm"} {
			v, ok := s.raw[key]
			if !ok {
				t.Errorf("%s set: %s key is missing; §4 wants it present and null", exType, key)
				continue
			}
			// Absent must be null, never 0 — a zero weight is a real
			// bodyweight rep, which is not the same as no weight.
			if v == nil {
				continue
			}
			if _, isNum := v.(float64); !isNum {
				t.Errorf("%s set: %s = %v (%T), want a number or null", exType, key, v, v)
			}
		}
	}
	// The one case worth naming: ND must be null on both, not 0.
	nd := setsByExerciseType(t)["ND"]
	if nd.raw["weightLb"] != nil || nd.raw["distanceKm"] != nil {
		t.Errorf("ND siblings = %v / %v, want null / null — 0 would read as a real measurement",
			nd.raw["weightLb"], nd.raw["distanceKm"])
	}
}

// §4: "A codec MUST NOT convert on the way out." The sibling is a
// relabelling of a number we already emit, so it must be the same
// number — to the last digit. A kilogram-origin weight is where that
// bites: 50 kg arrives as 110.2311310925 lb, and a float round trip
// through a different formatter is exactly how a sibling starts
// disagreeing with the field it was projected from.
func TestSetUnitSiblingsDoNotConvertOrReformat(t *testing.T) {
	s := setsByExerciseType(t)["WR"]
	if got, want := numberOrEmpty(s.WeightLb), s.InputOne.String(); got != want {
		t.Errorf("weightLb = %q but inputOne = %q; the sibling must carry the same digits", got, want)
	}
	if got := numberOrEmpty(s.WeightLb); got != "110.2311310925" {
		t.Errorf("weightLb = %q, want 110.2311310925 (50 kg as Liftoff stores it)", got)
	}

	dd := setsByExerciseType(t)["DD"]
	if got, want := numberOrEmpty(dd.DistanceKm), dd.InputOne.String(); got != want {
		t.Errorf("DD distanceKm = %q but inputOne = %q", got, want)
	}
}

// The §9 boundary carried from QUA-119: "a unit change is performed as a
// rename, never in place … for a field grandfathered under a bare name
// the unit documented in `prime` is thereby fixed." The siblings are
// additive. inputOne and inputTwo keep their names, their values and
// their polymorphic meaning, so no existing jq path or agent tool-call
// changes behaviour.
func TestBareInputsAreUnchangedByTheSiblings(t *testing.T) {
	want := map[string][2]string{
		"WR": {"110.2311310925", "5"},
		"AB": {"100", "3"},
		"BR": {"25", "6"},
		"WD": {"80", "0.4"},
		"DD": {"8.04672", "3600"},
		"ND": {"0", "90"},
		"ZZ": {"7", "8"},
	}
	for exType, s := range setsByExerciseType(t) {
		w := want[exType]
		if s.InputOne.String() != w[0] || s.InputTwo.String() != w[1] {
			t.Errorf("%s: inputOne/inputTwo = %q/%q, want %q/%q — the bare fields must not move",
				exType, s.InputOne, s.InputTwo, w[0], w[1])
		}
		for _, key := range []string{"setIndex", "setType", "inputOne", "inputTwo"} {
			if _, ok := s.raw[key]; !ok {
				t.Errorf("%s: pre-existing key %s disappeared", exType, key)
			}
		}
		// Six keys exactly: the four that were there plus the two added.
		// A seventh means something was emitted that nothing documents.
		if len(s.raw) != 6 {
			t.Errorf("%s: set has keys %v, want exactly the 4 original plus weightLb/distanceKm",
				exType, keysOf(s.raw))
		}
	}
}

// The new ExerciseData.MarshalJSON rebuilds setsData, so the §4
// empty-collection invariant from the other half of this work has to
// hold through it: an exercise with no sets is `[]`, never `null`.
func TestSetsWithUnitsKeepsEmptyCollectionsAsArrays(t *testing.T) {
	for _, sets := range [][]SetData{nil, {}} {
		b, err := json.Marshal(ExerciseData{ExerciseTypes: "WR", SetsData: sets})
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		if !strings.Contains(string(b), `"setsData":[]`) {
			t.Errorf("setsData from %v slice = %s, want []", sets, b)
		}
	}
}

// An input the upstream omits leaves json.Number empty. Emitting it raw
// would produce `"weightLb":` with no value — invalid JSON, which fails
// the whole export rather than one field.
func TestAbsentInputYieldsNullSibling(t *testing.T) {
	b, err := json.Marshal(setsWithUnits("WD", []SetData{{SetIndex: 0, SetType: "normal"}}))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got []map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("an absent input produced invalid JSON: %v\n%s", err, b)
	}
	if got[0]["weightLb"] != nil || got[0]["distanceKm"] != nil {
		t.Errorf("siblings = %v / %v, want null / null", got[0]["weightLb"], got[0]["distanceKm"])
	}
}

// csv is a projection of the json record (§4), so the two columns carry
// the same values under this codec's declared camelCase → snake_case
// mapping, and an inapplicable one is an empty cell rather than a 0.
func TestWorkoutsCSVUnitColumns(t *testing.T) {
	withPosts(t, unitFixture)
	stdout, _, err := run(t, "workouts", "list", "--format", "csv")
	if err != nil {
		t.Fatalf("workouts list --format csv failed: %v", err)
	}
	header, rows := parseCSV(t, stdout)
	if len(rows) != 7 {
		t.Fatalf("got %d rows, want one per exercise's single set (7):\n%s", len(rows), stdout)
	}

	want := map[string][2]string{
		"WR": {"110.2311310925", ""},
		"AB": {"100", ""},
		"BR": {"25", ""},
		"WD": {"80", "0.4"},
		"DD": {"", "8.04672"},
		"ND": {"", ""},
		"ZZ": {"", ""},
	}
	seen := map[string]bool{}
	for _, row := range rows {
		exType := col(t, header, row, "exercise_types")
		seen[exType] = true
		w, ok := want[exType]
		if !ok {
			t.Fatalf("unexpected exercise_types %q", exType)
		}
		if got := col(t, header, row, "weight_lb"); got != w[0] {
			t.Errorf("%s: weight_lb = %q, want %q", exType, got, w[0])
		}
		if got := col(t, header, row, "distance_km"); got != w[1] {
			t.Errorf("%s: distance_km = %q, want %q", exType, got, w[1])
		}
	}
	for exType := range want {
		if !seen[exType] {
			t.Errorf("no csv row for exercise_types %q", exType)
		}
	}
}

// A set-less exercise and an exercise-less workout still produce a
// padded row, so the new columns must be empty there too rather than
// shifting the row's field count.
func TestWorkoutsCSVUnitColumnsOnPaddedRows(t *testing.T) {
	withPosts(t, awkwardPosts)
	stdout, _, err := run(t, "workouts", "list", "--format", "csv")
	if err != nil {
		t.Fatalf("workouts list --format csv failed: %v", err)
	}
	header, rows := parseCSV(t, stdout)
	// parseCSV already rejects a short record; assert the cells are empty
	// rather than carrying a neighbouring column's value.
	for _, i := range []int{2, 3} { // set-less exercise, exercise-less workout
		for _, c := range []string{"weight_lb", "distance_km"} {
			if got := col(t, header, rows[i], c); got != "" {
				t.Errorf("padded row %d: %s = %q, want empty", i, c, got)
			}
		}
	}
	// And the ordinary WR rows do carry the weight.
	if got := col(t, header, rows[0], "weight_lb"); got != "60.5" {
		t.Errorf("weight_lb = %q, want 60.5", got)
	}
	if got := col(t, header, rows[1], "weight_lb"); got != "102.5" {
		t.Errorf("string-typed weight_lb = %q, want 102.5", got)
	}
}

// §4 makes this a MUST, not a nicety: "the dispatch field and its full
// mapping MUST appear in `prime` under SUBCOMMANDS". Assert the mapping
// is there and complete — a code dropped from the table is a consumer
// left guessing, and the table is the only place the grandfathered bare
// fields' units are fixed (§9).
func TestPrimeDocumentsExerciseTypesDispatch(t *testing.T) {
	subcommands := sectionOfPrime(t, "SUBCOMMANDS")

	if !strings.Contains(subcommands, "exerciseTypes") {
		t.Error("prime's SUBCOMMANDS section does not name the dispatch field exerciseTypes")
	}
	for _, code := range []string{"WR", "AB", "BR", "WD", "DD", "ND"} {
		if !strings.Contains(subcommands, code) {
			t.Errorf("prime does not document exerciseTypes %q; §4 wants the full mapping", code)
		}
	}
	// The units themselves, which is what makes the mapping readable.
	for _, unit := range []string{"lb", "km", "sec", "reps"} {
		if !strings.Contains(subcommands, unit) {
			t.Errorf("prime's dispatch mapping does not mention %q", unit)
		}
	}
	// And the siblings, so a reader learns they exist from prime alone.
	for _, key := range []string{"weightLb", "distanceKm"} {
		if !strings.Contains(subcommands, key) {
			t.Errorf("prime does not document the %s sibling", key)
		}
	}
}

// sectionOfPrime returns primeText from the named heading up to the next
// all-caps heading at column 0, so a test can assert that something is
// documented in the section §4 actually names.
func sectionOfPrime(t *testing.T, heading string) string {
	t.Helper()
	lines := strings.Split(primeText, "\n")
	start := -1
	for i, l := range lines {
		if l == heading || strings.HasPrefix(l, heading+" ") {
			start = i
			continue
		}
		if start >= 0 && l != "" && l[0] != ' ' && l == strings.ToUpper(l) {
			return strings.Join(lines[start:i], "\n")
		}
	}
	if start < 0 {
		t.Fatalf("prime has no %s section", heading)
	}
	return strings.Join(lines[start:], "\n")
}

func numberOrEmpty(n *json.Number) string {
	if n == nil {
		return ""
	}
	return n.String()
}

func keysOf(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
