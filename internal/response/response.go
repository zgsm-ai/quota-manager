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
	BadRequestCode   = "quota-manager.bad_request"
	UnauthorizedCode = "quota-manager.unauthorized"
	NotFoundCode     = "quota-manager.not_found"

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

	// The following codes are used for internal only

	// Model permission codes
	ModelPermissionWhitelistExistsCode              = "quota-manager.whitelist_exists"
	ModelPermissionUserNotFoundCode                 = "quota-manager.user_not_found"
	ModelPermissionDepartmentNotFoundCode           = "quota-manager.department_not_found"
	ModelPermissionDatabaseErrorCode                = "quota-manager.database_error"
	ModelPermissionSetUserWhitelistFailedCode       = "quota-manager.set_user_whitelist_failed"
	ModelPermissionSetDepartmentWhitelistFailedCode = "quota-manager.set_department_whitelist_failed"
	ModelPermissionGetUserWhitelistFailedCode       = "quota-manager.get_user_whitelist_failed"
	ModelPermissionGetDepartmentWhitelistFailedCode = "quota-manager.get_department_whitelist_failed"
	ModelPermissionGetPermissionsFailedCode         = "quota-manager.get_permissions_failed"

	// Star check permission codes
	StarCheckPermissionSettingExistsCode              = "quota-manager.setting_exists"
	StarCheckPermissionUserNotFoundCode               = "quota-manager.user_not_found"
	StarCheckPermissionDepartmentNotFoundCode         = "quota-manager.department_not_found"
	StarCheckPermissionDatabaseErrorCode              = "quota-manager.database_error"
	StarCheckPermissionSetUserSettingFailedCode       = "quota-manager.set_user_setting_failed"
	StarCheckPermissionSetDepartmentSettingFailedCode = "quota-manager.set_department_setting_failed"
	StarCheckPermissionGetUserSettingFailedCode       = "quota-manager.get_user_setting_failed"
	StarCheckPermissionGetDepartmentSettingFailedCode = "quota-manager.get_department_setting_failed"
	StarCheckPermissionGetPermissionsFailedCode       = "quota-manager.get_permissions_failed"

	// Quota check permission codes
	QuotaCheckPermissionUserNotFoundCode               = "quota-manager.user_not_found"
	QuotaCheckPermissionDepartmentNotFoundCode         = "quota-manager.department_not_found"
	QuotaCheckPermissionDatabaseErrorCode              = "quota-manager.database_error"
	QuotaCheckPermissionSetUserSettingFailedCode       = "quota-manager.set_user_setting_failed"
	QuotaCheckPermissionSetDepartmentSettingFailedCode = "quota-manager.set_department_setting_failed"
	QuotaCheckPermissionGetUserSettingFailedCode       = "quota-manager.get_user_setting_failed"
	QuotaCheckPermissionGetDepartmentSettingFailedCode = "quota-manager.get_department_setting_failed"
	QuotaCheckPermissionGetPermissionsFailedCode       = "quota-manager.get_permissions_failed"

	UnifiedPermissionInvalidTypeCode = "quota-manager.invalid_permission_type"
	EmployeeSyncFailedCode           = "quota-manager.employee_sync_failed"
)
