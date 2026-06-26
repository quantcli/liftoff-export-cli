package cmd

import (
	"bytes"
	"strings"
	"testing"
)

// On a flag-parse error the root command must not dump cobra's usage/flags
// block: stdout stays data-only and stderr stays a single error line that
// Execute prints itself. (#31)
func TestRootCmd_NoUsageDumpOnError(t *testing.T) {
	var out, errBuf bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&errBuf)
	rootCmd.SetArgs([]string{"--this-flag-does-not-exist"})
	t.Cleanup(func() {
		rootCmd.SetArgs(nil)
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
	})

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected an error for an unknown flag, got nil")
	}
	combined := out.String() + errBuf.String()
	if strings.Contains(combined, "Usage:") {
		t.Errorf("usage block should be suppressed on error; got:\n%s", combined)
	}
	if strings.Contains(combined, "Available Commands:") {
		t.Errorf("command list should be suppressed on error; got:\n%s", combined)
	}
}
