package client

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/quantcli/liftoff-export-cli/internal/auth"
)

// Base URL — versioned per Liftoff release. Update via: liftoff config set-url <url>
const BaseURL = "https://v2-12-2.api.getgymbros.com"

// UserAgent matches the iOS app so the server accepts our requests.
const UserAgent = "Liftoff/528 CFNetwork/3860.400.51 Darwin/25.3.0"

type Client struct {
	http    *http.Client
	baseURL string
}

func New() *Client {
	return &Client{
		http:    &http.Client{},
		baseURL: BaseURL,
	}
}

// Query calls a tRPC query procedure (HTTP GET) using the batch envelope format.
// procedure is e.g. "post.filteredPosts"
func (c *Client) Query(procedure string, input any, out any) error {
	token, err := auth.GetToken()
	if err != nil {
		return err
	}

	// tRPC batch format: input={"0":{"json":<args>}}
	// When input is nil, include meta to signal undefined (matches app behavior)
	var batchInput map[string]any
	if input == nil {
		batchInput = map[string]any{
			"0": map[string]any{"json": nil, "meta": map[string]any{"values": []string{"undefined"}}},
		}
	} else {
		batchInput = map[string]any{
			"0": map[string]any{"json": input},
		}
	}
	inputJSON, err := json.Marshal(batchInput)
	if err != nil {
		return err
	}

	reqURL := fmt.Sprintf("%s/api/trpc/%s?batch=1&input=%s",
		c.baseURL, procedure, url.QueryEscape(string(inputJSON)))

	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("User-Agent", UserAgent)

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode != 200 {
		return fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	// tRPC batch response: [{"result":{"data":{"json":<payload>}}}]
	var batch []struct {
		Result *struct {
			Data struct {
				JSON json.RawMessage `json:"json"`
			} `json:"data"`
		} `json:"result"`
		Error *struct {
			JSON struct {
				Message string `json:"message"`
			} `json:"json"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &batch); err != nil {
		return fmt.Errorf("failed to parse tRPC response: %w\nbody: %s", err, string(body))
	}
	if len(batch) == 0 {
		return fmt.Errorf("empty tRPC response")
	}
	if batch[0].Error != nil {
		return fmt.Errorf("tRPC error: %s", batch[0].Error.JSON.Message)
	}
	if out != nil {
		return json.Unmarshal(batch[0].Result.Data.JSON, out)
	}
	return nil
}
