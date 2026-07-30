package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// version is overwritten at release time via
// -ldflags "-X github.com/quantcli/liftoff-export-cli/cmd.version=v1.2.0".
// Setting rootCmd.Version below makes cobra register --version for free.
var version = "dev"

var rootCmd = &cobra.Command{
	Use:     "liftoff-export",
	Short:   "CLI for the Liftoff fitness app",
	Version: version,
	Long: `liftoff-export reads your personal Liftoff (gymbros.com) workout and
bodyweight data and prints it on stdout. Default output is narrow,
fitdown-style markdown; pass --format json for the full structured row.

LLM agents: run 'liftoff-export prime' for a one-screen orientation
(I/O contract, subcommands, date flags, jq recipes).`,
	// Keep the output contract clean: stdout is for data only. By default
	// cobra dumps the full usage/flags block to stderr on any RunE or
	// flag-parse error, which clutters logs and buries the actual message.
	// Silence both and print a single-line error ourselves in Execute. (#31)
	SilenceUsage:  true,
	SilenceErrors: true,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(authCmd)
	rootCmd.AddCommand(bodyweightsCmd)
	rootCmd.AddCommand(routinesCmd)
	rootCmd.AddCommand(workoutsCmd)
}
