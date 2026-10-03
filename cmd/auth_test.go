package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/quantcli/liftoff-export-cli/internal/auth"
)

// With LIFTOFF_REFRESH_TOKEN set, 'auth status' reports the headless source
// and exits 0 without consulting the saved token file — that file may not
// exist at all in a container. (#55)
func TestAuthStatus_HeadlessEnvWins(t *testing.T) {
	t.Setenv("LIFTOFF_REFRESH_TOKEN", "rt-from-env")
	t.Setenv("HOME", t.TempDir()) // no auth.json anywhere

	var out bytes.Buffer
	statusCmd.SetOut(&out)
	t.Cleanup(func() { statusCmd.SetOut(nil) })

	if err := statusCmd.RunE(statusCmd, nil); err != nil {
		t.Fatalf("headless status should succeed with no saved token, got: %v", err)
	}
	if !strings.Contains(out.String(), "LIFTOFF_REFRESH_TOKEN") {
		t.Errorf("status should name the token source, got: %q", out.String())
	}
}

// 'auth import --no-verify' writes the captured tokens straight to the
// token file with no network call, so a Google Sign-In account that has
// proxy-captured its tokens gets a working login. (#55)
func TestAuthImport_NoVerifyWritesTokenFile(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Cleanup(resetImportFlags)

	importRefreshTokenFlag = "rt-captured"
	importAccessTokenFlag = "at-captured"
	importExpiresAtFlag = "2099-01-02T15:04:05Z"
	importNoVerifyFlag = true

	var out bytes.Buffer
	importCmd.SetOut(&out)
	t.Cleanup(func() { importCmd.SetOut(nil) })

	if err := importCmd.RunE(importCmd, nil); err != nil {
		t.Fatalf("import --no-verify should succeed, got: %v", err)
	}

	store, err := auth.Load()
	if err != nil {
		t.Fatalf("token file should be readable after import: %v", err)
	}
	if store.RefreshToken != "rt-captured" || store.AccessToken != "at-captured" {
		t.Errorf("token file did not round-trip the captured tokens: %+v", store)
	}
}

// --no-verify without the companion values is a flag error, not a silent
// half-written token file.
func TestAuthImport_NoVerifyRequiresCompanions(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Cleanup(resetImportFlags)

	importRefreshTokenFlag = "rt-only"
	importNoVerifyFlag = true

	if err := importCmd.RunE(importCmd, nil); err == nil {
		t.Fatal("expected an error when --access-token / --expires-at are missing")
	}
}

// No token anywhere to import is a clear error, not a hang or a panic.
func TestAuthImport_NoTokenFails(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Cleanup(resetImportFlags)

	importCmd.SetIn(strings.NewReader("\n"))
	t.Cleanup(func() { importCmd.SetIn(nil) })

	if err := importCmd.RunE(importCmd, nil); err == nil {
		t.Fatal("expected an error when no refresh token is supplied")
	}
}

func resetImportFlags() {
	importRefreshTokenFlag = ""
	importAccessTokenFlag = ""
	importExpiresAtFlag = ""
	importNoVerifyFlag = false
}

// Without the env var and without a saved token, status still fails and
// points at both recovery paths.
func TestAuthStatus_NoTokenFails(t *testing.T) {
	t.Setenv("LIFTOFF_REFRESH_TOKEN", "")
	t.Setenv("HOME", t.TempDir())

	err := statusCmd.RunE(statusCmd, nil)
	if err == nil {
		t.Fatal("expected an error when no token is available")
	}
	if !strings.Contains(err.Error(), "LIFTOFF_REFRESH_TOKEN") {
		t.Errorf("error should mention the headless option, got: %v", err)
	}
}
