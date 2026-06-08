package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "liftoff-export",
	Short: "CLI for the Liftoff fitness app",
	Long: `liftoff-export reads your personal Liftoff (gymbros.com) workout and
bodyweight data and prints it on stdout. Default output is narrow,
fitdown-style markdown; pass --format json for the full structured row.

LLM agents: run 'liftoff-export prime' for a one-screen orientation
(I/O contract, subcommands, date flags, jq recipes).`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(authCmd)
	rootCmd.AddCommand(bodyweightsCmd)
	rootCmd.AddCommand(routinesCmd)
	rootCmd.AddCommand(workoutsCmd)
}
