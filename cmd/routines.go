package cmd

import (
	"encoding/json"
	"fmt"
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

// presetsResponse is the top-level shape of fitnessService.fetchUserPresetsWithFolders.
// We currently surface only presetsWithoutFolder; folders trigger a stderr
// warning so a non-empty value isn't silently dropped. Folder support is a
// follow-up once we have a non-empty example to model.
type presetsResponse struct {
	Folders                json.RawMessage `json:"folders"`
	PresetsWithoutFolder   []Preset        `json:"presetsWithoutFolder"`
}

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
		presets, err := fetchPresets()
		if err != nil {
			return err
		}
		if format == "json" {
			return printJSON(presets)
		}
		return printRoutinesFitdown(presets)
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
		presets, err := fetchPresets()
		if err != nil {
			return err
		}
		match, err := pickPreset(presets, args[0])
		if err != nil {
			return err
		}
		if format == "json" {
			return printJSON([]Preset{*match})
		}
		return printRoutinesFitdown([]Preset{*match})
	},
}

func fetchPresets() ([]Preset, error) {
	c := client.New()
	var resp presetsResponse
	if err := c.Query("fitnessService.fetchUserPresetsWithFolders", nil, &resp); err != nil {
		return nil, err
	}
	if len(resp.Folders) > 0 && string(resp.Folders) != "[]" && string(resp.Folders) != "null" {
		fmt.Fprintln(os.Stderr, "liftoff-export: warning — routine folders detected but not yet rendered; presets outside folders will be shown. File a follow-up if you need folder support.")
	}
	return resp.PresetsWithoutFolder, nil
}

// pickPreset resolves a `show` argument to exactly one preset. Match order:
// (1) case-insensitive exact name match, (2) exact id match. Multiple
// name-matches return an error so the caller can disambiguate by id.
func pickPreset(presets []Preset, arg string) (*Preset, error) {
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

// printRoutinesFitdown renders presets in the same fitdown notation
// `workouts list` uses. Each routine gets a header; favorite routines are
// marked with a star; exerciseNotes per-exercise are surfaced below the
// exercise name so cues like "Left Only" aren't lost.
func printRoutinesFitdown(presets []Preset) error {
	for i, p := range presets {
		if i > 0 {
			fmt.Println()
			fmt.Println("---")
			fmt.Println()
		}
		star := ""
		if p.IsFavorite {
			star = " ★"
		}
		fmt.Printf("# Routine: %s%s\n", p.Name, star)
		for _, ex := range p.ExerciseData {
			fmt.Println()
			fmt.Println(ex.ExerciseName)
			if ex.ExerciseNotes != nil && *ex.ExerciseNotes != "" {
				fmt.Printf("# %s\n", *ex.ExerciseNotes)
			}
			var lines []string
			for _, s := range ex.SetsData {
				lines = append(lines, fitdownSetLine(ex.ExerciseTypes, s))
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
