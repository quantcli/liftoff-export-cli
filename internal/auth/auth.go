package auth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	apiBase          = "https://v2-12-2.api.getgymbros.com"
	refreshProcedure = "user.refreshToken"
	userAgent        = "Liftoff/528 CFNetwork/3860.400.51 Darwin/25.3.0"
)

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
		apiBase, refreshProcedure, url.QueryEscape(string(input)))

	req, _ := http.NewRequest("GET", reqURL, nil)
	req.Header.Set("Accept", "*/*")
	req.Header.Set("User-Agent", userAgent)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

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
	if err := json.NewDecoder(resp.Body).Decode(&batch); err != nil {
		return nil, err
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
	reqURL := fmt.Sprintf("%s/api/trpc/user.signIn?batch=1", apiBase)
	req, _ := http.NewRequest("POST", reqURL, bytes.NewReader(input))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("User-Agent", userAgent)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

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
	if err := json.NewDecoder(resp.Body).Decode(&batch); err != nil {
		return err
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
