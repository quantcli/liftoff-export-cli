package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"time"
)

func init() {
	for _, cmd := range []*cobra.Command{listCmd, statsCmd, bodyweightsListCmd, bodyweightsStatsCmd} {
		cmd.Flags().Bool("print-resolved-window", false, "Print resolved dates without contacting Liftoff")
	}
}

func printResolvedWindow(cmd *cobra.Command, since, until time.Time) (bool, error) {
	enabled, err := cmd.Flags().GetBool("print-resolved-window")
	if err != nil || !enabled {
		return false, err
	}
	format := func(t time.Time) string {
		if t.IsZero() {
			return "unbounded"
		}
		return t.Format(time.RFC3339)
	}
	_, err = fmt.Fprintf(cmd.OutOrStdout(), "since=%s until_exclusive=%s\n", format(since), format(until))
	return true, err
}
