package services

import (
	"fmt"
	"quota-manager/pkg/aigateway"
)

// AiGatewayAdminService is a thin wrapper around aigateway.Client for admin passthrough APIs
type AiGatewayAdminService struct {
	client *aigateway.Client
}

func NewAiGatewayAdminService(client *aigateway.Client) *AiGatewayAdminService {
	return &AiGatewayAdminService{client: client}
}

// Quota total
func (s *AiGatewayAdminService) QueryQuota(userID string) (float64, error) {
	return s.client.QueryQuotaValue(userID)
}

func (s *AiGatewayAdminService) RefreshQuota(userID string, quota float64) error {
	return s.client.RefreshQuota(userID, quota)
}

func (s *AiGatewayAdminService) DeltaQuota(userID string, value float64) error {
	return s.client.DeltaQuota(userID, value)
}

// Quota used
func (s *AiGatewayAdminService) QueryUsedQuota(userID string) (float64, error) {
	return s.client.QueryUsedQuotaValue(userID)
}

func (s *AiGatewayAdminService) RefreshUsedQuota(userID string, quota float64) error {
	return s.client.RefreshUsedQuota(userID, quota)
}

func (s *AiGatewayAdminService) DeltaUsedQuota(userID string, value float64) error {
	return s.client.DeltaUsedQuota(userID, value)
}

// Star projects
func (s *AiGatewayAdminService) QueryStarProjects(employeeNumber string) (string, error) {
	resp, err := s.client.QueryGithubStarProjects(employeeNumber)
	if err != nil {
		return "", err
	}
	return resp.StarredProjects, nil
}

func (s *AiGatewayAdminService) SetStarProjects(employeeNumber string, projectsCSV string) error {
	return s.client.SetGithubStarProjects(employeeNumber, projectsCSV)
}

// Star check toggle
func (s *AiGatewayAdminService) QueryStarCheck(employeeNumber string) (bool, error) {
	resp, err := s.client.QueryStarCheckPermission(employeeNumber)
	if err != nil {
		return false, err
	}
	return resp.Enabled, nil
}

func (s *AiGatewayAdminService) SetStarCheck(employeeNumber string, enabled bool) error {
	return s.client.SetUserStarCheckPermission(employeeNumber, enabled)
}

// Quota check toggle
func (s *AiGatewayAdminService) QueryQuotaCheck(employeeNumber string) (bool, error) {
	resp, err := s.client.QueryQuotaCheckPermission(employeeNumber)
	if err != nil {
		return false, err
	}
	return resp.Enabled, nil
}

func (s *AiGatewayAdminService) SetQuotaCheck(employeeNumber string, enabled bool) error {
	return s.client.SetUserQuotaCheckPermission(employeeNumber, enabled)
}

// Models permission
func (s *AiGatewayAdminService) SetUserModels(employeeNumber string, models []string) error {
	if employeeNumber == "" {
		return fmt.Errorf("employee_number cannot be empty")
	}
	return s.client.SetUserPermission(employeeNumber, models)
}

func (s *AiGatewayAdminService) QueryUserModels(employeeNumber string) ([]string, error) {
	if employeeNumber == "" {
		return nil, fmt.Errorf("employee_number cannot be empty")
	}
	resp, err := s.client.QueryUserPermission(employeeNumber)
	if err != nil {
		return nil, err
	}
	return resp.Models, nil
}
