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

// ResponseData defines the standard API response format from AI Gateway
type ResponseData struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Success bool   `json:"success"`
	Data    any    `json:"data,omitempty"`
}

type QuotaResponse struct {
	Quota  int    `json:"quota"`
	UserID string `json:"user_id"`
}

type StarResponse struct {
	UserID    string `json:"user_id"`
	StarValue bool   `json:"star_value"`
}

type StarSetRequest struct {
	UserID    string `json:"user_id"`
	StarValue bool   `json:"star_value"`
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

	// Parse the response to check for success
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	var respData ResponseData
	if err := json.Unmarshal(body, &respData); err != nil {
		return fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if !respData.Success {
		return fmt.Errorf("AI Gateway error: %s - %s", respData.Code, respData.Message)
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

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var respData ResponseData
	if err := json.Unmarshal(body, &respData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if !respData.Success {
		return nil, fmt.Errorf("AI Gateway error: %s - %s", respData.Code, respData.Message)
	}

	// Parse the data field
	dataMap, ok := respData.Data.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid response data format")
	}

	quota, ok := dataMap["quota"].(float64)
	if !ok {
		return nil, fmt.Errorf("invalid quota format in response")
	}

	return &QuotaResponse{
		Quota:  int(quota),
		UserID: userID,
	}, nil
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

	// Parse the response to check for success
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	var respData ResponseData
	if err := json.Unmarshal(body, &respData); err != nil {
		return fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if !respData.Success {
		return fmt.Errorf("AI Gateway error: %s - %s", respData.Code, respData.Message)
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

// QueryGithubStar queries user's GitHub star status for zgsm-ai.zgsm project
func (c *Client) QueryGithubStar(userID string) (*StarResponse, error) {
	apiUrl := fmt.Sprintf("%s%s/star?user_id=%s", c.BaseURL, c.AdminPath, userID)

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

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var respData ResponseData
	if err := json.Unmarshal(body, &respData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if !respData.Success {
		return nil, fmt.Errorf("AI Gateway error: %s - %s", respData.Code, respData.Message)
	}

	// Parse the data field
	dataMap, ok := respData.Data.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid response data format")
	}

	starValueStr, ok := dataMap["star_value"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid star_value format in response")
	}

	starValue := starValueStr == "true"

	return &StarResponse{
		UserID:    userID,
		StarValue: starValue,
	}, nil
}

// SetGithubStar sets user's GitHub star status for zgsm-ai.zgsm project
func (c *Client) SetGithubStar(userID string, starValue bool) error {
	apiUrl := fmt.Sprintf("%s%s/star/set", c.BaseURL, c.AdminPath)

	data := url.Values{}
	data.Set("user_id", userID)
	data.Set("star_value", strconv.FormatBool(starValue))

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

	// Parse the response to check for success
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	var respData ResponseData
	if err := json.Unmarshal(body, &respData); err != nil {
		return fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if !respData.Success {
		return fmt.Errorf("AI Gateway error: %s - %s", respData.Code, respData.Message)
	}

	return nil
}

// SetUserPermission sets user permission in Higress
func (c *Client) SetUserPermission(employeeNumber string, models []string) error {
	// Prepare request data
	data := url.Values{}
	data.Set("employee_number", employeeNumber)

	// Convert models to JSON string
	modelsJSON, err := json.Marshal(models)
	if err != nil {
		return fmt.Errorf("failed to marshal models: %w", err)
	}
	data.Set("models", string(modelsJSON))

	// Create request
	requestURL := fmt.Sprintf("%s/model-permission/set", c.BaseURL)
	req, err := http.NewRequest("POST", requestURL, strings.NewReader(data.Encode()))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set(c.AuthHeader, c.AuthValue)

	// Make request
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Higress returned status: %d", resp.StatusCode)
	}

	// Parse response to check for success
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if success, ok := result["success"].(bool); !ok || !success {
		if message, ok := result["message"].(string); ok {
			return fmt.Errorf("Higress error: %s", message)
		}
		return fmt.Errorf("Higress operation failed")
	}

	return nil
}
