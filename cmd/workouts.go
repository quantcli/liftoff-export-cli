package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
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
type SetData struct {
	SetIndex int            `json:"setIndex"`
	SetType  string         `json:"setType"`
	InputOne json.Number    `json:"inputOne"` // weight (kg) or distance
	InputTwo json.Number    `json:"inputTwo"` // reps or duration (seconds)
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
		c := client.New()
		var posts []Post
		// post.getMyPosts returns all the user's own workouts (Memories view)
		if err := c.Query("post.getMyPosts", nil, &posts); err != nil {
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
		posts = filterByWindow(posts, since, until)
		if listExerciseFlag != "" {
			posts = filterExercises(posts, listExerciseFlag)
		}
		if format == "json" {
			return printJSON(posts)
		}
		return printFitdown(posts)
	},
}

// validateFormat checks --format against the quantcli shared output set
// (CONTRACT §4) and normalizes it to a canonical value. Liftoff currently
// supports markdown (the default; fitdown-style) and json. "md" is accepted
// as an alias.
func validateFormat(format string) (string, error) {
	switch format {
	case "", "markdown", "md":
		return "markdown", nil
	case "json":
		return "json", nil
	default:
		return "", fmt.Errorf("unknown --format %q (use markdown or json)", format)
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

		c := client.New()
		var posts []Post
		if err := c.Query("post.getMyPosts", nil, &posts); err != nil {
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
			fmt.Printf("No workouts found for %s.\n", target.Format("January 2, 2006"))
			return nil
		}

		if format == "json" {
			return printJSON(matched)
		}
		return printFitdown(matched)
	},
}

func init() {
	workoutsCmd.AddCommand(listCmd)
	workoutsCmd.AddCommand(showCmd)
	showCmd.Flags().StringVar(&showFormatFlag, "format", "markdown",
		"Output format: markdown (default, fitdown-style) or json")
	listCmd.Flags().StringVar(&listFormatFlag, "format", "markdown",
		"Output format: markdown (default, fitdown-style) or json")
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

func printFitdown(posts []Post) error {
	for i, post := range posts {
		t, err := time.Parse(time.RFC3339Nano, post.StartedAt)
		if err != nil {
			fmt.Printf("Workout %s\n", post.StartedAt)
		} else {
			fmt.Printf("Workout %s\n", t.Local().Format("January 2, 2006"))
		}

		if post.SessionNotes != "" {
			fmt.Printf("# %s\n", post.SessionNotes)
		}

		for _, ex := range post.ExerciseData {
			fmt.Println()
			fmt.Println(ex.ExerciseName)

			var lines []string
			for _, s := range ex.SetsData {
				var line string
				switch ex.ExerciseTypes {
				case "WR":
					line = fmt.Sprintf("%s@%s", s.InputTwo, s.InputOne)
				case "AB":
					line = fmt.Sprintf("%s@-%s", s.InputTwo, s.InputOne)
				case "BR":
					line = fmt.Sprintf("%s@+%s", s.InputTwo, s.InputOne)
				case "WD":
					km, _ := s.InputTwo.Float64()
					line = fmt.Sprintf("%slb %.3fmi", s.InputOne, km/1.60934)
				case "DD":
					secs, _ := s.InputTwo.Int64()
					km, _ := s.InputOne.Float64()
					line = fmt.Sprintf("%.2fmi %d:%02d", km/1.60934, secs/60, secs%60)
				case "ND":
					secs, _ := s.InputTwo.Int64()
					line = fmt.Sprintf("%d:%02d", secs/60, secs%60)
				default:
					line = fmt.Sprintf("[%s] %s %s", ex.ExerciseTypes, s.InputOne, s.InputTwo)
				}
				lines = append(lines, line)
			}

			// Compress consecutive identical lines into Nx... notation
			for i := 0; i < len(lines); {
				j := i + 1
				for j < len(lines) && lines[j] == lines[i] {
					j++
				}
				if n := j - i; n > 1 {
					fmt.Printf("%dx%s\n", n, lines[i])
				} else {
					fmt.Println(lines[i])
				}
				i = j
			}
		}

		if i < len(posts)-1 {
			fmt.Println()
		}
	}
	return nil
}

func printJSON(v any) error {
	// A nil slice marshals as "null"; force "[]" so empty windows match the
	// prime contract (and don't blow up jq pipelines).
	if rv := reflect.ValueOf(v); rv.Kind() == reflect.Slice && rv.IsNil() {
		_, err := os.Stdout.WriteString("[]\n")
		return err
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return fmt.Errorf("failed to encode output: %w", err)
	}
	return nil
}
