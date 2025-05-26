package condition

import "quota-manager/pkg/aigateway"

// AiGatewayQuotaQuerier adapts aigateway.Client to implement QuotaQuerier interface
type AiGatewayQuotaQuerier struct {
	client *aigateway.Client
}

// NewAiGatewayQuotaQuerier creates a new adapter for aigateway.Client
func NewAiGatewayQuotaQuerier(client *aigateway.Client) QuotaQuerier {
	return &AiGatewayQuotaQuerier{client: client}
}

// QueryQuota implements QuotaQuerier interface
func (a *AiGatewayQuotaQuerier) QueryQuota(userID string) (int, error) {
	return a.client.QueryQuotaValue(userID)
}
