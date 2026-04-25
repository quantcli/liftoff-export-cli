package cmd

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/quantcli/liftoff-export-cli/internal/client"
	"github.com/spf13/cobra"
)

var (
	statsSinceFlag    string
	statsUntilFlag    string
	statsExerciseFlag string
	statsFormatFlag   string
	statsDetailFlag   bool
)

// SessionStats holds stats for one exercise in one workout.
type SessionStats struct {
	Date       string  `json:"date"`
	Bodyweight float64 `json:"bodyweight"`
	Sets       int     `json:"sets"`
	Reps       int     `json:"reps"`
	BestWeight float64 `json:"bestWeight,omitempty"`
	BestReps   int     `json:"bestReps,omitempty"`
	Volume     float64 `json:"volume,omitempty"`
	Duration   int     `json:"duration,omitempty"`
	Distance   float64 `json:"distance,omitempty"`
}

// ExerciseSummary holds all sessions for one exercise.
type ExerciseSummary struct {
	Name     string         `json:"name"`
	Type     string         `json:"type"`
	Sessions []SessionStats `json:"sessions"`
}

var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show per-exercise statistics across workouts",
	RunE: func(cmd *cobra.Command, args []string) error {
		format, err := validateFormat(statsFormatFlag)
		if err != nil {
			return err
		}
		c := client.New()
		var posts []Post
		if err := c.Query("post.getMyPosts", nil, &posts); err != nil {
			return err
		}
		since, err := parseDateValue(statsSinceFlag)
		if err != nil {
			return err
		}
		until, err := parseUntilValue(statsUntilFlag)
		if err != nil {
			return err
		}
		posts = filterByWindow(posts, since, until)
		if statsExerciseFlag != "" {
			posts = filterExercises(posts, statsExerciseFlag)
		}
		if len(posts) == 0 {
			if format == "json" {
				return printJSON([]ExerciseSummary{})
			}
			fmt.Println("No workouts found.")
			return nil
		}

		// Sort posts oldest-first
		sort.Slice(posts, func(i, j int) bool {
			return posts[i].StartedAt < posts[j].StartedAt
		})

		summaries := buildSummaries(posts)
		if format == "json" {
			return printJSON(summaries)
		}
		if statsDetailFlag {
			printSummariesTextDetail(summaries)
		} else {
			printSummariesText(summaries)
		}
		return nil
	},
}

func init() {
	workoutsCmd.AddCommand(statsCmd)
	statsCmd.Flags().StringVar(&statsSinceFlag, "since", "", "Filter workouts on or after date (today, yesterday, YYYY-MM-DD, or Nd/Nw/Nm/Ny)")
	statsCmd.Flags().StringVar(&statsUntilFlag, "until", "", "Filter workouts through date, inclusive (today, yesterday, YYYY-MM-DD, or Nd/Nw/Nm/Ny)")
	statsCmd.Flags().StringVar(&statsExerciseFlag, "exercise", "", "Filter to exercises matching this name (word-prefix match)")
	statsCmd.Flags().StringVar(&statsFormatFlag, "format", "markdown", "Output format: markdown (default) or json")
	statsCmd.Flags().BoolVar(&statsDetailFlag, "detail", false, "Show per-session breakdown")
}

func buildSummaries(posts []Post) []ExerciseSummary {
	// Preserve first-seen order
	orderMap := map[string]int{}
	dataMap := map[string]*ExerciseSummary{}

	for _, p := range posts {
		bw, _ := strconv.ParseFloat(strings.TrimSpace(p.Bodyweight), 64)
		t, _ := time.Parse(time.RFC3339Nano, p.StartedAt)
		date := t.Local().Format("2006-01-02")

		for _, e := range p.ExerciseData {
			ss := sessionStats(e, bw)
			ss.Date = date
			ss.Bodyweight = bw

			key := e.ExerciseName
			if _, exists := dataMap[key]; !exists {
				dataMap[key] = &ExerciseSummary{
					Name: e.ExerciseName,
					Type: e.ExerciseTypes,
				}
				orderMap[key] = len(orderMap)
			}
			dataMap[key].Sessions = append(dataMap[key].Sessions, ss)
		}
	}

	// Sort by first-seen order
	result := make([]ExerciseSummary, len(dataMap))
	for key, summary := range dataMap {
		result[orderMap[key]] = *summary
	}
	return result
}

func sessionStats(e ExerciseData, bw float64) SessionStats {
	ss := SessionStats{}

	for _, s := range e.SetsData {
		if s.SetType == "warmup" {
			continue
		}
		ss.Sets++

		switch e.ExerciseTypes {
		case "WR":
			weight, _ := s.InputOne.Float64()
			reps, _ := s.InputTwo.Int64()
			ss.Reps += int(reps)
			ss.Volume += weight * float64(reps)
			if weight > ss.BestWeight || (weight == ss.BestWeight && int(reps) > ss.BestReps) {
				ss.BestWeight = weight
				ss.BestReps = int(reps)
			}
		case "AB":
			assist, _ := s.InputOne.Float64()
			reps, _ := s.InputTwo.Int64()
			eff := bw - assist
			ss.Reps += int(reps)
			ss.Volume += eff * float64(reps)
			if eff > ss.BestWeight || (eff == ss.BestWeight && int(reps) > ss.BestReps) {
				ss.BestWeight = eff
				ss.BestReps = int(reps)
			}
		case "BR":
			added, _ := s.InputOne.Float64()
			reps, _ := s.InputTwo.Int64()
			eff := bw + added
			ss.Reps += int(reps)
			ss.Volume += eff * float64(reps)
			if eff > ss.BestWeight || (eff == ss.BestWeight && int(reps) > ss.BestReps) {
				ss.BestWeight = eff
				ss.BestReps = int(reps)
			}
		case "DD":
			km, _ := s.InputOne.Float64()
			secs, _ := s.InputTwo.Int64()
			ss.Reps += int(secs)
			ss.Duration += int(secs)
			ss.Distance += km
		}
	}

	return ss
}

func printSummariesText(summaries []ExerciseSummary) {
	for i, ex := range summaries {
		fmt.Printf("%s — %d sessions\n", ex.Name, len(ex.Sessions))

		switch ex.Type {
		case "WR", "AB", "BR":
			printWeightSummary(ex.Sessions)
		case "DD":
			printDurationSummary(ex.Sessions)
		}

		months := monthlyExerciseStats(ex.Sessions, ex.Type)
		if len(months) > 0 && ex.Type != "ND" {
			fmt.Println()
			printExerciseBarGraph(months, ex.Type)
		}

		if i < len(summaries)-1 {
			fmt.Println()
		}
	}
}

func printWeightSummary(sessions []SessionStats) {
	var prIdx int
	for i, ss := range sessions {
		if ss.BestWeight > sessions[prIdx].BestWeight ||
			(ss.BestWeight == sessions[prIdx].BestWeight && ss.BestReps > sessions[prIdx].BestReps) {
			prIdx = i
		}
	}
	pr := sessions[prIdx]
	recent := sessions[len(sessions)-1]

	prDate, _ := time.Parse("2006-01-02", pr.Date)
	fmt.Printf("  PR:     %.0fx%d (%s)\n", pr.BestWeight, pr.BestReps, prDate.Format("Jan 2006"))
	fmt.Printf("  Recent: %.0fx%d\n", recent.BestWeight, recent.BestReps)
}

func printDurationSummary(sessions []SessionStats) {
	var bestIdx int
	for i, ss := range sessions {
		if ss.Duration > sessions[bestIdx].Duration {
			bestIdx = i
		}
	}
	best := sessions[bestIdx]
	recent := sessions[len(sessions)-1]

	bestDate, _ := time.Parse("2006-01-02", best.Date)
	fmt.Printf("  Best:   %s (%s)\n", formatDuration(best.Duration), bestDate.Format("Jan 2006"))
	fmt.Printf("  Recent: %s\n", formatDuration(recent.Duration))
}

type monthStat struct {
	month string
	value float64
}

func monthlyExerciseStats(sessions []SessionStats, exType string) []monthStat {
	monthMap := map[string][]SessionStats{}
	for _, ss := range sessions {
		key := ss.Date[:7]
		monthMap[key] = append(monthMap[key], ss)
	}

	var months []monthStat
	for key, mSessions := range monthMap {
		var val float64
		switch exType {
		case "WR", "AB", "BR":
			for _, ss := range mSessions {
				if ss.BestWeight > val {
					val = ss.BestWeight
				}
			}
		case "DD":
			for _, ss := range mSessions {
				val += float64(ss.Duration)
			}
		}
		months = append(months, monthStat{month: key, value: val})
	}

	sort.Slice(months, func(i, j int) bool {
		return months[i].month < months[j].month
	})
	return months
}

func printExerciseBarGraph(months []monthStat, exType string) {
	minVal := months[0].value
	maxVal := months[0].value
	for _, m := range months {
		if m.value < minVal {
			minVal = m.value
		}
		if m.value > maxVal {
			maxVal = m.value
		}
	}
	chartMin := minVal * 0.9

	// Determine label width for alignment
	maxLabelLen := 0
	labels := make([]string, len(months))
	for i, m := range months {
		switch exType {
		case "WR", "AB", "BR":
			labels[i] = formatWeight(m.value)
		case "DD":
			labels[i] = formatDuration(int(m.value))
		default:
			labels[i] = fmt.Sprintf("%.0f", m.value)
		}
		if len(labels[i]) > maxLabelLen {
			maxLabelLen = len(labels[i])
		}
	}

	fmtStr := fmt.Sprintf("  %%s  %%%ds %%s\n", maxLabelLen)
	for i, m := range months {
		barLen := scaledBarLength(m.value, chartMin, maxVal, 40)
		fmt.Printf(fmtStr, m.month, labels[i], strings.Repeat("█", barLen))
	}
}

func formatDuration(totalSecs int) string {
	if totalSecs >= 3600 {
		h := totalSecs / 3600
		m := (totalSecs % 3600) / 60
		if m == 0 {
			return fmt.Sprintf("%dh", h)
		}
		return fmt.Sprintf("%dh %dm", h, m)
	}
	m := totalSecs / 60
	s := totalSecs % 60
	if s == 0 {
		return fmt.Sprintf("%dm", m)
	}
	return fmt.Sprintf("%d:%02d", m, s)
}

func printSummariesTextDetail(summaries []ExerciseSummary) {
	for i, ex := range summaries {
		fmt.Printf("%s — %d sessions\n", ex.Name, len(ex.Sessions))
		for _, ss := range ex.Sessions {
			fmt.Printf("  %s  BW=%.0f  %d sets  %d reps", ss.Date, ss.Bodyweight, ss.Sets, ss.Reps)
			if ss.BestWeight > 0 {
				fmt.Printf("  %.0fx%d", ss.BestWeight, ss.BestReps)
			}
			if ss.Volume > 0 {
				fmt.Printf("  vol=%.0f", ss.Volume)
			}
			fmt.Println()
		}
		if i < len(summaries)-1 {
			fmt.Println()
		}
	}
}
