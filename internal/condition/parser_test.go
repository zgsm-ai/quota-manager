package condition

import (
	"quota-manager/internal/models"
	"testing"
	"time"
)

// MockQuotaQuerier implements QuotaQuerier interface for testing
type MockQuotaQuerier struct {
	quotas map[string]int
}

func NewMockQuotaQuerier() *MockQuotaQuerier {
	return &MockQuotaQuerier{
		quotas: make(map[string]int),
	}
}

func (m *MockQuotaQuerier) QueryQuota(userID string) (int, error) {
	if quota, exists := m.quotas[userID]; exists {
		return quota, nil
	}
	return 0, nil
}

func (m *MockQuotaQuerier) SetQuota(userID string, quota int) {
	m.quotas[userID] = quota
}

func TestQuotaLEConditionWithMock(t *testing.T) {
	// Create a mock quota querier
	mockQuerier := NewMockQuotaQuerier()
	mockQuerier.SetQuota("user1", 5)
	mockQuerier.SetQuota("user2", 15)

	// Create evaluation context
	ctx := &EvaluationContext{
		QuotaQuerier: mockQuerier,
	}

	// Test users
	user1 := &models.UserInfo{ID: "user1"}
	user2 := &models.UserInfo{ID: "user2"}

	// Test quota-le condition
	condition := `quota-le("test-model", 10)`

	// user1 has quota 5, should satisfy quota-le(10)
	result1, err := CalcCondition(user1, condition, ctx)
	if err != nil {
		t.Fatalf("Failed to calculate condition for user1: %v", err)
	}
	if !result1 {
		t.Errorf("Expected user1 to satisfy condition, but got false")
	}

	// user2 has quota 15, should not satisfy quota-le(10)
	result2, err := CalcCondition(user2, condition, ctx)
	if err != nil {
		t.Fatalf("Failed to calculate condition for user2: %v", err)
	}
	if result2 {
		t.Errorf("Expected user2 to not satisfy condition, but got true")
	}
}

func TestConditionWithoutQuotaQuerier(t *testing.T) {
	// Test with nil quota querier
	ctx := &EvaluationContext{
		QuotaQuerier: nil,
	}

	user := &models.UserInfo{ID: "user1"}
	condition := `quota-le("test-model", 10)`

	_, err := CalcCondition(user, condition, ctx)
	if err == nil {
		t.Errorf("Expected error when quota querier is nil, but got none")
	}
}

func TestNonQuotaConditions(t *testing.T) {
	// Test conditions that don't require quota querier
	ctx := &EvaluationContext{
		QuotaQuerier: nil, // Can be nil for non-quota conditions
	}

	user := &models.UserInfo{
		ID:           "test_user",
		VIP:          2,
		Org:          "test_org",
		GithubStar:   "zgsm,openai/gpt-4",
		RegisterTime: time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
		AccessTime:   time.Now(),
	}

	tests := []struct {
		condition string
		expected  bool
	}{
		{`is-vip(1)`, true},
		{`is-vip(3)`, false},
		{`belong-to("test_org")`, true},
		{`belong-to("other_org")`, false},
		{`github-star("zgsm")`, true},
		{`github-star("nonexistent")`, false},
		{`register-before("2024-01-01 00:00:00")`, true},
		{`access-after("2020-01-01 00:00:00")`, true},
	}

	for _, test := range tests {
		result, err := CalcCondition(user, test.condition, ctx)
		if err != nil {
			t.Errorf("Failed to calculate condition '%s': %v", test.condition, err)
			continue
		}
		if result != test.expected {
			t.Errorf("Condition '%s': expected %v, got %v", test.condition, test.expected, result)
		}
	}
}
