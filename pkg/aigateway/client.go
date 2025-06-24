package aigateway

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type Client struct {
	BaseURL    string
	AdminPath  string
	AuthHeader string
	AuthValue  string
	HTTPClient *http.Client
}

type QuotaResponse struct {
	Quota  int    `json:"quota"`
	UserID string `json:"user_id"`
}

func NewClient(baseURL, adminPath, authHeader, authValue string) *Client {
	return &Client{
		BaseURL:    baseURL,
		AdminPath:  adminPath,
		AuthHeader: authHeader,
		AuthValue:  authValue,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// RefreshQuota refreshes user quota
func (c *Client) RefreshQuota(userID string, quota int) error {
	apiUrl := fmt.Sprintf("%s%s/refresh", c.BaseURL, c.AdminPath)

	data := url.Values{}
	data.Set("user_id", userID)
	data.Set("quota", strconv.Itoa(quota))

	req, err := http.NewRequest("POST", apiUrl, strings.NewReader(data.Encode()))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set admin key header if configured
	if c.AuthHeader != "" && c.AuthValue != "" {
		req.Header.Set(c.AuthHeader, c.AuthValue)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}

// QueryQuota queries user quota
func (c *Client) QueryQuota(userID string) (*QuotaResponse, error) {
	apiUrl := fmt.Sprintf("%s%s?user_id=%s", c.BaseURL, c.AdminPath, userID)

	req, err := http.NewRequest("GET", apiUrl, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set admin key header if configured
	if c.AuthHeader != "" && c.AuthValue != "" {
		req.Header.Set(c.AuthHeader, c.AuthValue)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var quotaResp QuotaResponse
	if err := json.Unmarshal(body, &quotaResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &quotaResp, nil
}

// DeltaQuota increases or decreases user quota
func (c *Client) DeltaQuota(userID string, value int) error {
	apiUrl := fmt.Sprintf("%s%s/delta", c.BaseURL, c.AdminPath)

	data := url.Values{}
	data.Set("user_id", userID)
	data.Set("value", strconv.Itoa(value))

	req, err := http.NewRequest("POST", apiUrl, strings.NewReader(data.Encode()))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set admin key header if configured
	if c.AuthHeader != "" && c.AuthValue != "" {
		req.Header.Set(c.AuthHeader, c.AuthValue)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}

// QueryQuotaValue implements the QuotaQuerier interface
// Returns only the quota value as an integer
func (c *Client) QueryQuotaValue(userID string) (int, error) {
	resp, err := c.QueryQuota(userID)
	if err != nil {
		return 0, err
	}
	return resp.Quota, nil
}
