package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/quantcli/liftoff-export-cli/internal/auth"
	"github.com/spf13/cobra"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Authentication commands",
}

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Log in to Liftoff",
	RunE: func(cmd *cobra.Command, args []string) error {
		scanner := bufio.NewScanner(os.Stdin)

		fmt.Print("Email: ")
		scanner.Scan()
		email := strings.TrimSpace(scanner.Text())

		fmt.Print("Password: ")
		scanner.Scan()
		password := strings.TrimSpace(scanner.Text())

		if email == "" || password == "" {
			return fmt.Errorf("email and password are required")
		}

		fmt.Println("Logging in...")
		if err := auth.Login(email, password); err != nil {
			return err
		}
		fmt.Println("Logged in. Tokens saved to ~/.config/liftoff-export/auth.json")
		return nil
	},
}

var refreshCmd = &cobra.Command{
	Use:   "refresh",
	Short: "Manually refresh the access token",
	RunE: func(cmd *cobra.Command, args []string) error {
		token, err := auth.GetToken()
		if err != nil {
			return err
		}
		fmt.Printf("Token valid: %s...\n", token[:20])
		return nil
	},
}

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Remove stored auth tokens",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := auth.Logout(); err != nil {
			return err
		}
		fmt.Println("Logged out.")
		return nil
	},
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Print one-line auth readiness state and exit 0 if usable",
	Long: `Print a one-line summary of whether the CLI has a usable token. Exit 0
if a saved token is present and not yet expired, 1 otherwise.

This is a local check — no network call and no refresh is attempted, even
when the saved token is expired. Use 'auth refresh' (or any export
subcommand) to actually refresh.

If LIFTOFF_REFRESH_TOKEN is set it wins over any saved token, and status
reports that instead. Its validity is unknown without a network call, so
exit 0 there means "a token was supplied", not "the token works".

Per the quantcli shared contract:
https://github.com/quantcli/common/blob/main/CONTRACT.md#5-auth`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Headless mode has no saved token to inspect and no expiry to
		// report until a refresh happens, so report the source and stop.
		if auth.EnvRefreshToken() != "" {
			fmt.Fprintln(cmd.OutOrStdout(), "using LIFTOFF_REFRESH_TOKEN (headless; no saved token consulted)")
			return nil
		}

		store, err := auth.Load()
		if err != nil {
			return fmt.Errorf("not logged in — run: liftoff-export auth login (or set LIFTOFF_REFRESH_TOKEN)")
		}
		exp := store.ExpiresAt.Local().Format(time.RFC3339)
		if time.Now().After(store.ExpiresAt) {
			return fmt.Errorf("token expired %s — run: liftoff-export auth refresh", exp)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "logged in (token expires %s)\n", exp)
		return nil
	},
}

func init() {
	authCmd.AddCommand(loginCmd)
	authCmd.AddCommand(logoutCmd)
	authCmd.AddCommand(refreshCmd)
	authCmd.AddCommand(statusCmd)
}
