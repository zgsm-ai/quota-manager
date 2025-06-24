package response

// ResponseData defines the standard API response format
type ResponseData struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Success bool   `json:"success"`
	Data    any    `json:"data,omitempty"`
}

// NewSuccessResponse creates a success response
func NewSuccessResponse(data any, message string) ResponseData {
	if message == "" {
		message = "Operation successful"
	}
	return ResponseData{
		Code:    SuccessCode,
		Message: message,
		Success: true,
		Data:    data,
	}
}

// NewErrorResponse creates an error response
func NewErrorResponse(code string, message string) ResponseData {
	return ResponseData{
		Code:    code,
		Message: message,
		Success: false,
	}
}

// Success and error codes with meaningful names
const (
	// Success code
	SuccessCode = "quota-manager.success"

	// Client error codes
	BadRequestCode    = "quota-manager.bad_request"
	UnauthorizedCode  = "quota-manager.unauthorized"
	NotFoundCode      = "quota-manager.not_found"

	// Server error codes
	InternalErrorCode = "quota-manager.internal_error"

	// Business logic error codes
	InvalidStrategyIDCode    = "quota-manager.invalid_strategy_id"
	StrategyNotFoundCode     = "quota-manager.strategy_not_found"
	InsufficientQuotaCode    = "quota-manager.insufficient_quota"
	VoucherInvalidCode       = "quota-manager.voucher_invalid"
	VoucherExpiredCode       = "quota-manager.voucher_expired"
	VoucherRedeemedCode      = "quota-manager.voucher_already_redeemed"
	UserNotFoundCode         = "quota-manager.user_not_found"
	TokenInvalidCode         = "quota-manager.token_invalid"
	QuotaTransferFailedCode  = "quota-manager.quota_transfer_failed"
	StrategyCreateFailedCode = "quota-manager.strategy_create_failed"
	StrategyUpdateFailedCode = "quota-manager.strategy_update_failed"
	StrategyDeleteFailedCode = "quota-manager.strategy_delete_failed"
	DatabaseErrorCode        = "quota-manager.database_error"
	AiGatewayErrorCode       = "quota-manager.aigateway_error"
)