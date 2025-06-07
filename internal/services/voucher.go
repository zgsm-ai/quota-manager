package services

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// VoucherData represents the data structure in voucher code
type VoucherData struct {
	GiverID     string             `json:"giver_id"`
	GiverName   string             `json:"giver_name"`
	GiverPhone  string             `json:"giver_phone"`
	GiverGithub string             `json:"giver_github"`
	ReceiverID  string             `json:"receiver_id"`
	QuotaList   []VoucherQuotaItem `json:"quota_list"`
	Timestamp   int64              `json:"timestamp"`
}

// VoucherQuotaItem represents quota item in voucher
type VoucherQuotaItem struct {
	Amount     int       `json:"amount"`
	ExpiryDate time.Time `json:"expiry_date"`
}

// VoucherService handles voucher code generation and validation
type VoucherService struct {
	signingKey []byte
}

// NewVoucherService creates a new voucher service
func NewVoucherService(signingKey string) *VoucherService {
	return &VoucherService{
		signingKey: []byte(signingKey),
	}
}

// GenerateVoucher generates a voucher code
func (s *VoucherService) GenerateVoucher(data *VoucherData) (string, error) {
	// Set timestamp
	data.Timestamp = time.Now().Unix()

	// Serialize to JSON
	jsonData, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("failed to marshal voucher data: %w", err)
	}

	// Generate HMAC signature
	signature := s.generateSignature(jsonData)

	// Combine JSON and signature
	combined := string(jsonData) + "." + hex.EncodeToString(signature)

	// Base64URL encode
	voucherCode := base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString([]byte(combined))

	return voucherCode, nil
}

// ValidateAndDecodeVoucher validates and decodes a voucher code
func (s *VoucherService) ValidateAndDecodeVoucher(voucherCode string) (*VoucherData, error) {
	// Base64URL decode
	decoded, err := base64.URLEncoding.WithPadding(base64.NoPadding).DecodeString(voucherCode)
	if err != nil {
		return nil, fmt.Errorf("failed to decode voucher code: %w", err)
	}

	// Split by "."
	parts := strings.Split(string(decoded), ".")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid voucher code format")
	}

	jsonData := []byte(parts[0])
	signatureHex := parts[1]

	// Decode signature
	signature, err := hex.DecodeString(signatureHex)
	if err != nil {
		return nil, fmt.Errorf("failed to decode signature: %w", err)
	}

	// Verify signature
	expectedSignature := s.generateSignature(jsonData)
	if !hmac.Equal(signature, expectedSignature) {
		return nil, fmt.Errorf("invalid voucher signature")
	}

	// Unmarshal JSON data
	var data VoucherData
	if err := json.Unmarshal(jsonData, &data); err != nil {
		return nil, fmt.Errorf("failed to unmarshal voucher data: %w", err)
	}

	return &data, nil
}

// generateSignature generates HMAC-SHA256 signature
func (s *VoucherService) generateSignature(data []byte) []byte {
	h := hmac.New(sha256.New, s.signingKey)
	h.Write(data)
	return h.Sum(nil)
}
