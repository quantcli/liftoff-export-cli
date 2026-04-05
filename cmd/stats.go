package cmd

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/dturner/liftoff-cli/internal/client"
	"github.com/spf13/cobra"
)

var (
	statsSinceFlag    string
	statsExerciseFlag string
	statsJSONFlag     bool
)

// WorkoutStats holds computed stats for a single workout.
type WorkoutStats struct {
	Date       string          `json:"date"`
	Bodyweight float64         `json:"bodyweight"`
	DurationMin float64        `json:"durationMinutes"`
	PRCount    int             `json:"prCount"`
	Exercises  []ExerciseStats `json:"exercises"`
}

// ExerciseStats holds computed stats for a single exercise within a workout.
type ExerciseStats struct {
	Name       string  `json:"name"`
	Type       string  `json:"type"`
	Sets       int     `json:"sets"`
	Reps       int     `json:"reps"`
	BestWeight float64 `json:"bestWeight,omitempty"`
	BestReps   int     `json:"bestReps,omitempty"`
	Volume     float64 `json:"volume,omitempty"`
}

var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show per-workout statistics",
	RunE: func(cmd *cobra.Command, args []string) error {
		c := client.New()
		var posts []Post
		if err := c.Query("post.getMyPosts", nil, &posts); err != nil {
			return err
		}
		if statsSinceFlag != "" {
			since, err := parseSince(statsSinceFlag)
			if err != nil {
				return err
			}
			var filtered []Post
			for _, p := range posts {
				t, err := time.Parse(time.RFC3339Nano, p.StartedAt)
				if err != nil || !t.Before(since) {
					filtered = append(filtered, p)
				}
			}
			posts = filtered
		}
		if statsExerciseFlag != "" {
			posts = filterExercises(posts, statsExerciseFlag)
		}
		if len(posts) == 0 {
			fmt.Println("No workouts found.")
			return nil
		}

		stats := computeStats(posts)
		if statsJSONFlag {
			return printJSON(stats)
		}
		printStatsText(stats)
		return nil
	},
}

func init() {
	workoutsCmd.AddCommand(statsCmd)
	statsCmd.Flags().StringVar(&statsSinceFlag, "since", "", "Filter workouts on or after date (e.g. 2025-01-01, 30d, 4w, 6m, 1y)")
	statsCmd.Flags().StringVar(&statsExerciseFlag, "exercise", "", "Filter to exercises matching this name (word-prefix match)")
	statsCmd.Flags().BoolVar(&statsJSONFlag, "json", false, "Output as JSON")
}

func computeStats(posts []Post) []WorkoutStats {
	stats := make([]WorkoutStats, 0, len(posts))
	for _, p := range posts {
		bw, _ := strconv.ParseFloat(strings.TrimSpace(p.Bodyweight), 64)
		t, _ := time.Parse(time.RFC3339Nano, p.StartedAt)

		ws := WorkoutStats{
			Date:        t.Format("2006-01-02"),
			Bodyweight:  bw,
			DurationMin: parseDurationMin(p.SessionDuration),
			PRCount:     p.PRCount,
		}

		for _, e := range p.ExerciseData {
			es := exerciseStats(e, bw)
			ws.Exercises = append(ws.Exercises, es)
		}

		stats = append(stats, ws)
	}
	return stats
}

func exerciseStats(e ExerciseData, bw float64) ExerciseStats {
	es := ExerciseStats{
		Name: e.ExerciseName,
		Type: e.ExerciseTypes,
	}

	for _, s := range e.SetsData {
		if s.SetType == "warmup" {
			continue
		}
		es.Sets++

		switch e.ExerciseTypes {
		case "WR":
			weight, _ := s.InputOne.Float64()
			reps, _ := s.InputTwo.Int64()
			es.Reps += int(reps)
			es.Volume += weight * float64(reps)
			if weight > es.BestWeight || (weight == es.BestWeight && int(reps) > es.BestReps) {
				es.BestWeight = weight
				es.BestReps = int(reps)
			}
		case "AB":
			assist, _ := s.InputOne.Float64()
			reps, _ := s.InputTwo.Int64()
			eff := bw - assist
			es.Reps += int(reps)
			es.Volume += eff * float64(reps)
			if eff > es.BestWeight || (eff == es.BestWeight && int(reps) > es.BestReps) {
				es.BestWeight = eff
				es.BestReps = int(reps)
			}
		case "BR":
			added, _ := s.InputOne.Float64()
			reps, _ := s.InputTwo.Int64()
			eff := bw + added
			es.Reps += int(reps)
			es.Volume += eff * float64(reps)
			if eff > es.BestWeight || (eff == es.BestWeight && int(reps) > es.BestReps) {
				es.BestWeight = eff
				es.BestReps = int(reps)
			}
		case "DD":
			// distance/duration — no weight volume
			reps, _ := s.InputTwo.Int64()
			es.Reps += int(reps)
		case "ND":
			es.Sets++ // count the set, nothing else to track
		}
	}

	return es
}

func parseDurationMin(s string) float64 {
	parts := strings.Fields(s)
	if len(parts) < 6 {
		return 0
	}
	h, _ := strconv.ParseFloat(parts[0], 64)
	m, _ := strconv.ParseFloat(parts[2], 64)
	sec, _ := strconv.ParseFloat(parts[4], 64)
	return h*60 + m + sec/60
}

func printStatsText(stats []WorkoutStats) {
	for i, ws := range stats {
		totalSets, totalReps := 0, 0
		totalVolume := 0.0
		for _, e := range ws.Exercises {
			totalSets += e.Sets
			totalReps += e.Reps
			totalVolume += e.Volume
		}

		fmt.Printf("%s  BW=%.0f  %0.fm  %d exercises  %d sets  %d reps",
			ws.Date, ws.Bodyweight, ws.DurationMin,
			len(ws.Exercises), totalSets, totalReps)
		if totalVolume > 0 {
			fmt.Printf("  %.0f lb vol", totalVolume)
		}
		if ws.PRCount > 0 {
			fmt.Printf("  %d PRs", ws.PRCount)
		}
		fmt.Println()

		for _, e := range ws.Exercises {
			fmt.Printf("  %-30s  %d sets  %d reps", e.Name, e.Sets, e.Reps)
			if e.BestWeight > 0 {
				fmt.Printf("  best=%.0fx%d", e.BestWeight, e.BestReps)
			}
			if e.Volume > 0 {
				fmt.Printf("  vol=%.0f", e.Volume)
			}
			fmt.Println()
		}

		if i < len(stats)-1 {
			fmt.Println()
		}
	}
}
