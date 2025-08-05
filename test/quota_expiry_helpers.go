package main

import (
	"fmt"
	"math"
	"quota-manager/internal/models"
	"time"

	"github.com/stretchr/testify/mock"
)

// 统一过期时间设置函数

// 获取上个月月底最后一天 23:59:59
func getLastMonthEndTime() time.Time {
	now := time.Now()
	currentYear, currentMonth, _ := now.Date()

	// 计算上个月
	if currentMonth == 1 {
		currentMonth = 12
		currentYear--
	} else {
		currentMonth--
	}

	// 获取上个月最后一天
	lastDay := time.Date(currentYear, currentMonth+1, 0, 23, 59, 59, 0, time.Local)
	return lastDay
}

// 获取指定月份的月底最后一天 23:59:59
func getMonthEndTime(year int, month time.Month) time.Time {
	lastDay := time.Date(year, month+1, 0, 23, 59, 59, 0, time.Local)
	return lastDay
}

// 创建已过期的测试配额数据
func createExpiredTestQuota(ctx *TestContext, userID string, amount float64) (*models.Quota, error) {
	expiryTime := getLastMonthEndTime()
	return createTestQuota(ctx, userID, amount, models.StatusValid, expiryTime)
}

// 创建未过期的测试配额数据
func createValidTestQuota(ctx *TestContext, userID string, amount float64) (*models.Quota, error) {
	// 使用秒级精度创建时间，避免数据库存储时的纳秒精度问题
	rawTime := time.Now().AddDate(0, 1, 0)
	expiryTime := time.Date(rawTime.Year(), rawTime.Month(), rawTime.Day(), rawTime.Hour(), rawTime.Minute(), rawTime.Second(), 0, rawTime.Location())
	return createTestQuota(ctx, userID, amount, models.StatusValid, expiryTime)
}

// 创建指定过期时间的测试配额数据
func createTestQuotaWithExpiry(ctx *TestContext, userID string, amount float64, expiryTime time.Time) (*models.Quota, error) {
	// 使用秒级精度创建时间，避免数据库存储时的纳秒精度问题
	normalizedTime := time.Date(expiryTime.Year(), expiryTime.Month(), expiryTime.Day(), expiryTime.Hour(), expiryTime.Minute(), expiryTime.Second(), 0, expiryTime.Location())
	return createTestQuota(ctx, userID, amount, models.StatusValid, normalizedTime)
}

// 创建测试配额数据（通用函数）
func createTestQuota(ctx *TestContext, userID string, amount float64, status string, expiryTime time.Time) (*models.Quota, error) {
	quota := &models.Quota{
		UserID:     userID,
		Amount:     amount,
		Status:     status,
		ExpiryDate: expiryTime,
		CreateTime: time.Now(),
		UpdateTime: time.Now(),
	}

	err := ctx.DB.Create(quota).Error
	if err != nil {
		return nil, err
	}
	return quota, nil
}

// 配额表数据验证函数

// 验证配额状态
func verifyQuotaStatus(ctx *TestContext, quotaID string, expectedStatus string) error {
	var quota models.Quota
	err := ctx.DB.Where("id = ?", quotaID).First(&quota).Error
	if err != nil {
		return err
	}
	if quota.Status != expectedStatus {
		return fmt.Errorf("expected status %s, got %s", expectedStatus, quota.Status)
	}
	return nil
}

// 验证配额过期时间是否为上个月月底
func verifyQuotaExpiredLastMonth(ctx *TestContext, quotaID string) error {
	var quota models.Quota
	err := ctx.DB.Where("id = ?", quotaID).First(&quota).Error
	if err != nil {
		return err
	}

	expectedTime := getLastMonthEndTime()

	// 验证时间是否一致（允许秒级精度差异）
	if quota.ExpiryDate.Unix() != expectedTime.Unix() {
		return fmt.Errorf("expected expiry time %v, got %v", expectedTime, quota.ExpiryDate)
	}
	return nil
}

// 验证用户有效配额数量
func verifyUserValidQuotaCount(ctx *TestContext, userID string, expectedCount int) error {
	var count int64
	err := ctx.DB.Model(&models.Quota{}).Where("user_id = ? AND status = ?", userID, models.StatusValid).Count(&count).Error
	if err != nil {
		return err
	}
	if int(count) != expectedCount {
		return fmt.Errorf("expected %d valid quotas, got %d", expectedCount, count)
	}
	return nil
}

// 验证用户过期配额数量
func verifyUserExpiredQuotaCount(ctx *TestContext, userID string, expectedCount int) error {
	var count int64
	err := ctx.DB.Model(&models.Quota{}).Where("user_id = ? AND status = ?", userID, models.StatusExpired).Count(&count).Error
	if err != nil {
		return err
	}
	if int(count) != expectedCount {
		return fmt.Errorf("expected %d expired quotas, got %d", expectedCount, count)
	}
	return nil
}

// 验证用户配额总金额（按状态分组）
func verifyUserQuotaAmountByStatus(ctx *TestContext, userID string, status string, expectedAmount float64) error {
	var totalAmount float64
	err := ctx.DB.Model(&models.Quota{}).
		Where("user_id = ? AND status = ?", userID, status).
		Select("COALESCE(SUM(amount), 0)").Scan(&totalAmount).Error
	if err != nil {
		return err
	}
	if math.Abs(totalAmount-expectedAmount) > 0.0001 {
		return fmt.Errorf("expected amount %f for status %s, got %f", expectedAmount, status, totalAmount)
	}
	return nil
}

// 验证用户所有配额记录的完整性
func verifyUserQuotaRecordsIntegrity(ctx *TestContext, userID string, expectedRecords []QuotaRecordExpectation) error {
	var quotas []models.Quota
	err := ctx.DB.Where("user_id = ?", userID).Order("expiry_date ASC").Find(&quotas).Error
	if err != nil {
		return err
	}

	if len(quotas) != len(expectedRecords) {
		return fmt.Errorf("expected %d quota records, got %d", len(expectedRecords), len(quotas))
	}

	for i, quota := range quotas {
		expected := expectedRecords[i]
		if quota.Status != expected.Status {
			return fmt.Errorf("quota record %d: expected status %s, got %s", i, expected.Status, quota.Status)
		}
		if math.Abs(quota.Amount-expected.Amount) > 0.0001 {
			return fmt.Errorf("quota record %d: expected amount %f, got %f", i, expected.Amount, quota.Amount)
		}

		// 使用秒级精度比较时间，避免数据库存储时纳秒精度截断导致的问题
		if quota.ExpiryDate.Unix() != expected.ExpiryDate.Unix() {
			return fmt.Errorf("quota record %d: expiry time mismatch (Unix timestamp): expected %d (%v), got %d (%v)",
				i, expected.ExpiryDate.Unix(), expected.ExpiryDate, quota.ExpiryDate.Unix(), quota.ExpiryDate)
		}
	}
	return nil
}

type QuotaRecordExpectation struct {
	Amount     float64
	ExpiryDate time.Time
	Status     string
}

// MockQuotaStore 数据验证函数

// 验证 MockQuotaStore 总配额同步
func verifyMockQuotaStoreTotalQuota(ctx *TestContext, userID string, expectedTotalQuota float64) error {
	// 从 MockQuotaStore 获取总配额
	actualTotalQuota := ctx.MockQuotaStore.GetQuota(userID)
	if math.Abs(actualTotalQuota-expectedTotalQuota) > 0.0001 {
		return fmt.Errorf("MockQuotaStore total quota mismatch: expected %f, got %f", expectedTotalQuota, actualTotalQuota)
	}
	return nil
}

// 验证 MockQuotaStore 已使用配额同步
func verifyMockQuotaStoreUsedQuota(ctx *TestContext, userID string, expectedUsedQuota float64) error {
	// 从 MockQuotaStore 获取已使用配额
	actualUsedQuota := ctx.MockQuotaStore.GetUsed(userID)
	if math.Abs(actualUsedQuota-expectedUsedQuota) > 0.0001 {
		return fmt.Errorf("MockQuotaStore used quota mismatch: expected %f, got %f", expectedUsedQuota, actualUsedQuota)
	}
	return nil
}

// 验证 MockQuotaStore delta 调用记录（不依赖顺序）
func verifyMockQuotaStoreDeltaCalls(ctx *TestContext, expectedCalls []MockQuotaStoreDeltaCall) error {
	actualCalls := ctx.MockQuotaStore.GetDeltaCalls()

	if len(actualCalls) != len(expectedCalls) {
		return fmt.Errorf("expected %d MockQuotaStore delta calls, got %d", len(expectedCalls), len(actualCalls))
	}

	// 创建期望调用的副本用于标记匹配状态
	expectedCallsCopy := make([]MockQuotaStoreDeltaCall, len(expectedCalls))
	copy(expectedCallsCopy, expectedCalls)
	matched := make([]bool, len(expectedCallsCopy))

	// 为每个实际调用寻找匹配的期望调用
	for _, actual := range actualCalls {
		found := false
		for i, expected := range expectedCallsCopy {
			if !matched[i] && actual.EmployeeNumber == expected.EmployeeNumber && math.Abs(actual.Delta-expected.Delta) <= 0.0001 {
				matched[i] = true
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("unexpected MockQuotaStore delta call: employee_number=%s, delta=%f", actual.EmployeeNumber, actual.Delta)
		}
	}

	// 检查是否有期望调用未匹配
	for i, expected := range expectedCallsCopy {
		if !matched[i] {
			return fmt.Errorf("expected MockQuotaStore delta call not found: employee_number=%s, delta=%f", expected.EmployeeNumber, expected.Delta)
		}
	}

	return nil
}

// 验证 MockQuotaStore used delta 调用记录（不依赖顺序）
func verifyMockQuotaStoreUsedDeltaCalls(ctx *TestContext, expectedCalls []MockQuotaStoreUsedDeltaCall) error {
	actualCalls := ctx.MockQuotaStore.GetUsedDeltaCalls()

	if len(actualCalls) != len(expectedCalls) {
		return fmt.Errorf("expected %d MockQuotaStore used delta calls, got %d", len(expectedCalls), len(actualCalls))
	}

	// 创建期望调用的副本用于标记匹配状态
	expectedCallsCopy := make([]MockQuotaStoreUsedDeltaCall, len(expectedCalls))
	copy(expectedCallsCopy, expectedCalls)
	matched := make([]bool, len(expectedCallsCopy))

	// 为每个实际调用寻找匹配的期望调用
	for _, actual := range actualCalls {
		found := false
		for i, expected := range expectedCallsCopy {
			if !matched[i] && actual.EmployeeNumber == expected.EmployeeNumber && math.Abs(actual.Delta-expected.Delta) <= 0.0001 {
				matched[i] = true
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("unexpected MockQuotaStore used delta call: employee_number=%s, delta=%f", actual.EmployeeNumber, actual.Delta)
		}
	}

	// 检查是否有期望调用未匹配
	for i, expected := range expectedCallsCopy {
		if !matched[i] {
			return fmt.Errorf("expected MockQuotaStore used delta call not found: employee_number=%s, delta=%f", expected.EmployeeNumber, expected.Delta)
		}
	}

	return nil
}

// 审计记录验证函数

// 验证配额过期审计记录是否存在
func verifyQuotaExpiryAuditExists(ctx *TestContext, userID string, expectedAmount float64) error {
	var auditCount int64
	err := ctx.DB.Model(&models.QuotaAudit{}).
		Where("user_id = ? AND amount < ? AND operation = ?", userID, 0, "EXPIRE").
		Count(&auditCount).Error
	if err != nil {
		return err
	}

	if auditCount == 0 {
		return fmt.Errorf("no expiry audit record found for user %s", userID)
	}

	// 验证最新的一条审计记录
	var auditRecord models.QuotaAudit
	err = ctx.DB.Where("user_id = ? AND amount < ? AND operation = ?", userID, 0, "EXPIRE").
		Order("create_time DESC").First(&auditRecord).Error
	if err != nil {
		return err
	}

	if math.Abs(auditRecord.Amount-expectedAmount) > 0.0001 {
		return fmt.Errorf("audit record amount mismatch: expected %f, got %f", expectedAmount, auditRecord.Amount)
	}

	if auditRecord.Operation != "EXPIRE" {
		return fmt.Errorf("audit record operation mismatch: expected EXPIRE, got %s", auditRecord.Operation)
	}

	return nil
}

// 验证没有生成意外的审计记录
func verifyNoUnexpectedAuditRecords(ctx *TestContext, userID string, excludedOperations []string) error {
	var auditRecords []models.QuotaAudit
	err := ctx.DB.Where("user_id = ?", userID).Find(&auditRecords).Error
	if err != nil {
		return err
	}

	for _, record := range auditRecords {
		// 检查是否在排除的操作列表中
		excluded := false
		for _, op := range excludedOperations {
			if record.Operation == op {
				excluded = true
				break
			}
		}

		if !excluded {
			return fmt.Errorf("found unexpected audit record: operation=%s, amount=%f", record.Operation, record.Amount)
		}
	}
	return nil
}

// 综合数据一致性验证函数

// 验证配额过期后的完整数据一致性
func verifyQuotaExpiryDataConsistency(ctx *TestContext, userID string, expectedData QuotaExpiryExpectation) error {
	// 1. 验证配额表数据
	if err := verifyUserValidQuotaCount(ctx, userID, expectedData.ValidQuotaCount); err != nil {
		return fmt.Errorf("valid quota count verification failed: %w", err)
	}

	if err := verifyUserExpiredQuotaCount(ctx, userID, expectedData.ExpiredQuotaCount); err != nil {
		return fmt.Errorf("expired quota count verification failed: %w", err)
	}

	if err := verifyUserQuotaAmountByStatus(ctx, userID, models.StatusValid, expectedData.ValidQuotaAmount); err != nil {
		return fmt.Errorf("valid quota amount verification failed: %w", err)
	}

	if err := verifyUserQuotaAmountByStatus(ctx, userID, models.StatusExpired, expectedData.ExpiredQuotaAmount); err != nil {
		return fmt.Errorf("expired quota amount verification failed: %w", err)
	}

	// 2. 验证 MockQuotaStore 数据
	if err := verifyMockQuotaStoreTotalQuota(ctx, userID, expectedData.MockQuotaStoreTotalQuota); err != nil {
		return fmt.Errorf("MockQuotaStore total quota verification failed: %w", err)
	}

	if err := verifyMockQuotaStoreUsedQuota(ctx, userID, expectedData.MockQuotaStoreUsedQuota); err != nil {
		return fmt.Errorf("MockQuotaStore used quota verification failed: %w", err)
	}

	// 3. 验证 MockQuotaStore 调用记录
	if err := verifyMockQuotaStoreDeltaCalls(ctx, expectedData.ExpectedDeltaCalls); err != nil {
		return fmt.Errorf("MockQuotaStore delta calls verification failed: %w", err)
	}

	if err := verifyMockQuotaStoreUsedDeltaCalls(ctx, expectedData.ExpectedUsedDeltaCalls); err != nil {
		return fmt.Errorf("MockQuotaStore used delta calls verification failed: %w", err)
	}

	// 4. 验证审计记录
	if expectedData.ExpectedAuditAmount != 0 {
		if err := verifyQuotaExpiryAuditExists(ctx, userID, expectedData.ExpectedAuditAmount); err != nil {
			return fmt.Errorf("audit record verification failed: %w", err)
		}
	}

	// 5. 验证没有意外审计记录
	if err := verifyNoUnexpectedAuditRecords(ctx, userID, expectedData.AllowedAuditOperations); err != nil {
		return fmt.Errorf("unexpected audit records verification failed: %w", err)
	}

	// 6. 验证配额记录完整性
	if len(expectedData.ExpectedQuotaRecords) > 0 {
		if err := verifyUserQuotaRecordsIntegrity(ctx, userID, expectedData.ExpectedQuotaRecords); err != nil {
			return fmt.Errorf("quota records integrity verification failed: %w", err)
		}
	}

	return nil
}

type QuotaExpiryExpectation struct {
	ValidQuotaCount          int
	ExpiredQuotaCount        int
	ValidQuotaAmount         float64
	ExpiredQuotaAmount       float64
	MockQuotaStoreTotalQuota float64
	MockQuotaStoreUsedQuota  float64
	ExpectedDeltaCalls       []MockQuotaStoreDeltaCall
	ExpectedUsedDeltaCalls   []MockQuotaStoreUsedDeltaCall
	ExpectedAuditAmount      float64
	AllowedAuditOperations   []string
	ExpectedQuotaRecords     []QuotaRecordExpectation
}

// MockQuotaStore 辅助函数

// 配置 MockQuotaStore 服务器响应
func setupMockQuotaStore(ctx *TestContext, shouldFail bool) {
	if shouldFail {
		ctx.MockQuotaStore.On("SyncQuota", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("float64")).
			Return(fmt.Errorf("MockQuotaStore sync failed"))
	} else {
		ctx.MockQuotaStore.On("SyncQuota", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("float64")).
			Return(nil)
	}
}

// 配置 MockQuotaStore 部分失败响应
func setupMockQuotaStorePartialFail(ctx *TestContext, failUserIDs []string) {
	ctx.MockQuotaStore.On("SyncQuota", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("float64")).
		Return(func(userID string, quotaType string, amount float64) error {
			for _, failUserID := range failUserIDs {
				if userID == failUserID {
					return fmt.Errorf("MockQuotaStore sync failed for user %s", userID)
				}
			}
			return nil
		})
}

// 验证 MockQuotaStore 调用
func verifyMockQuotaStoreCalls(ctx *TestContext, expectedCallCount int) error {
	if ctx.MockQuotaStore.CallCount != expectedCallCount {
		return fmt.Errorf("expected %d MockQuotaStore calls, got %d", expectedCallCount, ctx.MockQuotaStore.CallCount)
	}
	return nil
}

// 审计记录验证函数

// 验证配额过期审计记录
func verifyQuotaExpiryAudit(ctx *TestContext, quotaID string, expectedOperation string, expectedAmount float64) error {
	var auditRecord models.QuotaAudit
	err := ctx.DB.Where("operation = ?", expectedOperation).
		Order("create_time DESC").First(&auditRecord).Error
	if err != nil {
		return err
	}

	if math.Abs(auditRecord.Amount-expectedAmount) > 0.0001 {
		return fmt.Errorf("audit record amount mismatch: expected %f, got %f", expectedAmount, auditRecord.Amount)
	}

	if auditRecord.Operation != expectedOperation {
		return fmt.Errorf("audit record operation mismatch: expected %s, got %s", expectedOperation, auditRecord.Operation)
	}

	return nil
}

// 时间控制辅助函数

// 设置当前时间用于测试
func setCurrentTimeForTest(t time.Time) {
	// 这里需要根据实际的时间控制机制来实现
	// 可能需要使用 time mocking 库或者全局变量
}

// 重置时间为系统时间
func resetTimeForTest() {
	// 重置时间为系统时间
}

// 清理测试数据辅助函数

// 清理用户的所有配额数据
func cleanupUserQuotaData(ctx *TestContext, userID string) error {
	// 删除配额记录
	if err := ctx.DB.Where("user_id = ?", userID).Delete(&models.Quota{}).Error; err != nil {
		return err
	}

	// 删除审计记录
	if err := ctx.DB.Where("user_id = ?", userID).Delete(&models.QuotaAudit{}).Error; err != nil {
		return err
	}

	// 删除月度配额使用记录
	if err := ctx.DB.Where("user_id = ?", userID).Delete(&models.MonthlyQuotaUsage{}).Error; err != nil {
		return err
	}

	return nil
}

// 清理 MockQuotaStore 数据
func cleanupMockQuotaStore(ctx *TestContext) {
	ctx.MockQuotaStore.ClearData()
	ctx.MockQuotaStore.ClearAllCalls()
}

// 创建测试数据辅助函数

// 创建测试用户和配额的完整数据
func createTestUserWithQuota(ctx *TestContext, userName, displayName string, quotaAmount float64, isExpired bool) (*models.UserInfo, *models.Quota, error) {
	// 创建用户
	user := createTestUser(userName, displayName, 0)
	if err := ctx.DB.Create(user).Error; err != nil {
		return nil, nil, fmt.Errorf("create user failed: %v", err)
	}

	// 创建配额
	var quota *models.Quota
	var err error
	if isExpired {
		quota, err = createExpiredTestQuota(ctx, user.ID, quotaAmount)
	} else {
		quota, err = createValidTestQuota(ctx, user.ID, quotaAmount)
	}

	if err != nil {
		return nil, nil, fmt.Errorf("create quota failed: %v", err)
	}

	// 同步到 MockQuotaStore
	ctx.MockQuotaStore.SetQuota(user.ID, quotaAmount)
	ctx.MockQuotaStore.SetUsed(user.ID, 0.0)

	return user, quota, nil
}

// 执行 expireQuotasTask 并处理错误
func executeExpireQuotasTask(ctx *TestContext) error {
	if err := ctx.QuotaService.ExpireQuotas(); err != nil {
		return fmt.Errorf("expire quotas task failed: %v", err)
	}
	return nil
}

// 月度配额使用记录验证函数

// 验证单个用户的月度配额使用记录
func verifyMonthlyQuotaUsageRecord(ctx *TestContext, userID string, expectedYearMonth string, expectedUsedQuota float64) error {
	var monthlyQuotaUsage models.MonthlyQuotaUsage
	err := ctx.DB.Where("user_id = ? AND year_month = ?", userID, expectedYearMonth).First(&monthlyQuotaUsage).Error
	if err != nil {
		return fmt.Errorf("failed to find monthly quota usage record for user %s, year_month %s: %v", userID, expectedYearMonth, err)
	}

	// 验证已使用配额金额
	if math.Abs(monthlyQuotaUsage.UsedQuota-expectedUsedQuota) > 0.0001 {
		return fmt.Errorf("monthly quota usage amount mismatch for user %s: expected %f, got %f", userID, expectedUsedQuota, monthlyQuotaUsage.UsedQuota)
	}

	// 验证记录时间不为空
	if monthlyQuotaUsage.RecordTime.IsZero() {
		return fmt.Errorf("monthly quota usage record time is zero for user %s", userID)
	}

	// 验证创建时间不为空
	if monthlyQuotaUsage.CreateTime.IsZero() {
		return fmt.Errorf("monthly quota usage create time is zero for user %s", userID)
	}

	return nil
}

// 验证零配额用户被跳过（没有创建月度配额使用记录）
func verifyZeroQuotaUserSkipped(ctx *TestContext, userID string, yearMonth string) error {
	var count int64
	err := ctx.DB.Model(&models.MonthlyQuotaUsage{}).
		Where("user_id = ? AND year_month = ?", userID, yearMonth).
		Count(&count).Error
	if err != nil {
		return fmt.Errorf("failed to query monthly quota usage record count for user %s: %v", userID, err)
	}

	if count > 0 {
		return fmt.Errorf("expected no monthly quota usage record for zero quota user %s, but found %d records", userID, count)
	}

	return nil
}

// 验证多个用户的月度配额使用记录
func verifyMonthlyQuotaUsageRecords(ctx *TestContext, expectedRecords []MonthlyQuotaUsageExpectation) error {
	for _, expected := range expectedRecords {
		if expected.ExpectedUsedQuota == 0 {
			// 零配额用户，验证被跳过
			if err := verifyZeroQuotaUserSkipped(ctx, expected.UserID, expected.YearMonth); err != nil {
				return fmt.Errorf("zero quota user verification failed for user %s: %w", expected.UserID, err)
			}
		} else {
			// 非零配额用户，验证记录存在且正确
			if err := verifyMonthlyQuotaUsageRecord(ctx, expected.UserID, expected.YearMonth, expected.ExpectedUsedQuota); err != nil {
				return fmt.Errorf("monthly quota usage record verification failed for user %s: %w", expected.UserID, err)
			}
		}
	}
	return nil
}

// 扩展配额过期期望结构体，添加月度配额使用记录期望
type MonthlyQuotaUsageExpectation struct {
	UserID            string
	YearMonth         string
	ExpectedUsedQuota float64
}

// 扩展 QuotaExpiryExpectation 结构体，添加月度配额使用记录期望
type QuotaExpiryExpectationWithMonthly struct {
	QuotaExpiryExpectation
	ExpectedMonthlyQuotaUsage []MonthlyQuotaUsageExpectation
	ExpectedYearMonth         string
}

// 扩展的配额过期数据一致性验证函数，包含月度配额使用记录验证
func verifyQuotaExpiryDataConsistencyWithMonthly(ctx *TestContext, userID string, expectedData QuotaExpiryExpectationWithMonthly) error {
	// 1. 执行原有的配额过期数据一致性验证
	baseExpectation := expectedData.QuotaExpiryExpectation
	if err := verifyQuotaExpiryDataConsistency(ctx, userID, baseExpectation); err != nil {
		return fmt.Errorf("base quota expiry data consistency verification failed: %w", err)
	}

	// 2. 验证月度配额使用记录
	if len(expectedData.ExpectedMonthlyQuotaUsage) > 0 {
		// 如果提供了具体的期望记录，验证这些记录
		if err := verifyMonthlyQuotaUsageRecords(ctx, expectedData.ExpectedMonthlyQuotaUsage); err != nil {
			return fmt.Errorf("monthly quota usage records verification failed: %w", err)
		}
	} else if expectedData.ExpectedYearMonth != "" {
		// 如果没有提供具体记录，但提供了年月，验证该用户在该年月是否有记录
		var count int64
		err := ctx.DB.Model(&models.MonthlyQuotaUsage{}).
			Where("user_id = ? AND year_month = ?", userID, expectedData.ExpectedYearMonth).
			Count(&count).Error
		if err != nil {
			return fmt.Errorf("failed to query monthly quota usage record count: %w", err)
		}

		// 根据用户的有效配额总额判断是否应该有记录
		if baseExpectation.ValidQuotaAmount > 0 {
			if count == 0 {
				return fmt.Errorf("expected monthly quota usage record for user %s with year_month %s, but found none", userID, expectedData.ExpectedYearMonth)
			}
		} else {
			if count > 0 {
				return fmt.Errorf("expected no monthly quota usage record for user %s with zero quota, but found %d records", userID, count)
			}
		}
	}

	return nil
}
