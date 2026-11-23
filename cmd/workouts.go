package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/quantcli/liftoff-export-cli/internal/client"
	"github.com/spf13/cobra"
)

var workoutsCmd = &cobra.Command{
	Use:   "workouts",
	Short: "Workout commands",
}

// SetData mirrors the setsData array items.
//
// Both inputs are polymorphic, dispatched on the enclosing exercise's
// exerciseTypes (fitdownSetLine holds the full mapping):
//
//	WR/BR/AB  inputOne = weight, assistance or added weight in POUNDS
//	          inputTwo = reps
//	WD        inputOne = pounds, inputTwo = kilometres
//	DD        inputOne = kilometres, inputTwo = seconds
//	ND        inputTwo = seconds; inputOne is unused
//
// Pounds, not kilograms: every weight this CLI prints is labelled lbs,
// and bodyweight — which AB and BR arithmetic adds to these values — is
// pounds upstream.
type SetData struct {
	SetIndex int         `json:"setIndex"`
	SetType  string      `json:"setType"`
	InputOne json.Number `json:"inputOne"` // pounds or kilometres, per exerciseTypes
	InputTwo json.Number `json:"inputTwo"` // reps, kilometres or seconds, per exerciseTypes
}

// setDataWithUnits is the shape `workouts list`/`show --format json`
// actually emits for a set: SetData plus the two unit-named siblings
// CONTRACT §4 asks for beside a polymorphic field.
//
//	"the dispatch field and its full mapping MUST appear in `prime`
//	 under SUBCOMMANDS, and the preferred fix is additive unit-named
//	 siblings (`weight_lb` / `distance_km`) beside the polymorphic field"
//	 — §4, Units and cross-codec parity
//
// Spelled weightLb/distanceKm rather than weight_lb/distance_km: §4 fixes
// the field *name* (unit in the key) and leaves the case convention to
// each CLI, declared in `prime` — liftoff's json is camelCase throughout
// and maps camelCase → snake_case for csv, so the csv columns are
// weight_lb / distance_km. compat/parity's CSVColumnsExistInJSON
// compares names ignoring case and `_`, so both spellings are one field
// to the conformance bundle. The precedent in this file is
// sessionDurationSeconds, the §4 sibling added for a display phrase.
//
// Both keys are always present, `null` where the exercise type has no
// such quantity (§4 record shape: a field with no value is present and
// null, not omitted). They declare the *unit* of an input, not its
// meaning: for AB the pounds number is assistance subtracted from
// bodyweight and for BR it is load added to it, so what a weight means
// still comes from exerciseTypes. The bare inputOne/inputTwo are
// untouched — §9 forbids redefining a field under its existing name.
type setDataWithUnits struct {
	SetData
	WeightLb   *json.Number `json:"weightLb"`
	DistanceKm *json.Number `json:"distanceKm"`
}

// setUnitFields resolves one set's polymorphic inputs into the pounds
// and kilometres siblings, dispatched on the enclosing exercise's
// exerciseTypes (the table on SetData above is the full mapping).
//
// An exerciseTypes code this CLI does not know about yields two nulls.
// We cannot say what its inputs mean, and a guess here is the exact
// failure the siblings exist to prevent — the bare inputOne/inputTwo
// still carry the values, and `prime` names the codes we do handle.
func setUnitFields(exTypes string, s SetData) (weightLb, distanceKm *json.Number) {
	switch exTypes {
	case "WR", "AB", "BR":
		return numberPtr(s.InputOne), nil
	case "WD":
		return numberPtr(s.InputOne), numberPtr(s.InputTwo)
	case "DD":
		return nil, numberPtr(s.InputOne)
	default: // ND has neither; an unknown code gets no guess.
		return nil, nil
	}
}

// numberPtr carries a set input into a sibling field unchanged — same
// digits, no arithmetic (§4: "A codec MUST NOT convert on the way out",
// so 110.2311310925 lb stays 110.2311310925 lb). An absent or
// non-numeric input yields nil, which marshals as null; returning the
// raw value there would emit an empty token and produce invalid JSON.
func numberPtr(n json.Number) *json.Number {
	if _, err := n.Float64(); err != nil {
		return nil
	}
	return &n
}

// setsWithUnits decorates every set of one exercise. The result is
// always non-nil so an exercise with no sets still marshals as `[]` and
// never `null`, independently of normalizePostCollections below.
func setsWithUnits(exTypes string, sets []SetData) []setDataWithUnits {
	out := make([]setDataWithUnits, 0, len(sets))
	for _, s := range sets {
		weightLb, distanceKm := setUnitFields(exTypes, s)
		out = append(out, setDataWithUnits{SetData: s, WeightLb: weightLb, DistanceKm: distanceKm})
	}
	return out
}

// ExerciseData mirrors the exerciseData array items.
type ExerciseData struct {
	ExerciseIndex int       `json:"exerciseIndex"`
	ExerciseName  string    `json:"exerciseName"`
	ExerciseID    string    `json:"exerciseId"`
	ExerciseTypes string    `json:"exerciseTypes"` // WR=weight/reps, DD=distance/duration, ND=no data
	ExerciseNotes string    `json:"exerciseNotes"`
	SetsData      []SetData `json:"setsData"`
}

// MarshalJSON replaces setsData with the unit-decorated form. The
// dispatch field lives on the exercise, not the set, so this is the
// shallowest place that knows both — a SetData cannot decorate itself.
func (e ExerciseData) MarshalJSON() ([]byte, error) {
	// alias drops ExerciseData's methods, so this Marshal call is not
	// recursive. The outer setsData field shadows the embedded one (same
	// tag, shallower depth wins).
	type alias ExerciseData
	return json.Marshal(struct {
		alias
		SetsData []setDataWithUnits `json:"setsData"`
	}{
		alias:    alias(e),
		SetsData: setsWithUnits(e.ExerciseTypes, e.SetsData),
	})
}

// Post is a Liftoff workout post.
type Post struct {
	ID              string         `json:"id"`
	StartedAt       string         `json:"startedAt"`
	PostedAt        string         `json:"postedAt"`
	SessionDuration string         `json:"sessionDuration"`
	SessionNotes    string         `json:"sessionNotes"`
	Bodyweight      string         `json:"bodyweight"`
	CaloriesBurned  int            `json:"caloriesBurned"`
	PRCount         int            `json:"prCount"`
	ExerciseData    []ExerciseData `json:"exerciseData"`
}

// MarshalJSON normalizes the field types in `workouts list`/`show` JSON so they
// match the rest of the contract. The Liftoff API delivers two fields in
// awkward forms that we keep decoding as strings but emit more usefully:
//
//   - bodyweight arrives as a quoted string ("175"); we emit it as a JSON
//     number (or null when absent/unparseable) so it matches the numeric
//     bodyweight in `workouts stats`. (#33)
//   - sessionDuration is a human phrase ("01 hours 06 minutes 01 seconds").
//     We keep it for display but add a numeric sessionDurationSeconds so
//     consumers can do arithmetic without parsing prose. (#36)
func (p Post) MarshalJSON() ([]byte, error) {
	// alias drops Post's methods, so this Marshal call is not recursive. The
	// outer bodyweight field shadows the embedded string field (same tag,
	// shallower depth wins), replacing it with a number/null.
	type alias Post
	return json.Marshal(struct {
		alias
		Bodyweight             *float64 `json:"bodyweight"`
		SessionDurationSeconds *int     `json:"sessionDurationSeconds"`
	}{
		alias:                  alias(p),
		Bodyweight:             parseBodyweightValue(p.Bodyweight),
		SessionDurationSeconds: parseDurationSeconds(p.SessionDuration),
	})
}

// parseBodyweightValue converts the API's string bodyweight to a number.
// Empty or unparseable values yield nil, which marshals as null.
func parseBodyweightValue(s string) *float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil
	}
	return &f
}

// parseDurationSeconds parses Liftoff's "NN hours NN minutes NN seconds"
// session-duration phrase into a total number of seconds. It tolerates a
// missing unit group (e.g. "45 minutes 30 seconds") but returns nil on any
// shape it does not recognize rather than guessing.
func parseDurationSeconds(s string) *int {
	fields := strings.Fields(s)
	if len(fields) < 2 || len(fields)%2 != 0 {
		return nil
	}
	total := 0
	for i := 0; i+1 < len(fields); i += 2 {
		n, err := strconv.Atoi(fields[i])
		if err != nil {
			return nil
		}
		switch strings.ToLower(strings.TrimSuffix(fields[i+1], "s")) {
		case "hour":
			total += n * 3600
		case "minute", "min":
			total += n * 60
		case "second", "sec":
			total += n
		default:
			return nil
		}
	}
	return &total
}

// fetchPosts loads the user's own workouts — the app's Memories view —
// and normalizes their collections on the way through.
//
// post.getMyPosts is the one upstream call behind `workouts list`,
// `workouts show`, `workouts stats`, `bodyweights list` and `bodyweights
// stats`, so the normalization lives here rather than at each call site.
// Routing every caller through one function is also what keeps the
// invariant checkable: grep for the procedure name and there is exactly
// one hit.
func fetchPosts() ([]Post, error) {
	c := client.New()
	var posts []Post
	if err := c.Query("post.getMyPosts", nil, &posts); err != nil {
		return nil, err
	}
	for i := range posts {
		normalizePostCollections(&posts[i])
	}
	return posts, nil
}

// normalizePostCollections allocates every nil slice reachable from a
// Post, so `--format json` emits `[]` and never `null`.
//
// CONTRACT §4 record shape: "An empty collection is `[]`, never `null`."
// In Go that is a default rather than a decision — a field declared []T
// marshals as `null` when the slice is nil, which is what the decoder
// leaves behind when upstream sends `null` or omits the key for a
// workout logged with no exercises. printJSON already covers the
// top-level slice; these are the nested ones it cannot see.
//
// The cost of getting it wrong is out of proportion to the cause: a
// consumer that recognizes one of our records by duck-typing this field
// (`Array.isArray(exerciseData)`) fails recognition rather than one
// value, and one that validates the whole array before accepting it then
// rejects the entire export over a single empty element.
func normalizePostCollections(p *Post) {
	if p.ExerciseData == nil {
		p.ExerciseData = []ExerciseData{}
	}
	for i := range p.ExerciseData {
		if p.ExerciseData[i].SetsData == nil {
			p.ExerciseData[i].SetsData = []SetData{}
		}
	}
}

var listFormatFlag string
var listSinceFlag string
var listUntilFlag string
var listExerciseFlag string

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all your workouts",
	RunE: func(cmd *cobra.Command, args []string) error {
		format, err := validateFormat(listFormatFlag)
		if err != nil {
			return err
		}
		since, err := parseDateValue(listSinceFlag)
		if err != nil {
			return err
		}
		until, err := parseUntilValue(listUntilFlag)
		if err != nil {
			return err
		}
		if printed, err := printResolvedWindow(cmd, since, until); printed || err != nil {
			return err
		}
		posts, err := fetchPosts()
		if err != nil {
			return err
		}
		posts = filterByWindow(posts, since, until)
		if listExerciseFlag != "" {
			posts = filterExercises(posts, listExerciseFlag)
		}
		if format == "json" {
			return printJSON(cmd.OutOrStdout(), posts)
		}
		if format == "csv" {
			// Header-only when posts is empty; see writeCSV.
			return writeWorkoutsCSV(cmd.OutOrStdout(), posts)
		}
		if len(posts) == 0 {
			// §4: no rows on stdout, notice on stderr. (#24, #37)
			emptyNotice(cmd, "No workouts found.")
			return nil
		}
		return renderWorkoutsFitdown(cmd.OutOrStdout(), posts)
	},
}

// validateFormat checks --format against the quantcli shared output set
// (CONTRACT §4) and normalizes it to a canonical value. Liftoff currently
// supports markdown (the default; fitdown-style) and json. "md" is accepted
// as an alias.
//
// The empty string is rejected rather than silently treated as markdown
// (#32). Every --format flag in this CLI is declared with a "markdown"
// default, so "" can only reach here from an explicit `--format ""` —
// almost always an unset shell variable (`--format "$FMT"`), where
// quietly emitting markdown hands a jq pipeline prose instead of failing.
func validateFormat(format string) (string, error) {
	switch format {
	case "markdown", "md":
		return "markdown", nil
	case "json":
		return "json", nil
	case "csv":
		return "csv", nil
	case "":
		return "", fmt.Errorf(`--format was given an empty value (use markdown, json, or csv)`)
	default:
		return "", fmt.Errorf("unknown --format %q (use markdown, json, or csv)", format)
	}
}

// parseDateValue parses --since / --until / show argument values.
// Accepted forms: "today", "yesterday", absolute YYYY-MM-DD, or relative
// Nd/Nw/Nm/Ny. Returns local midnight for the target day; empty string
// yields the zero time.
func parseDateValue(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)

	switch strings.ToLower(s) {
	case "today":
		return today, nil
	case "yesterday":
		return today.AddDate(0, 0, -1), nil
	}
	if t, err := time.ParseInLocation("2006-01-02", s, time.Local); err == nil {
		return t, nil
	}
	if len(s) < 2 {
		return time.Time{}, fmt.Errorf("invalid date %q (use YYYY-MM-DD, today, yesterday, or Nd/Nw/Nm/Ny)", s)
	}
	n := 0
	if _, err := fmt.Sscanf(s[:len(s)-1], "%d", &n); err != nil {
		return time.Time{}, fmt.Errorf("invalid date %q (use YYYY-MM-DD, today, yesterday, or Nd/Nw/Nm/Ny)", s)
	}
	switch s[len(s)-1] {
	case 'd':
		return today.AddDate(0, 0, -n), nil
	case 'w':
		return today.AddDate(0, 0, -n*7), nil
	case 'm':
		return today.AddDate(0, -n, 0), nil
	case 'y':
		return today.AddDate(-n, 0, 0), nil
	default:
		return time.Time{}, fmt.Errorf("invalid date unit %q: use d, w, m, or y", string(s[len(s)-1]))
	}
}

// parseUntilValue resolves --until to the exclusive upper bound of a half-open
// window. The user-supplied date names a calendar day they expect to be
// included, so we add 24h to the parsed start-of-day. Empty string yields the
// zero time, which callers treat as "no upper bound".
func parseUntilValue(s string) (time.Time, error) {
	t, err := parseDateValue(s)
	if err != nil || t.IsZero() {
		return t, err
	}
	return t.AddDate(0, 0, 1), nil
}

// filterByWindow keeps posts with StartedAt in [since, until). Either bound
// being zero disables that side. RFC3339Nano parse failures are kept (the
// user can decide what to do with malformed timestamps downstream).
func filterByWindow(posts []Post, since, until time.Time) []Post {
	if since.IsZero() && until.IsZero() {
		return posts
	}
	out := posts[:0]
	for _, p := range posts {
		t, err := time.Parse(time.RFC3339Nano, p.StartedAt)
		if err != nil {
			out = append(out, p)
			continue
		}
		if !since.IsZero() && t.Before(since) {
			continue
		}
		if !until.IsZero() && !t.Before(until) {
			continue
		}
		out = append(out, p)
	}
	return out
}

var showFormatFlag string

var showCmd = &cobra.Command{
	Use:   "show <date>",
	Short: "Show workout(s) for a given date (e.g. 2025-03-08, today, yesterday)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		format, err := validateFormat(showFormatFlag)
		if err != nil {
			return err
		}
		target, err := parseDateValue(args[0])
		if err != nil {
			return err
		}
		if target.IsZero() {
			return fmt.Errorf("date argument is required")
		}

		posts, err := fetchPosts()
		if err != nil {
			return err
		}

		var matched []Post
		for _, p := range posts {
			t, err := time.Parse(time.RFC3339Nano, p.StartedAt)
			if err != nil {
				continue
			}
			if t.Local().Format("2006-01-02") == target.Format("2006-01-02") {
				matched = append(matched, p)
			}
		}

		if len(matched) == 0 {
			// §4: an empty result is `[]` on stdout for json and no rows
			// for markdown, exit 0 either way. Printing the prose line on
			// stdout broke both codecs — `show --format json` emitted
			// text that no JSON parser accepts. (#23)
			if format == "json" {
				return printJSON(cmd.OutOrStdout(), []Post{})
			}
			if format == "csv" {
				return writeWorkoutsCSV(cmd.OutOrStdout(), nil)
			}
			emptyNotice(cmd, "No workouts found for %s.", target.Format("January 2, 2006"))
			return nil
		}

		if format == "json" {
			return printJSON(cmd.OutOrStdout(), matched)
		}
		if format == "csv" {
			return writeWorkoutsCSV(cmd.OutOrStdout(), matched)
		}
		return renderWorkoutsFitdown(cmd.OutOrStdout(), matched)
	},
}

func init() {
	workoutsCmd.AddCommand(listCmd)
	workoutsCmd.AddCommand(showCmd)
	showCmd.Flags().StringVar(&showFormatFlag, "format", "markdown",
		"Output format: markdown (default, fitdown-style), json, or csv")
	listCmd.Flags().StringVar(&listFormatFlag, "format", "markdown",
		"Output format: markdown (default, fitdown-style), json, or csv")
	listCmd.Flags().StringVar(&listSinceFlag, "since", "", "Filter workouts on or after date (today, yesterday, YYYY-MM-DD, or Nd/Nw/Nm/Ny)")
	listCmd.Flags().StringVar(&listUntilFlag, "until", "", "Filter workouts through date, inclusive (today, yesterday, YYYY-MM-DD, or Nd/Nw/Nm/Ny)")
	listCmd.Flags().StringVar(&listExerciseFlag, "exercise", "", "Filter to exercises matching this name (word-prefix match)")
}

// matchesExercise checks if every word in pattern matches a word-prefix in name.
// e.g. "pull up" matches "Assisted Pull Ups", "chin" matches "Assisted Chin Ups"
// but "chin" does not match "Machine Row".
func matchesExercise(name, pattern string) bool {
	nameWords := strings.Fields(strings.ToLower(name))
	patWords := strings.Fields(strings.ToLower(pattern))
	for _, pw := range patWords {
		found := false
		for _, nw := range nameWords {
			if strings.HasPrefix(nw, pw) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// filterExercises keeps only exercises matching pattern within each post,
// and drops posts that end up with no matching exercises.
func filterExercises(posts []Post, pattern string) []Post {
	var out []Post
	for _, p := range posts {
		var matched []ExerciseData
		for _, e := range p.ExerciseData {
			if matchesExercise(e.ExerciseName, pattern) {
				matched = append(matched, e)
			}
		}
		if len(matched) > 0 {
			p.ExerciseData = matched
			out = append(out, p)
		}
	}
	return out
}

func renderWorkoutsFitdown(w io.Writer, posts []Post) error {
	for i, post := range posts {
		var header string
		t, err := time.Parse(time.RFC3339Nano, post.StartedAt)
		if err != nil {
			header = fmt.Sprintf("Workout %s", post.StartedAt)
		} else {
			header = fmt.Sprintf("Workout %s", t.Local().Format("January 2, 2006"))
		}
		if post.SessionNotes != "" {
			header = fmt.Sprintf("%s (%s)", header, post.SessionNotes)
		}
		fmt.Fprintln(w, header)

		for _, ex := range post.ExerciseData {
			fmt.Fprintln(w)
			name := ex.ExerciseName
			if ex.ExerciseNotes != "" {
				name = fmt.Sprintf("%s (%s)", name, ex.ExerciseNotes)
			}
			fmt.Fprintln(w, name)

			var lines []string
			for _, s := range ex.SetsData {
				lines = append(lines, fitdownSetLine(ex.ExerciseTypes, s.InputOne, s.InputTwo))
			}

			// Compress consecutive identical lines into Nx... notation
			for i := 0; i < len(lines); {
				j := i + 1
				for j < len(lines) && lines[j] == lines[i] {
					j++
				}
				if n := j - i; n > 1 {
					fmt.Fprintf(w, "%dx%s\n", n, lines[i])
				} else {
					fmt.Fprintln(w, lines[i])
				}
				i = j
			}
		}

		if i < len(posts)-1 {
			fmt.Fprintln(w)
		}
	}
	return nil
}

// workoutsCSVHeader is the column set for `workouts list|show --format csv`:
// the Post, ExerciseData and SetData JSON keys in snake_case, plus `date`
// (the local calendar day, as in `workouts stats` and `bodyweights list`).
var workoutsCSVHeader = []string{
	"id", "date", "started_at", "posted_at",
	"session_duration", "session_duration_seconds", "session_notes",
	"bodyweight", "calories_burned", "pr_count",
	"exercise_index", "exercise_name", "exercise_id", "exercise_types", "exercise_notes",
	"set_index", "set_type", "input_one", "input_two",
	"weight_lb", "distance_km",
}

// writeWorkoutsCSV renders posts as one row per set, with the workout and
// exercise columns repeated on each row.
//
// input_one / input_two are the raw upstream pair, exactly as in --format
// json; their meaning depends on exercise_types (WR: weight, reps — see
// fitdownSetLine). They are not split into weight/reps/distance columns
// because an exercise type this CLI does not know about would then have
// nowhere to go.
//
// weight_lb / distance_km are the §4 unit-named siblings, projected from
// the json weightLb / distanceKm keys by this codec's declared
// camelCase → snake_case mapping, and carrying the same digits. §4 makes
// csv a projection of the json record and lets it omit a field, so these
// columns are a choice rather than an obligation; they are here because
// a spreadsheet is where someone sums a weight column by hand, which is
// the case most exposed to mistaking kilometres for pounds. Empty where
// the exercise type has no such quantity, per the empty-cell rule for an
// absent value.
//
// A workout with no exercises, or an exercise with no sets, still gets one
// row with the deeper columns empty, so it does not silently vanish from
// the export the way an inner join would drop it.
func writeWorkoutsCSV(w io.Writer, posts []Post) error {
	var rows [][]string
	for _, p := range posts {
		workout := []string{
			p.ID,
			csvLocalDate(p.StartedAt),
			csvLocalTime(p.StartedAt),
			csvLocalTime(p.PostedAt),
			p.SessionDuration,
			csvIntPtr(parseDurationSeconds(p.SessionDuration)),
			p.SessionNotes,
			csvFloatPtr(parseBodyweightValue(p.Bodyweight)),
			strconv.Itoa(p.CaloriesBurned),
			strconv.Itoa(p.PRCount),
		}
		if len(p.ExerciseData) == 0 {
			rows = append(rows, csvPad(workout, len(workoutsCSVHeader)))
			continue
		}
		for _, e := range p.ExerciseData {
			exercise := append(append([]string{}, workout...),
				strconv.Itoa(e.ExerciseIndex),
				e.ExerciseName,
				e.ExerciseID,
				e.ExerciseTypes,
				e.ExerciseNotes,
			)
			if len(e.SetsData) == 0 {
				rows = append(rows, csvPad(exercise, len(workoutsCSVHeader)))
				continue
			}
			for _, s := range e.SetsData {
				weightLb, distanceKm := setUnitFields(e.ExerciseTypes, s)
				rows = append(rows, append(append([]string{}, exercise...),
					strconv.Itoa(s.SetIndex),
					s.SetType,
					s.InputOne.String(),
					s.InputTwo.String(),
					csvNumberPtr(weightLb),
					csvNumberPtr(distanceKm),
				))
			}
		}
	}
	return writeCSV(w, workoutsCSVHeader, rows)
}
