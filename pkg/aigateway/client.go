package aigateway

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"quota-manager/internal/utils"
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
	Quota  float64 `json:"quota"`
	UserID string  `json:"user_id"`
}

type StarProjectsResponse struct {
	EmployeeNumber  string `json:"employee_number"`
	StarredProjects string `json:"starred_projects"` // Comma-separated list
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

// RefreshQuota refreshes user quota with retry mechanism
func (c *Client) RefreshQuota(userID string, quota float64) error {
	_, err := utils.WithRetry(context.Background(), func() (struct{}, error) {
		return struct{}{}, c.refreshQuotaImpl(userID, quota)
	})
	return err
}

// refreshQuotaImpl implements the actual RefreshQuota logic
func (c *Client) refreshQuotaImpl(userID string, quota float64) error {
	apiUrl := fmt.Sprintf("%s%s/refresh", c.BaseURL, c.AdminPath)

	data := url.Values{}
	data.Set("user_id", userID)
	data.Set("quota", strconv.FormatFloat(quota, 'f', -1, 64))

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
		// Wrap error with HTTP status code for retry classification
		return &utils.HTTPError{
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("AI Gateway error: %s - %s", respData.Code, respData.Message),
		}
	}

	return nil
}

// QueryQuota queries user quota with retry mechanism
func (c *Client) QueryQuota(userID string) (*QuotaResponse, error) {
	return utils.WithRetry(context.Background(), func() (*QuotaResponse, error) {
		return c.queryQuotaImpl(userID)
	})
}

// queryQuotaImpl implements the actual QueryQuota logic
func (c *Client) queryQuotaImpl(userID string) (*QuotaResponse, error) {
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
		return nil, &utils.HTTPError{
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("AI Gateway error: %s - %s", respData.Code, respData.Message),
		}
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
		Quota:  quota,
		UserID: userID,
	}, nil
}

// DeltaQuota increases or decreases user quota with retry mechanism
func (c *Client) DeltaQuota(userID string, value float64) error {
	_, err := utils.WithRetry(context.Background(), func() (struct{}, error) {
		return struct{}{}, c.deltaQuotaImpl(userID, value)
	})
	return err
}

// deltaQuotaImpl implements the actual DeltaQuota logic
func (c *Client) deltaQuotaImpl(userID string, value float64) error {
	apiUrl := fmt.Sprintf("%s%s/delta", c.BaseURL, c.AdminPath)

	data := url.Values{}
	data.Set("user_id", userID)
	data.Set("value", strconv.FormatFloat(value, 'f', -1, 64))

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
		return &utils.HTTPError{
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("AI Gateway error: %s - %s", respData.Code, respData.Message),
		}
	}

	return nil
}

// QueryQuotaValue implements the QuotaQuerier interface with retry mechanism
// Returns only the quota value as a float64
func (c *Client) QueryQuotaValue(userID string) (float64, error) {
	return utils.WithRetry(context.Background(), func() (float64, error) {
		return c.queryQuotaValueImpl(userID)
	})
}

// queryQuotaValueImpl implements the actual QueryQuotaValue logic
func (c *Client) queryQuotaValueImpl(userID string) (float64, error) {
	resp, err := c.QueryQuota(userID)
	if err != nil {
		return 0, err
	}
	return resp.Quota, nil
}

// QueryGithubStarProjects queries user's starred GitHub projects with retry mechanism (returns comma-separated list)
func (c *Client) QueryGithubStarProjects(employeeNumber string) (*StarProjectsResponse, error) {
	return utils.WithRetry(context.Background(), func() (*StarProjectsResponse, error) {
		return c.queryGithubStarProjectsImpl(employeeNumber)
	})
}

// queryGithubStarProjectsImpl implements the actual QueryGithubStarProjects logic
func (c *Client) queryGithubStarProjectsImpl(employeeNumber string) (*StarProjectsResponse, error) {
	apiUrl := fmt.Sprintf("%s%s/star/projects/query?employee_number=%s", c.BaseURL, c.AdminPath, employeeNumber)

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
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, &utils.HTTPError{
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("API returned status %d: %s", resp.StatusCode, string(body)),
		}
	}

	var response ResponseData
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if !response.Success {
		return nil, &utils.HTTPError{
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("API returned error: %s", response.Message),
		}
	}

	// Parse the data field
	dataBytes, err := json.Marshal(response.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal data: %w", err)
	}

	var dataMap map[string]interface{}
	if err := json.Unmarshal(dataBytes, &dataMap); err != nil {
		return nil, fmt.Errorf("failed to parse data: %w", err)
	}

	starredProjects, _ := dataMap["starred_projects"].(string)

	return &StarProjectsResponse{
		EmployeeNumber:  employeeNumber,
		StarredProjects: starredProjects,
	}, nil
}

// SetGithubStarProjects sets user's starred GitHub projects (comma-separated list) with retry mechanism
func (c *Client) SetGithubStarProjects(employeeNumber string, starredProjects string) error {
	_, err := utils.WithRetry(context.Background(), func() (struct{}, error) {
		return struct{}{}, c.setGithubStarProjectsImpl(employeeNumber, starredProjects)
	})
	return err
}

// setGithubStarProjectsImpl implements the actual SetGithubStarProjects logic
func (c *Client) setGithubStarProjectsImpl(employeeNumber string, starredProjects string) error {
	apiUrl := fmt.Sprintf("%s%s/star/projects/set", c.BaseURL, c.AdminPath)

	data := url.Values{}
	data.Set("employee_number", employeeNumber)
	data.Set("starred_projects", starredProjects)

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
		// Wrap error with HTTP status code for retry classification
		return &utils.HTTPError{
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("AI Gateway error: %s - %s", respData.Code, respData.Message),
		}
	}

	return nil
}

// SetUserPermission sets user permission in Higress with retry mechanism
func (c *Client) SetUserPermission(employeeNumber string, models []string) error {
	_, err := utils.WithRetry(context.Background(), func() (struct{}, error) {
		return struct{}{}, c.setUserPermissionImpl(employeeNumber, models)
	})
	return err
}

// setUserPermissionImpl implements the actual SetUserPermission logic
func (c *Client) setUserPermissionImpl(employeeNumber string, models []string) error {
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
		return &utils.HTTPError{
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("Higress returned status: %d", resp.StatusCode),
		}
	}

	// Parse response to check for success
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if success, ok := result["success"].(bool); !ok || !success {
		if message, ok := result["message"].(string); ok {
			return &utils.HTTPError{
				StatusCode: resp.StatusCode,
				Message:    fmt.Sprintf("Higress error: %s", message),
			}
		}
		return &utils.HTTPError{
			StatusCode: resp.StatusCode,
			Message:    "Higress operation failed",
		}
	}

	return nil
}

// SetUserStarCheckPermission sets user star check permission in Higress with retry mechanism
func (c *Client) SetUserStarCheckPermission(employeeNumber string, enabled bool) error {
	_, err := utils.WithRetry(context.Background(), func() (struct{}, error) {
		return struct{}{}, c.setUserStarCheckPermissionImpl(employeeNumber, enabled)
	})
	return err
}

// setUserStarCheckPermissionImpl implements the actual SetUserStarCheckPermission logic
func (c *Client) setUserStarCheckPermissionImpl(employeeNumber string, enabled bool) error {
	// Prepare request data
	data := url.Values{}
	data.Set("employee_number", employeeNumber)
	if enabled {
		data.Set("enabled", "true")
	} else {
		data.Set("enabled", "false")
	}

	// Create request
	requestURL := fmt.Sprintf("%s/check-star/set", c.BaseURL)
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
		return &utils.HTTPError{
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("Higress returned status: %d", resp.StatusCode),
		}
	}

	// Parse response to check for success
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if success, ok := result["success"].(bool); !ok || !success {
		if message, ok := result["message"].(string); ok {
			return &utils.HTTPError{
				StatusCode: resp.StatusCode,
				Message:    fmt.Sprintf("Higress error: %s", message),
			}
		}
		return &utils.HTTPError{
			StatusCode: resp.StatusCode,
			Message:    "Higress operation failed",
		}
	}

	return nil
}

// SetUserQuotaCheckPermission sets user quota check permission in Higress with retry mechanism
func (c *Client) SetUserQuotaCheckPermission(employeeNumber string, enabled bool) error {
	_, err := utils.WithRetry(context.Background(), func() (struct{}, error) {
		return struct{}{}, c.setUserQuotaCheckPermissionImpl(employeeNumber, enabled)
	})
	return err
}

// setUserQuotaCheckPermissionImpl implements the actual SetUserQuotaCheckPermission logic
func (c *Client) setUserQuotaCheckPermissionImpl(employeeNumber string, enabled bool) error {
	// Prepare request data
	data := url.Values{}
	data.Set("employee_number", employeeNumber)
	if enabled {
		data.Set("enabled", "true")
	} else {
		data.Set("enabled", "false")
	}

	// Create request
	requestURL := fmt.Sprintf("%s/check-quota/set", c.BaseURL)
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
		return &utils.HTTPError{
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("Higress returned status: %d", resp.StatusCode),
		}
	}

	// Parse response to check for success
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if success, ok := result["success"].(bool); !ok || !success {
		if message, ok := result["message"].(string); ok {
			return &utils.HTTPError{
				StatusCode: resp.StatusCode,
				Message:    fmt.Sprintf("Higress error: %s", message),
			}
		}
		return &utils.HTTPError{
			StatusCode: resp.StatusCode,
			Message:    "Higress operation failed",
		}
	}

	return nil
}

// QueryUsedQuotaValue queries user used quota value with retry mechanism
// Returns only the used quota value as a float64
func (c *Client) QueryUsedQuotaValue(userID string) (float64, error) {
	return utils.WithRetry(context.Background(), func() (float64, error) {
		return c.queryUsedQuotaValueImpl(userID)
	})
}

// queryUsedQuotaValueImpl implements the actual QueryUsedQuotaValue logic
func (c *Client) queryUsedQuotaValueImpl(userID string) (float64, error) {
	apiUrl := fmt.Sprintf("%s%s/used?user_id=%s", c.BaseURL, c.AdminPath, userID)

	req, err := http.NewRequest("GET", apiUrl, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to create request: %w", err)
	}

	// Set admin key header if configured
	if c.AuthHeader != "" && c.AuthValue != "" {
		req.Header.Set(c.AuthHeader, c.AuthValue)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("failed to read response: %w", err)
	}

	var respData ResponseData
	if err := json.Unmarshal(body, &respData); err != nil {
		return 0, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if !respData.Success {
		return 0, &utils.HTTPError{
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("AI Gateway error: %s - %s", respData.Code, respData.Message),
		}
	}

	// Parse the data field
	dataMap, ok := respData.Data.(map[string]interface{})
	if !ok {
		return 0, fmt.Errorf("invalid response data format")
	}

	quota, ok := dataMap["quota"].(float64)
	if !ok {
		return 0, fmt.Errorf("invalid quota format in response")
	}

	return quota, nil
}

// DeltaUsedQuota increases or decreases user used quota with retry mechanism
func (c *Client) DeltaUsedQuota(userID string, value float64) error {
	_, err := utils.WithRetry(context.Background(), func() (struct{}, error) {
		return struct{}{}, c.deltaUsedQuotaImpl(userID, value)
	})
	return err
}

// deltaUsedQuotaImpl implements the actual DeltaUsedQuota logic
func (c *Client) deltaUsedQuotaImpl(userID string, value float64) error {
	apiUrl := fmt.Sprintf("%s%s/used/delta", c.BaseURL, c.AdminPath)

	data := url.Values{}
	data.Set("user_id", userID)
	data.Set("value", strconv.FormatFloat(value, 'f', -1, 64))

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
		return &utils.HTTPError{
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("AI Gateway error: %s - %s", respData.Code, respData.Message),
		}
	}

	return nil
}
