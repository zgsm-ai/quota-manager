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
	Credential string
	HTTPClient *http.Client
}

type QuotaResponse struct {
	Quota    int    `json:"quota"`
	Consumer string `json:"consumer"`
}

func NewClient(baseURL, adminPath, credential string) *Client {
	return &Client{
		BaseURL:    baseURL,
		AdminPath:  adminPath,
		Credential: credential,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// RefreshQuota refreshes user quota
func (c *Client) RefreshQuota(consumer string, quota int) error {
	apiUrl := fmt.Sprintf("%s%s/quota/refresh", c.BaseURL, c.AdminPath)

	data := url.Values{}
	data.Set("consumer", consumer)
	data.Set("quota", strconv.Itoa(quota))

	req, err := http.NewRequest("POST", apiUrl, strings.NewReader(data.Encode()))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.Credential)
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
func (c *Client) QueryQuota(consumer string) (*QuotaResponse, error) {
	apiUrl := fmt.Sprintf("%s%s/quota?consumer=%s", c.BaseURL, c.AdminPath, consumer)

	req, err := http.NewRequest("GET", apiUrl, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.Credential)

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
func (c *Client) DeltaQuota(consumer string, value int) error {
	apiUrl := fmt.Sprintf("%s%s/quota/delta", c.BaseURL, c.AdminPath)

	data := url.Values{}
	data.Set("consumer", consumer)
	data.Set("value", strconv.Itoa(value))

	req, err := http.NewRequest("POST", apiUrl, strings.NewReader(data.Encode()))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.Credential)
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
