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

var (
	importRefreshTokenFlag string
	importAccessTokenFlag  string
	importExpiresAtFlag    string
	importNoVerifyFlag     bool
)

var importCmd = &cobra.Command{
	Use:   "import",
	Short: "Import a refresh token captured from the Liftoff app",
	Long: `Import a Liftoff refresh token and save it to
~/.config/liftoff-export/auth.json, the same place 'auth login' writes.

This is the login path for accounts that have no password to type —
Google Sign-In accounts can't use 'auth login' at all (#55). Liftoff has
no web login, so the refresh token has to be read off the phone app with
an HTTPS proxy such as mitmproxy or Proxyman: log in the app, then look
for the POST to '.../api/trpc/user.signIn' and copy the 'refreshToken'
out of its JSON response.

By default the token is exchanged for an access token immediately, which
both verifies it and fills in the expiry. Pass the token on --refresh-token
or on stdin:

    liftoff-export auth import --refresh-token "$RT"
    echo "$RT" | liftoff-export auth import

With --no-verify the CLI writes what you give it and makes no network
call; --access-token and --expires-at (RFC3339) are then required too.
Headless callers that already have a refresh token don't need this
command at all — set LIFTOFF_REFRESH_TOKEN and skip the token file.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		rt := strings.TrimSpace(importRefreshTokenFlag)
		if rt == "" {
			fmt.Fprint(cmd.ErrOrStderr(), "Refresh token: ")
			scanner := bufio.NewScanner(cmd.InOrStdin())
			scanner.Scan()
			rt = strings.TrimSpace(scanner.Text())
		}
		if rt == "" {
			return fmt.Errorf("a refresh token is required (--refresh-token or stdin)")
		}

		if importNoVerifyFlag {
			at := strings.TrimSpace(importAccessTokenFlag)
			exp := strings.TrimSpace(importExpiresAtFlag)
			if at == "" || exp == "" {
				return fmt.Errorf("--no-verify requires --access-token and --expires-at")
			}
			if _, err := time.Parse(time.RFC3339Nano, exp); err != nil {
				return fmt.Errorf("--expires-at must be RFC3339 (e.g. 2026-01-02T15:04:05Z): %w", err)
			}
			if err := auth.SaveFromCapture(at, rt, exp); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Imported (unverified). Tokens saved to ~/.config/liftoff-export/auth.json")
			return nil
		}

		store, err := auth.Refresh(rt)
		if err != nil {
			return fmt.Errorf("refresh token rejected: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(),
			"Imported. Tokens saved to ~/.config/liftoff-export/auth.json (token expires %s)\n",
			store.ExpiresAt.Local().Format(time.RFC3339))
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
	authCmd.AddCommand(importCmd)
	authCmd.AddCommand(logoutCmd)
	authCmd.AddCommand(refreshCmd)
	authCmd.AddCommand(statusCmd)

	importCmd.Flags().StringVar(&importRefreshTokenFlag, "refresh-token", "", "refresh token (read from stdin if omitted)")
	importCmd.Flags().StringVar(&importAccessTokenFlag, "access-token", "", "access token (only with --no-verify)")
	importCmd.Flags().StringVar(&importExpiresAtFlag, "expires-at", "", "access-token expiry, RFC3339 (only with --no-verify)")
	importCmd.Flags().BoolVar(&importNoVerifyFlag, "no-verify", false, "save tokens without exchanging them (no network call)")
}
