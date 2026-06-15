package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/quantcli/liftoff-export-cli/internal/client"
	"github.com/spf13/cobra"
)

// Preset mirrors a Liftoff fitnessService preset (what the app calls a saved
// workout template; users call them "routines"). The field names match the
// upstream JSON so `--format json | jq` reads naturally against API docs.
type Preset struct {
	ID             string                `json:"id"`
	CreatedAt      string                `json:"createdAt"`
	UserID         string                `json:"userId"`
	Name           string                `json:"name"`
	Image          *string               `json:"image"`
	MarketPresetID *string               `json:"marketPresetId"`
	BookmarkID     string                `json:"bookmarkId"`
	Completed      int                   `json:"completed"`
	AvgDuration    int                   `json:"avgDuration"`
	IsFavorite     bool                  `json:"isFavorite"`
	FolderID       *string               `json:"folderId"`
	ExerciseData   []PresetExerciseData  `json:"exerciseData"`
}

// PresetExerciseData mirrors an entry in a preset's exerciseData array.
// Shape parallels workouts.ExerciseData, with the additional preset linkage
// fields (presetCuid, exerciseDataId on sets).
type PresetExerciseData struct {
	ID                 string           `json:"id"`
	PresetCUID         string           `json:"presetCuid"`
	ExerciseIndex      int              `json:"exerciseIndex"`
	ExerciseName       string           `json:"exerciseName"`
	ExerciseID         string           `json:"exerciseId"`
	ExerciseTypes      string           `json:"exerciseTypes"`
	Superset           *string          `json:"superset"`
	OverrideWeightUnit *string          `json:"overrideWeightUnit"`
	ExerciseCUID       *string          `json:"exerciseCuid"`
	MarketExerciseCUID *string          `json:"marketExerciseCuid"`
	ExerciseNotes      *string          `json:"exerciseNotes"`
	SetsData           []PresetSetData  `json:"setsData"`
}

type PresetSetData struct {
	ID             string      `json:"id"`
	ExerciseDataID string      `json:"exerciseDataId"`
	SetIndex       int         `json:"setIndex"`
	SetType        string      `json:"setType"`
	InputOne       json.Number `json:"inputOne"`
	InputTwo       json.Number `json:"inputTwo"`
}

// Folder is a Liftoff routine folder — a labeled group of presets in the
// app's Routines tab. The nested Presets array contains the full Preset
// objects (each with folderId pointing back to this folder).
type Folder struct {
	ID           string   `json:"id"`
	CreatedAt    string   `json:"createdAt"`
	UserID       string   `json:"userId"`
	Name         string   `json:"name"`
	PresetsOrder []string `json:"presetsOrder"`
	Presets      []Preset `json:"presets"`
}

// presetsResponse is the top-level shape of fitnessService.fetchUserPresetsWithFolders.
// PresetsWithoutFolder are what the Liftoff app surfaces under "My Routines"
// (the implicit ungrouped section); folder Presets are surfaced under their
// folder name. Both rendering paths share the same Preset shape.
type presetsResponse struct {
	Folders              []Folder `json:"folders"`
	PresetsWithoutFolder []Preset `json:"presetsWithoutFolder"`
}

// unfiledLabel is the section title we use for PresetsWithoutFolder in
// markdown output. Matches the Liftoff app's UI label so a user comparing
// terminal output to the app sees the same heading.
const unfiledLabel = "My Routines"

var routinesCmd = &cobra.Command{
	Use:   "routines",
	Short: "Saved workout routine (preset) commands",
	Long: `Routines are reusable workout templates saved in the Liftoff app.
The upstream API calls them "presets"; the JSON output preserves that
naming. The fitdown markdown renderer treats a routine the same way it
treats a logged workout.`,
}

var routinesListFormatFlag string

var routinesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all your saved routines",
	RunE: func(cmd *cobra.Command, args []string) error {
		format, err := validateFormat(routinesListFormatFlag)
		if err != nil {
			return err
		}
		resp, err := fetchPresets()
		if err != nil {
			return err
		}
		if format == "json" {
			return printJSON(resp)
		}
		return renderRoutinesFitdown(os.Stdout, resp)
	},
}

var routinesShowFormatFlag string

var routinesShowCmd = &cobra.Command{
	Use:   "show <name-or-id>",
	Short: "Show one routine by name (case-insensitive exact match) or by id",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		format, err := validateFormat(routinesShowFormatFlag)
		if err != nil {
			return err
		}
		resp, err := fetchPresets()
		if err != nil {
			return err
		}
		match, err := pickPreset(resp, args[0])
		if err != nil {
			return err
		}
		if format == "json" {
			return printJSON([]Preset{*match})
		}
		// `show` emits a single routine without any section heading —
		// the routine name is enough context for a one-off lookup.
		return renderOnePreset(os.Stdout, *match, "#")
	},
}

func fetchPresets() (*presetsResponse, error) {
	c := client.New()
	var resp presetsResponse
	if err := c.Query("fitnessService.fetchUserPresetsWithFolders", nil, &resp); err != nil {
		return nil, err
	}
	// Normalize nil → empty slice so --format json emits `[]` not `null`,
	// matching what the upstream API does on an empty account.
	if resp.Folders == nil {
		resp.Folders = []Folder{}
	}
	if resp.PresetsWithoutFolder == nil {
		resp.PresetsWithoutFolder = []Preset{}
	}
	return &resp, nil
}

// allPresets flattens both unfiled and foldered presets into one slice for
// lookup. Used by pickPreset so `routines show "Valley Creek 1"` finds a
// routine regardless of whether it lives in a folder.
func allPresets(resp *presetsResponse) []Preset {
	out := make([]Preset, 0, len(resp.PresetsWithoutFolder))
	out = append(out, resp.PresetsWithoutFolder...)
	for _, f := range resp.Folders {
		out = append(out, f.Presets...)
	}
	return out
}

// pickPreset resolves a `show` argument to exactly one preset across the
// full account (unfiled + foldered). Match order: (1) case-insensitive
// exact name match, (2) exact id match. Multiple name-matches return an
// error so the caller can disambiguate by id.
func pickPreset(resp *presetsResponse, arg string) (*Preset, error) {
	presets := allPresets(resp)
	target := strings.ToLower(strings.TrimSpace(arg))
	var nameHits []Preset
	for _, p := range presets {
		if strings.ToLower(p.Name) == target {
			nameHits = append(nameHits, p)
		}
	}
	if len(nameHits) == 1 {
		return &nameHits[0], nil
	}
	if len(nameHits) > 1 {
		ids := make([]string, 0, len(nameHits))
		for _, p := range nameHits {
			ids = append(ids, p.ID)
		}
		return nil, fmt.Errorf("multiple routines named %q — disambiguate by id: %s", arg, strings.Join(ids, ", "))
	}
	for i := range presets {
		if presets[i].ID == arg {
			return &presets[i], nil
		}
	}
	return nil, fmt.Errorf("no routine matches %q", arg)
}

// renderRoutinesFitdown renders the full account: a "# My Routines" H1 with
// each unfiled preset as an H2 underneath, then "# <FolderName>" H1 per
// folder with its foldered presets as H2. "---" rules separate sibling
// routines and section boundaries. Favorite routines are marked with a
// star; per-exercise notes are appended in parens so cues like "Left Only"
// aren't rendered as a markdown heading by mistake.
func renderRoutinesFitdown(w io.Writer, resp *presetsResponse) error {
	first := true
	emitSection := func(heading string, presets []Preset) {
		if len(presets) == 0 {
			return
		}
		if !first {
			fmt.Fprintln(w)
			fmt.Fprintln(w, "---")
			fmt.Fprintln(w)
		}
		first = false
		fmt.Fprintf(w, "# %s\n", heading)
		for i, p := range presets {
			if i > 0 {
				fmt.Fprintln(w)
				fmt.Fprintln(w, "---")
			}
			fmt.Fprintln(w)
			renderOnePreset(w, p, "##")
		}
	}
	emitSection(unfiledLabel, resp.PresetsWithoutFolder)
	for _, f := range resp.Folders {
		emitSection(f.Name, f.Presets)
	}
	return nil
}

// renderOnePreset prints a single preset under the given heading marker
// (e.g. "##" inside a folder section, "#" for `routines show` where the
// routine has no enclosing section). Body format (exercises, set lines,
// note parens) is identical regardless of caller.
func renderOnePreset(w io.Writer, p Preset, headingMarker string) error {
	star := ""
	if p.IsFavorite {
		star = " ★"
	}
	fmt.Fprintf(w, "%s Routine: %s%s\n", headingMarker, p.Name, star)
	for _, ex := range p.ExerciseData {
		fmt.Fprintln(w)
		name := ex.ExerciseName
		if ex.ExerciseNotes != nil && *ex.ExerciseNotes != "" {
			name = fmt.Sprintf("%s (%s)", name, *ex.ExerciseNotes)
		}
		fmt.Fprintln(w, name)
		var lines []string
		for _, s := range ex.SetsData {
			lines = append(lines, fitdownSetLine(ex.ExerciseTypes, s))
		}
		// Compress consecutive identical lines into Nx... notation.
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
	return nil
}

// fitdownSetLine is split out of workouts.printFitdown so routines can render
// the same notation without duplicating the type switch. Keeping it here
// rather than in workouts.go keeps the workouts file untouched, at the cost
// of a tiny bit of duplication of the WR/BR/AB/WD/DD/ND mapping.
func fitdownSetLine(exTypes string, s PresetSetData) string {
	switch exTypes {
	case "WR":
		return fmt.Sprintf("%s@%s", s.InputTwo, s.InputOne)
	case "AB":
		return fmt.Sprintf("%s@-%s", s.InputTwo, s.InputOne)
	case "BR":
		return fmt.Sprintf("%s@+%s", s.InputTwo, s.InputOne)
	case "WD":
		km, _ := s.InputTwo.Float64()
		return fmt.Sprintf("%slb %.3fmi", s.InputOne, km/1.60934)
	case "DD":
		secs, _ := s.InputTwo.Int64()
		km, _ := s.InputOne.Float64()
		return fmt.Sprintf("%.2fmi %d:%02d", km/1.60934, secs/60, secs%60)
	case "ND":
		secs, _ := s.InputTwo.Int64()
		return fmt.Sprintf("%d:%02d", secs/60, secs%60)
	default:
		return fmt.Sprintf("[%s] %s %s", exTypes, s.InputOne, s.InputTwo)
	}
}

func init() {
	routinesCmd.AddCommand(routinesListCmd)
	routinesCmd.AddCommand(routinesShowCmd)
	routinesListCmd.Flags().StringVar(&routinesListFormatFlag, "format", "markdown",
		"Output format: markdown (default, fitdown-style) or json")
	routinesShowCmd.Flags().StringVar(&routinesShowFormatFlag, "format", "markdown",
		"Output format: markdown (default, fitdown-style) or json")
}
