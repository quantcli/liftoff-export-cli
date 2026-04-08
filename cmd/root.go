package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "liftoff-export",
	Short: "CLI for the Liftoff fitness app",
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(authCmd)
	rootCmd.AddCommand(bodyweightsCmd)
	rootCmd.AddCommand(workoutsCmd)
}
