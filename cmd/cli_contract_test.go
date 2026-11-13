package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/quantcli/liftoff-export-cli/internal/stubapi"
	"github.com/spf13/cobra"
)

// These tests drive the real cobra command tree end to end — flag parse,
// fetch, filter, render — against an in-process stub of the Liftoff tRPC
// API. They are the regression tests for the CONTRACT §4 output rules the
// compat/formats bundle attests at the binary level; keeping an in-process
// copy here means a §4 regression fails `go test ./...` on the author's
// laptop, not only in the compat CI job.

var stubSrv *stubapi.Server

func TestMain(m *testing.M) {
	stubSrv = stubapi.New(stubapi.SamplePosts)
	defer stubSrv.Close()

	// internal/auth resolves LIFTOFF_API_BASE under a sync.Once, so this
	// must be set before the first command runs and cannot change
	// afterwards — hence one server for the whole test binary, with the
	// payload swapped per case via stubSrv.SetPosts.
	os.Setenv("LIFTOFF_API_BASE", stubSrv.BaseURL())
	// Headless auth: any non-empty value sends GetToken down the
	// refresh-token path, which the stub answers. No token file is read
	// or written, so a developer's real ~/.config/liftoff-export is not
	// touched.
	os.Setenv("LIFTOFF_REFRESH_TOKEN", "stub-refresh-token")

	os.Exit(m.Run())
}

// run executes the CLI with args and returns stdout, stderr and the error
// Execute surfaced (which is what Execute() in root.go turns into a
// non-zero exit code).
func run(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	resetFlags(t, rootCmd)

	var outBuf, errBuf bytes.Buffer
	rootCmd.SetOut(&outBuf)
	rootCmd.SetErr(&errBuf)
	rootCmd.SetArgs(args)
	t.Cleanup(func() {
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
		rootCmd.SetArgs(nil)
	})

	err = rootCmd.Execute()
	return outBuf.String(), errBuf.String(), err
}

// resetFlags restores the flag state one Execute leaves behind. Flag
// values are bound to package-level variables and cobra does not reset
// them between runs, so without this a --format from one subtest leaks
// into the next.
func resetFlags(t *testing.T, _ *cobra.Command) {
	t.Helper()
	listFormatFlag, listSinceFlag, listUntilFlag, listExerciseFlag = "markdown", "", "", ""
	showFormatFlag = "markdown"
	statsFormatFlag, statsSinceFlag, statsUntilFlag, statsExerciseFlag = "markdown", "", "", ""
	statsDetailFlag = false
	bodyweightsListFormatFlag, bodyweightsListSinceFlag, bodyweightsListUntilFlag = "markdown", "", ""
	bodyweightsStatsFormatFlag, bodyweightsStatsSinceFlag, bodyweightsStatsUntilFlag = "markdown", "", ""
	routinesListFormatFlag, routinesShowFormatFlag = "markdown", "markdown"

	// print-resolved-window is not bound to a variable; it is read back
	// off the FlagSet, so it has to be cleared there.
	for _, c := range []*cobra.Command{listCmd, statsCmd, bodyweightsListCmd, bodyweightsStatsCmd} {
		if err := c.Flags().Set("print-resolved-window", "false"); err != nil {
			t.Fatalf("reset print-resolved-window on %s: %v", c.Name(), err)
		}
	}
}

// withPosts swaps the stub's post.getMyPosts payload for one test and
// restores the sample workout afterwards.
func withPosts(t *testing.T, posts json.RawMessage) {
	t.Helper()
	stubSrv.SetPosts(posts)
	t.Cleanup(func() { stubSrv.SetPosts(stubapi.SamplePosts) })
}

// --- §4 empty-result rules -------------------------------------------------

// `workouts show` on a day with no workouts printed the prose line
// "No workouts found for ..." on stdout regardless of --format, so
// `--format json` emitted something no JSON parser accepts. §4 requires
// `[]` and exit 0. (#23)
func TestWorkoutsShowEmptyJSONEmitsArray(t *testing.T) {
	stdout, stderr, err := run(t, "workouts", "show", "2001-01-01", "--format", "json")
	if err != nil {
		t.Fatalf("exit non-zero on empty result (§4 says exit 0): %v", err)
	}
	var rows []any
	if jsonErr := json.Unmarshal([]byte(stdout), &rows); jsonErr != nil {
		t.Fatalf("stdout is not JSON: %v; stdout=%q", jsonErr, stdout)
	}
	if len(rows) != 0 {
		t.Errorf("expected empty array, got %d rows: %q", len(rows), stdout)
	}
	if strings.Contains(stdout, "No workouts") {
		t.Errorf("prose leaked into the JSON data stream: %q", stdout)
	}
	_ = stderr
}

// The markdown half of #23: still no rows on stdout, and the human
// notice on stderr so an interactive user is not left staring at a blank
// terminal.
func TestWorkoutsShowEmptyMarkdownKeepsStdoutClean(t *testing.T) {
	stdout, stderr, err := run(t, "workouts", "show", "2001-01-01")
	if err != nil {
		t.Fatalf("exit non-zero on empty result: %v", err)
	}
	if stdout != "" {
		t.Errorf("§4 says no rows on stdout for an empty markdown result; got %q", stdout)
	}
	if !strings.Contains(stderr, "No workouts found for January 1, 2001.") {
		t.Errorf("expected the empty-result notice on stderr; got %q", stderr)
	}
}

// `workouts list` with a window that matches nothing printed nothing at
// all — indistinguishable from a crash for an interactive user. (#24)
func TestWorkoutsListEmptyMarkdownNotifiesOnStderr(t *testing.T) {
	stdout, stderr, err := run(t, "workouts", "list", "--since", "2001-01-01", "--until", "2001-01-02")
	if err != nil {
		t.Fatalf("exit non-zero on empty result: %v", err)
	}
	if stdout != "" {
		t.Errorf("expected no rows on stdout; got %q", stdout)
	}
	if !strings.Contains(stderr, "No workouts found.") {
		t.Errorf("expected the empty-result notice on stderr; got %q", stderr)
	}
}

// Same rule on the --exercise filter path, which reaches the empty case
// from a different direction. (#37)
func TestWorkoutsListExerciseNoMatchNotifiesOnStderr(t *testing.T) {
	stdout, stderr, err := run(t, "workouts", "list", "--exercise", "zercher squat")
	if err != nil {
		t.Fatalf("exit non-zero on empty result: %v", err)
	}
	if stdout != "" {
		t.Errorf("expected no rows on stdout; got %q", stdout)
	}
	if !strings.Contains(stderr, "No workouts found.") {
		t.Errorf("expected the empty-result notice on stderr; got %q", stderr)
	}
}

func TestWorkoutsListEmptyJSONEmitsArray(t *testing.T) {
	withPosts(t, stubapi.EmptyPosts)
	stdout, _, err := run(t, "workouts", "list", "--format", "json")
	if err != nil {
		t.Fatalf("exit non-zero on empty result: %v", err)
	}
	if strings.TrimSpace(stdout) != "[]" {
		t.Errorf("empty JSON result should be `[]`; got %q", stdout)
	}
}

// --- §4 "json is an array on stdout" ---------------------------------------

// Every --format json surface the compat bundle exercises must unmarshal
// as a JSON array. `bodyweights stats` used to emit a bare object, which
// is the one §4 shape violation the bundle turns red once the data path
// runs.
func TestJSONSurfacesAreArrays(t *testing.T) {
	for _, sub := range [][]string{
		{"workouts", "list"},
		{"workouts", "stats"},
		{"bodyweights", "list"},
		{"bodyweights", "stats"},
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
		})
	}
}

// The empty-account case has to stay an array too, not `null` and not an
// empty object.
func TestJSONSurfacesAreArraysWhenEmpty(t *testing.T) {
	withPosts(t, stubapi.EmptyPosts)
	for _, sub := range [][]string{
		{"workouts", "list"},
		{"workouts", "stats"},
		{"bodyweights", "list"},
		{"bodyweights", "stats"},
	} {
		name := strings.Join(sub, " ")
		t.Run(name, func(t *testing.T) {
			stdout, _, err := run(t, append(sub, "--format", "json")...)
			if err != nil {
				t.Fatalf("%s --format json failed on empty account: %v", name, err)
			}
			if strings.TrimSpace(stdout) != "[]" {
				t.Errorf("%s --format json on an empty account should be `[]`; got %q", name, stdout)
			}
		})
	}
}

// --- §4 default-is-markdown ------------------------------------------------

// The compat bundle's DefaultIsMarkdown check, in-process: omitting
// --format must be byte-identical to passing --format markdown.
func TestDefaultFormatMatchesMarkdown(t *testing.T) {
	for _, sub := range [][]string{
		{"workouts", "list"},
		{"workouts", "stats"},
		{"bodyweights", "list"},
		{"bodyweights", "stats"},
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

// --- #32 empty --format ----------------------------------------------------

func TestValidateFormatRejectsEmptyString(t *testing.T) {
	if _, err := validateFormat(""); err == nil {
		t.Error(`validateFormat("") should error, not silently return markdown (#32)`)
	}
	for _, ok := range []string{"markdown", "md", "json"} {
		if _, err := validateFormat(ok); err != nil {
			t.Errorf("validateFormat(%q) = %v, want no error", ok, err)
		}
	}
	if _, err := validateFormat("yaml"); err == nil {
		t.Error(`validateFormat("yaml") should error`)
	}
}

// End to end: `--format ""` must fail with a stderr message and no
// stdout, exactly like any other unknown value.
func TestEmptyFormatFlagFailsCleanly(t *testing.T) {
	stdout, _, err := run(t, "workouts", "list", "--format", "")
	if err == nil {
		t.Fatal(`--format "" was accepted; want an error (#32)`)
	}
	if stdout != "" {
		t.Errorf("a flag-validation failure must not write to stdout (§4); got %q", stdout)
	}
}

// --- markdown polish -------------------------------------------------------

// `bodyweights stats` labelled only Current, leaving High/Low looking
// unitless. (#35)
func TestBodyweightStatsMarkdownLabelsEveryWeight(t *testing.T) {
	stdout, _, err := run(t, "bodyweights", "stats")
	if err != nil {
		t.Fatalf("bodyweights stats failed: %v", err)
	}
	want := "  Current: 175 lbs | High: 175 lbs | Low: 175 lbs"
	if !strings.Contains(stdout, want) {
		t.Errorf("expected %q in output; got:\n%s", want, stdout)
	}
}

// `workouts stats` always said "N sessions", including "1 sessions". (#28)
func TestWorkoutsStatsPluralizesSessionCount(t *testing.T) {
	stdout, _, err := run(t, "workouts", "stats")
	if err != nil {
		t.Fatalf("workouts stats failed: %v", err)
	}
	if strings.Contains(stdout, "1 sessions") {
		t.Errorf(`"1 sessions" should read "1 session"; got:\n%s`, stdout)
	}
	if !strings.Contains(stdout, "Bench Press — 1 session\n") {
		t.Errorf("expected singular session count; got:\n%s", stdout)
	}
}

func TestCountNoun(t *testing.T) {
	for _, c := range []struct {
		n    int
		want string
	}{{0, "0 sessions"}, {1, "1 session"}, {2, "2 sessions"}} {
		if got := countNoun(c.n, "session"); got != c.want {
			t.Errorf("countNoun(%d) = %q, want %q", c.n, got, c.want)
		}
	}
}
