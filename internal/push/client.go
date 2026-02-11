package push

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

// PushRequest is the payload sent to the push gateway's POST /v1/push endpoint.
type PushRequest struct {
	LicenseKey   string `json:"license_key"`
	PushToken    string `json:"push_token"`
	PushPlatform string `json:"push_platform"` // "fcm" or "apns"
	CallerID     string `json:"caller_id"`
	CallID       string `json:"call_id"`
}

// PushResponse is the response from POST /v1/push.
type PushResponse struct {
	Delivered bool   `json:"delivered"`
	CallID    string `json:"call_id"`
}

// envelope is the standard push gateway response wrapper.
type envelope struct {
	Data  json.RawMessage `json:"data"`
	Error string          `json:"error,omitempty"`
}

// Client is an HTTP client for communicating with the push gateway service.
type Client struct {
	httpClient *http.Client
	baseURL    string
	licenseKey string
}

// NewClient creates a new push gateway HTTP client.
// baseURL is the push gateway endpoint (e.g., "https://push.flowpbx.com").
// licenseKey is the PBX instance's license key sent with each request.
func NewClient(baseURL, licenseKey string) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 10 * time.Second},
		baseURL:    baseURL,
		licenseKey: licenseKey,
	}
}

// SendPush sends a push notification request to the gateway for waking a
// mobile app on an incoming call. It returns whether the push was delivered
// successfully.
func (c *Client) SendPush(ctx context.Context, pushToken, pushPlatform, callerID, callID string) (bool, error) {
	req := PushRequest{
		LicenseKey:   c.licenseKey,
		PushToken:    pushToken,
		PushPlatform: pushPlatform,
		CallerID:     callerID,
		CallID:       callID,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return false, fmt.Errorf("push: marshalling request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/push", bytes.NewReader(body))
	if err != nil {
		return false, fmt.Errorf("push: creating request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-License-Key", c.licenseKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return false, fmt.Errorf("push: sending request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return false, fmt.Errorf("push: reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var env envelope
		if json.Unmarshal(respBody, &env) == nil && env.Error != "" {
			return false, fmt.Errorf("push: gateway error (status %d): %s", resp.StatusCode, env.Error)
		}
		return false, fmt.Errorf("push: gateway returned status %d", resp.StatusCode)
	}

	var env envelope
	if err := json.Unmarshal(respBody, &env); err != nil {
		return false, fmt.Errorf("push: decoding response: %w", err)
	}

	var pushResp PushResponse
	if err := json.Unmarshal(env.Data, &pushResp); err != nil {
		return false, fmt.Errorf("push: decoding push response data: %w", err)
	}

	slog.Debug("push notification sent",
		"delivered", pushResp.Delivered,
		"call_id", callID,
		"platform", pushPlatform,
	)

	return pushResp.Delivered, nil
}

// Configured returns true if the client has a valid base URL and license key.
func (c *Client) Configured() bool {
	return c.baseURL != "" && c.licenseKey != ""
}
