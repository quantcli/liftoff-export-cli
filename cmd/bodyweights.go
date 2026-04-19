package cmd

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/quantcli/liftoff-export-cli/internal/client"
	"github.com/spf13/cobra"
)

type bodyweightEntry struct {
	date   time.Time
	weight float64
}

type monthAvg struct {
	month string
	avg   float64
}

var (
	bodyweightsListSinceFlag  string
	bodyweightsStatsSinceFlag string
)

var bodyweightsCmd = &cobra.Command{
	Use:   "bodyweights",
	Short: "Bodyweight commands",
}

var bodyweightsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List recorded bodyweights",
	RunE: func(cmd *cobra.Command, args []string) error {
		entries, err := loadBodyweightEntries(bodyweightsListSinceFlag)
		if err != nil {
			return err
		}
		if len(entries) == 0 {
			fmt.Println("No bodyweights found.")
			return nil
		}

		for _, entry := range entries {
			fmt.Printf("%s  %s lbs\n", entry.date.Format("2006-01-02"), formatWeight(entry.weight))
		}
		return nil
	},
}

var bodyweightsStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show bodyweight statistics",
	RunE: func(cmd *cobra.Command, args []string) error {
		entries, err := loadBodyweightEntries(bodyweightsStatsSinceFlag)
		if err != nil {
			return err
		}
		if len(entries) == 0 {
			fmt.Println("No bodyweights found.")
			return nil
		}

		printBodyweightStats(entries)
		return nil
	},
}

func init() {
	bodyweightsCmd.AddCommand(bodyweightsListCmd)
	bodyweightsCmd.AddCommand(bodyweightsStatsCmd)

	bodyweightsListCmd.Flags().StringVar(&bodyweightsListSinceFlag, "since", "", "Filter entries on or after date (e.g. 2025-01-01, 30d, 4w, 6m, 1y)")
	bodyweightsStatsCmd.Flags().StringVar(&bodyweightsStatsSinceFlag, "since", "", "Filter entries on or after date (e.g. 2025-01-01, 30d, 4w, 6m, 1y)")
}

func loadBodyweightEntries(sinceFlag string) ([]bodyweightEntry, error) {
	c := client.New()
	var posts []Post
	if err := c.Query("post.getMyPosts", nil, &posts); err != nil {
		return nil, err
	}

	var since time.Time
	if sinceFlag != "" {
		parsed, err := parseSince(sinceFlag)
		if err != nil {
			return nil, err
		}
		since = parsed
	}

	entries := make([]bodyweightEntry, 0, len(posts))
	for _, post := range posts {
		weight, err := strconv.ParseFloat(strings.TrimSpace(post.Bodyweight), 64)
		if err != nil || weight == 0 {
			continue
		}

		date, err := time.Parse(time.RFC3339Nano, post.StartedAt)
		if err != nil {
			continue
		}
		if !since.IsZero() && date.Before(since) {
			continue
		}

		entries = append(entries, bodyweightEntry{
			date:   date,
			weight: weight,
		})
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].date.Before(entries[j].date)
	})

	return entries, nil
}

func printBodyweightStats(entries []bodyweightEntry) {
	first := entries[0]
	last := entries[len(entries)-1]
	minWeight := entries[0].weight
	maxWeight := entries[0].weight

	for _, entry := range entries {
		if entry.weight < minWeight {
			minWeight = entry.weight
		}
		if entry.weight > maxWeight {
			maxWeight = entry.weight
		}
	}

	fmt.Println("Bodyweight")
	fmt.Printf("  Current: %s lbs | High: %s | Low: %s\n", formatWeight(last.weight), formatWeight(maxWeight), formatWeight(minWeight))

	change := last.weight - first.weight
	fmt.Printf("  Change:  %+.1f lbs (%s → %s)\n", change, first.date.Format("Jan 2006"), last.date.Format("Jan 2006"))

	months := last.date.Sub(first.date).Hours() / 24 / 30.44
	if months > 0.5 {
		rate := change / months
		fmt.Printf("  Rate:    %+.1f lbs/month\n", rate)
	}

	monthAvgs := monthlyBodyweightAverages(entries)
	if len(monthAvgs) >= 3 {
		tail := monthAvgs
		if len(tail) > 6 {
			tail = tail[len(tail)-6:]
		}

		mean := 0.0
		for _, month := range tail {
			mean += month.avg
		}
		mean /= float64(len(tail))

		variance := 0.0
		for _, month := range tail {
			variance += (month.avg - mean) * (month.avg - mean)
		}
		stddev := math.Sqrt(variance / float64(len(tail)-1))
		if stddev < 2.0 {
			fmt.Printf("  Plateau: %s - present (%s lbs avg, %.1f stddev)\n", monthNameFromKey(tail[0].month), formatWeight(mean), stddev)
		}
	}

	fmt.Println()
	chartMin := minWeight - 10
	for _, month := range monthAvgs {
		barLen := scaledBarLength(month.avg, chartMin, maxWeight, 40)
		fmt.Printf("  %s  %5s %s\n", month.month, formatWeight(month.avg), strings.Repeat("█", barLen))
	}
}

func monthlyBodyweightAverages(entries []bodyweightEntry) []monthAvg {
	monthMap := map[string][]float64{}
	for _, entry := range entries {
		key := entry.date.Format("2006-01")
		monthMap[key] = append(monthMap[key], entry.weight)
	}

	months := make([]monthAvg, 0, len(monthMap))
	for key, weights := range monthMap {
		sum := 0.0
		for _, weight := range weights {
			sum += weight
		}
		months = append(months, monthAvg{
			month: key,
			avg:   sum / float64(len(weights)),
		})
	}

	sort.Slice(months, func(i, j int) bool {
		return months[i].month < months[j].month
	})

	return months
}

func monthNameFromKey(key string) string {
	date, err := time.Parse("2006-01", key)
	if err != nil {
		return key
	}
	return date.Format("Jan 2006")
}

func formatWeight(weight float64) string {
	if weight == math.Trunc(weight) {
		return fmt.Sprintf("%.0f", weight)
	}
	return fmt.Sprintf("%.1f", weight)
}

func scaledBarLength(value, minValue, maxValue, width float64) int {
	if width <= 0 {
		return 0
	}
	if maxValue <= minValue {
		return int(width)
	}

	ratio := (value - minValue) / (maxValue - minValue)
	if ratio < 0 {
		ratio = 0
	}
	if ratio > 1 {
		ratio = 1
	}
	return int(math.Round(ratio * width))
}
