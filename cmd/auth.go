package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

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

func init() {
	authCmd.AddCommand(loginCmd)
	authCmd.AddCommand(logoutCmd)
	authCmd.AddCommand(refreshCmd)
}
