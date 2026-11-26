package cmd

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/quantcli/liftoff-export-cli/internal/stubapi"
)

// parseCSV reads s back with a strict reader: every record must have the
// header's field count, which is what a spreadsheet import assumes.
func parseCSV(t *testing.T, s string) (header []string, rows [][]string) {
	t.Helper()
	r := csv.NewReader(strings.NewReader(s))
	recs, err := r.ReadAll() // FieldsPerRecord 0: all records match the first
	if err != nil {
		t.Fatalf("output is not valid CSV: %v\n%s", err, s)
	}
	if len(recs) == 0 {
		t.Fatalf("CSV output has no header row; got %q", s)
	}
	return recs[0], recs[1:]
}

// col returns row's value under the named header column.
func col(t *testing.T, header, row []string, name string) string {
	t.Helper()
	for i, h := range header {
		if h == name {
			return row[i]
		}
	}
	t.Fatalf("no column %q in header %v", name, header)
	return ""
}

// useZone pins time.Local for one test so local-zone rendering is
// deterministic on any machine.
func useZone(t *testing.T, name string, offsetSeconds int) {
	t.Helper()
	prev := time.Local
	time.Local = time.FixedZone(name, offsetSeconds)
	t.Cleanup(func() { time.Local = prev })
}

// --- the codec --------------------------------------------------------------

// The values a naive strings.Join(",") writer corrupts. Each must survive
// a write → standard-reader round trip unchanged.
var awkwardCSVValues = []string{
	"plain",
	"",
	"comma, in the middle",
	`he said "heavy"`,
	`"fully quoted"`,
	"line one\nline two",
	"trailing newline\n",
	" leading space",
	"trailing space ",
	"tab\there",
	"ünïcödé — 深蹲 🏋️",
	"semi;colon|pipe",
	"=SUM(A1:A9)",
	"-5",
	`,",` + "\n",
}

func TestWriteCSVRoundTripsAwkwardValues(t *testing.T) {
	header := []string{"n", "value", "tail"}
	var rows [][]string
	for i, v := range awkwardCSVValues {
		rows = append(rows, []string{string(rune('a' + i)), v, "end"})
	}

	var buf bytes.Buffer
	if err := writeCSV(&buf, header, rows); err != nil {
		t.Fatalf("writeCSV: %v", err)
	}
	gotHeader, gotRows := parseCSV(t, buf.String())
	if !reflect.DeepEqual(gotHeader, header) {
		t.Errorf("header = %v, want %v", gotHeader, header)
	}
	if !reflect.DeepEqual(gotRows, rows) {
		t.Errorf("rows did not round-trip:\n got %q\nwant %q", gotRows, rows)
	}
}

// Pin the exact bytes, not just the round trip: RFC 4180 quoting (quote
// only when needed, double embedded quotes), LF terminators, no BOM.
func TestWriteCSVQuoting(t *testing.T) {
	var buf bytes.Buffer
	err := writeCSV(&buf, []string{"a", "b"}, [][]string{
		{"plain", "has,comma"},
		{`has"quote`, "has\nnewline"},
		{"", "深蹲"},
	})
	if err != nil {
		t.Fatalf("writeCSV: %v", err)
	}
	want := "a,b\n" +
		"plain,\"has,comma\"\n" +
		"\"has\"\"quote\",\"has\nnewline\"\n" +
		",深蹲\n"
	if got := buf.String(); got != want {
		t.Errorf("writeCSV bytes:\n got %q\nwant %q", got, want)
	}
}

// §4 empty result for csv: the header row and nothing else. A nil slice
// and an empty one must behave the same.
func TestWriteCSVEmptyIsHeaderOnly(t *testing.T) {
	for name, rows := range map[string][][]string{"nil": nil, "empty": {}} {
		t.Run(name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := writeCSV(&buf, []string{"date", "weight"}, rows); err != nil {
				t.Fatalf("writeCSV: %v", err)
			}
			if got, want := buf.String(), "date,weight\n"; got != want {
				t.Errorf("got %q, want %q", got, want)
			}
		})
	}
}

func TestCSVCellHelpers(t *testing.T) {
	useZone(t, "TEST", -5*3600)

	f, n, s := 174.5, 0, ""
	cases := []struct{ name, got, want string }{
		{"float integral", csvFloat(175), "175"},
		{"float fractional", csvFloat(174.5), "174.5"},
		{"float large has no exponent", csvFloat(12345678.25), "12345678.25"},
		{"float negative", csvFloat(-2.5), "-2.5"},
		{"nil float is empty", csvFloatPtr(nil), ""},
		{"float ptr", csvFloatPtr(&f), "174.5"},
		{"nil int is empty", csvIntPtr(nil), ""},
		{"zero int ptr is a real 0", csvIntPtr(&n), "0"},
		{"nil string is empty", csvStrPtr(nil), ""},
		{"empty string ptr", csvStrPtr(&s), ""},
		{"omitted zero float", csvOmitZeroFloat(0), ""},
		{"kept float", csvOmitZeroFloat(1.5), "1.5"},
		// 02:30Z is still the previous evening at UTC-5.
		{"timestamp to local zone", csvLocalTime("2026-06-08T02:30:00.000Z"), "2026-06-07T21:30:00-05:00"},
		{"date is the local day", csvLocalDate("2026-06-08T02:30:00.000Z"), "2026-06-07"},
		{"unparseable timestamp passes through", csvLocalTime("not a time"), "not a time"},
		{"unparseable date is empty", csvLocalDate("not a time"), ""},
		{"empty timestamp", csvLocalTime(""), ""},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, c.got, c.want)
		}
	}
}

// --- the data path, end to end ---------------------------------------------

// Every subcommand the stub can drive, with the header each must emit.
var csvSurfaces = []struct {
	args   []string
	header []string
}{
	{[]string{"workouts", "list"}, workoutsCSVHeader},
	{[]string{"workouts", "show", "2026-06-08"}, workoutsCSVHeader},
	{[]string{"workouts", "stats"}, statsCSVHeader},
	{[]string{"bodyweights", "list"}, []string{"date", "weight"}},
	{[]string{"bodyweights", "stats"}, bodyweightStatsCSVHeader},
}

func TestCSVSurfacesEmitHeaderAndRows(t *testing.T) {
	for _, s := range csvSurfaces {
		t.Run(strings.Join(s.args, " "), func(t *testing.T) {
			stdout, stderr, err := run(t, append(s.args, "--format", "csv")...)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if stderr != "" {
				t.Errorf("stderr should be empty on success, got %q", stderr)
			}
			header, rows := parseCSV(t, stdout)
			if !reflect.DeepEqual(header, s.header) {
				t.Errorf("header = %v, want %v", header, s.header)
			}
			if len(rows) == 0 {
				t.Errorf("expected data rows for the sample workout, got header only:\n%s", stdout)
			}
		})
	}
}

// §4: an empty result is exit 0 with the header row alone on stdout — no
// prose, and (unlike markdown, whose stdout is empty) no stderr notice is
// needed because the output is not silent.
func TestCSVSurfacesEmptyIsHeaderOnly(t *testing.T) {
	withPosts(t, stubapi.EmptyPosts)
	for _, s := range csvSurfaces {
		t.Run(strings.Join(s.args, " "), func(t *testing.T) {
			stdout, stderr, err := run(t, append(s.args, "--format", "csv")...)
			if err != nil {
				t.Fatalf("empty result must exit 0, got: %v", err)
			}
			if want := strings.Join(s.header, ",") + "\n"; stdout != want {
				t.Errorf("stdout = %q, want header only %q", stdout, want)
			}
			if stderr != "" {
				t.Errorf("stderr should be empty, got %q", stderr)
			}
		})
	}
}

// --exercise matching nothing is the other route to an empty result.
func TestCSVExerciseNoMatchIsHeaderOnly(t *testing.T) {
	for _, sub := range []string{"list", "stats"} {
		stdout, _, err := run(t, "workouts", sub, "--exercise", "zzznomatch", "--format", "csv")
		if err != nil {
			t.Fatalf("workouts %s: %v", sub, err)
		}
		if _, rows := parseCSV(t, stdout); len(rows) != 0 {
			t.Errorf("workouts %s: expected header only, got %q", sub, stdout)
		}
	}
}

func TestWorkoutsListCSVSampleRow(t *testing.T) {
	stdout, _, err := run(t, "workouts", "list", "--format", "csv")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	header, rows := parseCSV(t, stdout)
	if len(rows) != 2 {
		t.Fatalf("sample workout has 2 sets, so 2 rows; got %d:\n%s", len(rows), stdout)
	}
	want := map[string]string{
		"id":                       "stub-post-1",
		"date":                     "2026-06-08",
		"session_duration":         "01 hours 06 minutes 01 seconds",
		"session_duration_seconds": "3961",
		"bodyweight":               "175",
		"calories_burned":          "420",
		"pr_count":                 "1",
		"exercise_index":           "0",
		"exercise_name":            "Bench Press",
		"exercise_types":           "WR",
		"set_index":                "1",
		"set_type":                 "normal",
		"input_one":                "100",
		"input_two":                "5",
	}
	for name, w := range want {
		if got := col(t, header, rows[1], name); got != w {
			t.Errorf("%s = %q, want %q", name, got, w)
		}
	}
	// §2: csv timestamps are local, with an offset — not the upstream "Z" string.
	started := col(t, header, rows[0], "started_at")
	ts, perr := time.Parse(time.RFC3339, started)
	if perr != nil {
		t.Fatalf("started_at %q is not RFC3339: %v", started, perr)
	}
	if !ts.Equal(time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC)) {
		t.Errorf("started_at %q is not the instant the API sent", started)
	}
	if want := ts.Local().Format(time.RFC3339); started != want {
		t.Errorf("started_at = %q, want local-zone %q", started, want)
	}
}

// awkwardPosts exercises the values that break hand-rolled CSV, through
// the real fetch → render path: free-text notes with a comma, quotes, a
// line break and non-ASCII; a workout with no recorded bodyweight and an
// unparseable duration (JSON null → empty cell); an exercise with no
// sets; a workout with no exercises; fractional and string-typed set
// inputs.
var awkwardPosts = json.RawMessage(`[
  {
    "id": "awk-1",
    "startedAt": "2026-06-08T12:00:00.000Z",
    "postedAt": "2026-06-08T13:00:00.000Z",
    "sessionDuration": "",
    "sessionNotes": "Felt \"strong\", shoulder ok\nPR attempt — 深蹲 🏋️",
    "bodyweight": "",
    "caloriesBurned": 0,
    "prCount": 0,
    "exerciseData": [
      {
        "exerciseIndex": 0,
        "exerciseName": "Squat, High-Bar",
        "exerciseId": "ex-1",
        "exerciseTypes": "WR",
        "exerciseNotes": "pause 2\" above parallel",
        "setsData": [
          {"setIndex": 0, "setType": "warmup", "inputOne": 60.5, "inputTwo": 8},
          {"setIndex": 1, "setType": "normal", "inputOne": "102.5", "inputTwo": "5"}
        ]
      },
      {
        "exerciseIndex": 1,
        "exerciseName": "Stretching",
        "exerciseId": "ex-2",
        "exerciseTypes": "ND",
        "exerciseNotes": "",
        "setsData": []
      }
    ]
  },
  {
    "id": "awk-2",
    "startedAt": "2026-06-09T12:00:00.000Z",
    "postedAt": "2026-06-09T12:30:00.000Z",
    "sessionDuration": "30 minutes 00 seconds",
    "sessionNotes": "",
    "bodyweight": "174.5",
    "caloriesBurned": 0,
    "prCount": 0,
    "exerciseData": []
  }
]`)

func TestWorkoutsListCSVAwkwardValues(t *testing.T) {
	withPosts(t, awkwardPosts)
	stdout, _, err := run(t, "workouts", "list", "--format", "csv")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	header, rows := parseCSV(t, stdout)
	// 2 sets + 1 set-less exercise + 1 exercise-less workout.
	if len(rows) != 4 {
		t.Fatalf("got %d rows, want 4:\n%s", len(rows), stdout)
	}

	notes := "Felt \"strong\", shoulder ok\nPR attempt — 深蹲 🏋️"
	for i := 0; i < 3; i++ {
		if got := col(t, header, rows[i], "session_notes"); got != notes {
			t.Errorf("row %d session_notes = %q, want %q", i, got, notes)
		}
		// No bodyweight recorded and no parseable duration: JSON emits
		// null for both, so csv must not print 0.
		if got := col(t, header, rows[i], "bodyweight"); got != "" {
			t.Errorf("row %d bodyweight = %q, want empty cell for a missing value", i, got)
		}
		if got := col(t, header, rows[i], "session_duration_seconds"); got != "" {
			t.Errorf("row %d session_duration_seconds = %q, want empty cell", i, got)
		}
		// A real zero is still a zero.
		if got := col(t, header, rows[i], "calories_burned"); got != "0" {
			t.Errorf("row %d calories_burned = %q, want 0", i, got)
		}
	}

	if got := col(t, header, rows[0], "exercise_name"); got != "Squat, High-Bar" {
		t.Errorf("exercise_name = %q", got)
	}
	if got := col(t, header, rows[0], "exercise_notes"); got != `pause 2" above parallel` {
		t.Errorf("exercise_notes = %q", got)
	}
	if got := col(t, header, rows[0], "input_one"); got != "60.5" {
		t.Errorf("fractional input_one = %q, want 60.5", got)
	}
	if got := col(t, header, rows[1], "input_one"); got != "102.5" {
		t.Errorf("string-typed input_one = %q, want 102.5", got)
	}

	// The set-less exercise keeps its row; set columns are empty.
	if got := col(t, header, rows[2], "exercise_name"); got != "Stretching" {
		t.Errorf("set-less exercise row: exercise_name = %q", got)
	}
	for _, c := range []string{"set_index", "set_type", "input_one", "input_two"} {
		if got := col(t, header, rows[2], c); got != "" {
			t.Errorf("set-less exercise row: %s = %q, want empty", c, got)
		}
	}

	// The exercise-less workout keeps its row; exercise and set columns
	// are empty, workout columns are filled.
	if got := col(t, header, rows[3], "id"); got != "awk-2" {
		t.Errorf("exercise-less workout row: id = %q", got)
	}
	if got := col(t, header, rows[3], "bodyweight"); got != "174.5" {
		t.Errorf("exercise-less workout row: bodyweight = %q", got)
	}
	if got := col(t, header, rows[3], "session_duration_seconds"); got != "1800" {
		t.Errorf("exercise-less workout row: session_duration_seconds = %q", got)
	}
	for _, c := range []string{"exercise_index", "exercise_name", "set_index", "input_two"} {
		if got := col(t, header, rows[3], c); got != "" {
			t.Errorf("exercise-less workout row: %s = %q, want empty", c, got)
		}
	}
}

func TestWorkoutsStatsCSV(t *testing.T) {
	withPosts(t, awkwardPosts)
	stdout, _, err := run(t, "workouts", "stats", "--format", "csv")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	header, rows := parseCSV(t, stdout)
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want one per exercise session (2):\n%s", len(rows), stdout)
	}

	squat := rows[0]
	for name, want := range map[string]string{
		"name":        "Squat, High-Bar",
		"type":        "WR",
		"date":        "2026-06-08",
		"bodyweight":  "", // none recorded on that workout
		"sets":        "1",
		"reps":        "5",
		"best_weight": "102.5",
		"best_reps":   "5",
		"volume":      "512.5",
		"duration":    "", // null in JSON: not a duration exercise
		"distance":    "",
	} {
		if got := col(t, header, squat, name); got != want {
			t.Errorf("squat %s = %q, want %q", name, got, want)
		}
	}

	// An ND exercise measures none of these stats; all five are empty
	// cells, not zeros.
	stretch := rows[1]
	for _, c := range []string{"best_weight", "best_reps", "volume", "duration", "distance"} {
		if got := col(t, header, stretch, c); got != "" {
			t.Errorf("stretching %s = %q, want empty", c, got)
		}
	}
}

// --detail only changes the markdown renderer; csv, like json, ignores it.
func TestWorkoutsStatsCSVIgnoresDetail(t *testing.T) {
	plain, _, err := run(t, "workouts", "stats", "--format", "csv")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	detail, _, err := run(t, "workouts", "stats", "--format", "csv", "--detail")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plain != detail {
		t.Errorf("--detail changed csv output:\n%q\nvs\n%q", plain, detail)
	}
}

func TestBodyweightsListCSV(t *testing.T) {
	withPosts(t, awkwardPosts)
	stdout, _, err := run(t, "bodyweights", "list", "--format", "csv")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Only awk-2 recorded a bodyweight; the fractional value keeps its
	// decimal and gains no unit suffix.
	if want := "date,weight\n2026-06-09,174.5\n"; stdout != want {
		t.Errorf("got %q, want %q", stdout, want)
	}
}

func TestBodyweightStatsCSV(t *testing.T) {
	rate := -1.5
	stats := &bodyweightStatsJSON{
		Current: 174.5, High: 180, Low: 174.5, Change: -5.5,
		RatePerMonth: &rate,
		Plateau:      &plateauJSON{Since: "Apr 2026", Avg: 175.25, Stddev: 0.5},
		Months: []monthAvgJSON{
			{Month: "2026-04", Avg: 176},
			{Month: "2026-05", Avg: 175.25},
		},
	}
	var buf bytes.Buffer
	if err := writeBodyweightStatsCSV(&buf, stats); err != nil {
		t.Fatalf("writeBodyweightStatsCSV: %v", err)
	}
	want := "current,high,low,change,rate_per_month,plateau_since,plateau_avg,plateau_stddev,month,month_avg\n" +
		"174.5,180,174.5,-5.5,-1.5,Apr 2026,175.25,0.5,2026-04,176\n" +
		"174.5,180,174.5,-5.5,-1.5,Apr 2026,175.25,0.5,2026-05,175.25\n"
	if got := buf.String(); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}

	// JSON omits ratePerMonth and plateau when they do not apply; csv
	// leaves those cells empty.
	stats.RatePerMonth, stats.Plateau = nil, nil
	stats.Months = stats.Months[:1]
	buf.Reset()
	if err := writeBodyweightStatsCSV(&buf, stats); err != nil {
		t.Fatalf("writeBodyweightStatsCSV: %v", err)
	}
	header, rows := parseCSV(t, buf.String())
	if len(rows) != 1 {
		t.Fatalf("got %d rows, want 1", len(rows))
	}
	for _, c := range []string{"rate_per_month", "plateau_since", "plateau_avg", "plateau_stddev"} {
		if got := col(t, header, rows[0], c); got != "" {
			t.Errorf("%s = %q, want empty when JSON omits it", c, got)
		}
	}
}

// --- routines ---------------------------------------------------------------

// The stub API does not serve the presets procedure, so routines are
// tested at the row builder, which is everything between fetch and
// writeCSV.
func TestRoutinesCSV(t *testing.T) {
	useZone(t, "TEST", 0)
	str := func(s string) *string { return &s }

	unfiled := Preset{
		ID: "p1", Name: `Push "A", heavy`, CreatedAt: "2026-01-02T03:04:05.000Z",
		IsFavorite: true, Completed: 12, AvgDuration: 3600,
		ExerciseData: []PresetExerciseData{
			{
				ExerciseIndex: 0, ExerciseName: "Bench Press", ExerciseID: "e1", ExerciseTypes: "WR",
				ExerciseNotes: str("Left only,\nthen right"),
				SetsData: []PresetSetData{
					{SetIndex: 0, SetType: "normal", InputOne: "100", InputTwo: "5"},
					{SetIndex: 1, SetType: "normal", InputOne: "0", InputTwo: "0"},
				},
			},
			{ExerciseIndex: 1, ExerciseName: "Plank", ExerciseID: "e2", ExerciseTypes: "ND"},
		},
	}
	foldered := Preset{ID: "p2", Name: "Rücken", FolderID: str("f1")}
	resp := &presetsResponse{
		PresetsWithoutFolder: []Preset{unfiled},
		Folders:              []Folder{{ID: "f1", Name: "Pull, days", Presets: []Preset{foldered}}},
	}

	var buf bytes.Buffer
	if err := writeRoutinesCSV(&buf, routineCSVRows(resp)); err != nil {
		t.Fatalf("writeRoutinesCSV: %v", err)
	}
	header, rows := parseCSV(t, buf.String())
	if !reflect.DeepEqual(header, routinesCSVHeader) {
		t.Errorf("header = %v", header)
	}
	// 2 sets + 1 set-less exercise + 1 exercise-less routine.
	if len(rows) != 4 {
		t.Fatalf("got %d rows, want 4:\n%s", len(rows), buf.String())
	}

	for name, want := range map[string]string{
		"folder_id":      "", // null folderId
		"folder_name":    "", // unfiled: no "My Routines" label in data
		"id":             "p1",
		"name":           `Push "A", heavy`,
		"created_at":     "2026-01-02T03:04:05Z",
		"is_favorite":    "true",
		"completed":      "12",
		"avg_duration":   "3600",
		"exercise_name":  "Bench Press",
		"superset":       "", // null
		"exercise_notes": "Left only,\nthen right",
		"set_index":      "0",
		"input_one":      "100",
		"input_two":      "5",
	} {
		if got := col(t, header, rows[0], name); got != want {
			t.Errorf("row 0 %s = %q, want %q", name, got, want)
		}
	}
	// Placeholder all-zero sets are data in csv, even though markdown hides them.
	if got := col(t, header, rows[1], "input_one"); got != "0" {
		t.Errorf("placeholder set input_one = %q, want 0", got)
	}
	if got := col(t, header, rows[2], "exercise_name"); got != "Plank" {
		t.Errorf("set-less exercise row: exercise_name = %q", got)
	}
	if got := col(t, header, rows[2], "set_index"); got != "" {
		t.Errorf("set-less exercise row: set_index = %q, want empty", got)
	}
	for name, want := range map[string]string{
		"folder_id": "f1", "folder_name": "Pull, days", "id": "p2", "name": "Rücken",
		"is_favorite": "false", "exercise_name": "", "set_index": "",
	} {
		if got := col(t, header, rows[3], name); got != want {
			t.Errorf("foldered row %s = %q, want %q", name, got, want)
		}
	}

	// `routines show` resolves the folder name for the one routine it prints.
	if got := folderNameFor(resp, foldered); got != "Pull, days" {
		t.Errorf("folderNameFor(foldered) = %q", got)
	}
	if got := folderNameFor(resp, unfiled); got != "" {
		t.Errorf("folderNameFor(unfiled) = %q, want empty", got)
	}

	// An account with no routines: header only.
	buf.Reset()
	if err := writeRoutinesCSV(&buf, routineCSVRows(&presetsResponse{})); err != nil {
		t.Fatalf("writeRoutinesCSV: %v", err)
	}
	if want := strings.Join(routinesCSVHeader, ",") + "\n"; buf.String() != want {
		t.Errorf("empty routines = %q, want %q", buf.String(), want)
	}
}

// --- the flag surface -------------------------------------------------------

// Every subcommand that takes --format must document csv, and what it
// advertises must match what it accepts (the mismatch #26 was filed for).
func TestEveryFormatFlagDocumentsCSV(t *testing.T) {
	n := 0
	for _, group := range rootCmd.Commands() {
		for _, leaf := range group.Commands() {
			f := leaf.Flags().Lookup("format")
			if f == nil {
				continue
			}
			n++
			if !strings.Contains(f.Usage, "csv") {
				t.Errorf("%s %s: --format usage %q does not mention csv", group.Name(), leaf.Name(), f.Usage)
			}
		}
	}
	if n != 7 {
		t.Errorf("found %d subcommands with --format, want 7; update the csv wiring and this count together", n)
	}
}

func TestValidateFormatAcceptsCSV(t *testing.T) {
	if got, err := validateFormat("csv"); err != nil || got != "csv" {
		t.Errorf(`validateFormat("csv") = %q, %v`, got, err)
	}
	_, err := validateFormat("yaml")
	if err == nil || !strings.Contains(err.Error(), "markdown, json, or csv") {
		t.Errorf("unknown-format error should list all three codecs, got %v", err)
	}
}

func TestPrimeDocumentsCSV(t *testing.T) {
	stdout, _, err := run(t, "prime")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout, "csv") {
		t.Error("prime does not mention --format csv")
	}
}
