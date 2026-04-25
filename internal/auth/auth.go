package auth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	defaultAPIBase   = "https://v2-12-3.api.getgymbros.com"
	refreshProcedure = "user.refreshToken"
	userAgent        = "Liftoff/528 CFNetwork/3860.400.51 Darwin/25.3.0"
)

const apiBaseEnvVar = "LIFTOFF_API_BASE"

var (
	resolveOnce sync.Once
	resolved    string
)

// ResolveAPIBase returns the env override LIFTOFF_API_BASE if set, else the
// compiled-in default. Liftoff mints version-pinned hosts (e.g. v2-13-0) and
// retires older ones; the env var lets users dodge a deprecation without
// waiting for a new release. Logged once when an override is in effect so
// users can tell which endpoint is active when something breaks.
func ResolveAPIBase() string {
	resolveOnce.Do(func() {
		if v := strings.TrimSpace(os.Getenv(apiBaseEnvVar)); v != "" {
			resolved = strings.TrimRight(v, "/")
			fmt.Fprintf(os.Stderr, "liftoff-export: using %s=%s\n", apiBaseEnvVar, resolved)
		} else {
			resolved = defaultAPIBase
		}
	})
	return resolved
}

// deprecatedMarker is the substring the Liftoff backend returns when the
// version-pinned host this binary targets has been retired.
const deprecatedMarker = "server is deprecated"

func DeprecatedError(action string) error {
	return fmt.Errorf("%s: Liftoff retired the API version this binary targets (%s). "+
		"Workarounds: (a) update to a newer liftoff-export release, or "+
		"(b) set %s=https://vX-Y-Z.api.getgymbros.com to point at a current version. "+
		"The Liftoff iOS/Android app shows its version under Settings → About; matching that is usually safe.",
		action, ResolveAPIBase(), apiBaseEnvVar)
}

type TokenStore struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
}

func configPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "liftoff-export", "auth.json")
}

// GetToken returns a valid access token, refreshing if needed.
func GetToken() (string, error) {
	store, err := load()
	if err != nil {
		return "", fmt.Errorf("not logged in — run: liftoff-export auth login")
	}
	if time.Now().After(store.ExpiresAt) {
		store, err = Refresh(store.RefreshToken)
		if err != nil {
			return "", fmt.Errorf("token refresh failed: %w", err)
		}
	}
	return store.AccessToken, nil
}

// Refresh exchanges a refresh token for a new access token via user.refreshToken.
func Refresh(refreshToken string) (*TokenStore, error) {
	// tRPC batch GET: /api/trpc/user.refreshToken?batch=1&input={"0":{"json":"<token>"}}
	input, _ := json.Marshal(map[string]any{
		"0": map[string]any{"json": refreshToken},
	})
	reqURL := fmt.Sprintf("%s/api/trpc/%s?batch=1&input=%s",
		ResolveAPIBase(), refreshProcedure, url.QueryEscape(string(input)))

	req, _ := http.NewRequest("GET", reqURL, nil)
	req.Header.Set("Accept", "*/*")
	req.Header.Set("User-Agent", userAgent)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if strings.Contains(strings.ToLower(string(body)), deprecatedMarker) {
		return nil, DeprecatedError("token refresh")
	}

	var batch []struct {
		Result *struct {
			Data struct {
				JSON struct {
					AccessToken          string `json:"accessToken"`
					AccessTokenExpiresAt string `json:"accessTokenExpiresAt"`
				} `json:"json"`
			} `json:"data"`
		} `json:"result"`
		Error *struct {
			JSON struct{ Message string `json:"message"` } `json:"json"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &batch); err != nil {
		return nil, fmt.Errorf("parse tRPC response: %w\nbody: %s", err, string(body))
	}
	if len(batch) == 0 || batch[0].Error != nil {
		msg := "unknown error"
		if len(batch) > 0 && batch[0].Error != nil {
			msg = batch[0].Error.JSON.Message
		}
		return nil, fmt.Errorf("refresh failed: %s", msg)
	}

	expiresAt, _ := time.Parse(time.RFC3339Nano, batch[0].Result.Data.JSON.AccessTokenExpiresAt)

	store := &TokenStore{
		AccessToken:  batch[0].Result.Data.JSON.AccessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    expiresAt.Add(-5 * time.Minute), // refresh 5min early
	}
	return store, Save(store)
}

// Logout removes the stored auth tokens.
func Logout() error {
	err := os.Remove(configPath())
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// Login authenticates via the Liftoff tRPC user.signIn procedure.
func Login(email, password string) error {
	input, _ := json.Marshal(map[string]any{
		"0": map[string]any{
			"json": map[string]any{
				"usernameOrEmail": email,
				"password":        password,
				"provider":        "gymbros",
			},
		},
	})
	reqURL := fmt.Sprintf("%s/api/trpc/user.signIn?batch=1", ResolveAPIBase())
	req, _ := http.NewRequest("POST", reqURL, bytes.NewReader(input))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("User-Agent", userAgent)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if strings.Contains(strings.ToLower(string(body)), deprecatedMarker) {
		return DeprecatedError("login")
	}

	var batch []struct {
		Result *struct {
			Data struct {
				JSON struct {
					AccessToken          string `json:"accessToken"`
					RefreshToken         string `json:"refreshToken"`
					AccessTokenExpiresAt string `json:"accessTokenExpiresAt"`
				} `json:"json"`
			} `json:"data"`
		} `json:"result"`
		Error *struct {
			JSON struct{ Message string `json:"message"` } `json:"json"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &batch); err != nil {
		return fmt.Errorf("parse tRPC response: %w\nbody: %s", err, string(body))
	}
	if len(batch) == 0 || batch[0].Result == nil {
		msg := "unknown error"
		if len(batch) > 0 && batch[0].Error != nil {
			msg = batch[0].Error.JSON.Message
		}
		return fmt.Errorf("login failed: %s", msg)
	}

	expiresAt, _ := time.Parse(time.RFC3339Nano, batch[0].Result.Data.JSON.AccessTokenExpiresAt)
	store := &TokenStore{
		AccessToken:  batch[0].Result.Data.JSON.AccessToken,
		RefreshToken: batch[0].Result.Data.JSON.RefreshToken,
		ExpiresAt:    expiresAt.Add(-5 * time.Minute),
	}
	return Save(store)
}

// SaveFromCapture stores tokens captured manually (e.g. from MITM).
func SaveFromCapture(accessToken, refreshToken, expiresAt string) error {
	exp, _ := time.Parse(time.RFC3339Nano, expiresAt)
	return Save(&TokenStore{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    exp.Add(-5 * time.Minute),
	})
}

func Save(store *TokenStore) error {
	path := configPath()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, _ := json.MarshalIndent(store, "", "  ")
	return os.WriteFile(path, data, 0600)
}

func load() (*TokenStore, error) {
	data, err := os.ReadFile(configPath())
	if err != nil {
		return nil, err
	}
	var store TokenStore
	return &store, json.Unmarshal(data, &store)
}

// parseBearer extracts "Bearer <token>" from an Authorization header value.
func parseBearer(header string) string {
	if strings.HasPrefix(header, "Bearer ") {
		return strings.TrimPrefix(header, "Bearer ")
	}
	return header
}
