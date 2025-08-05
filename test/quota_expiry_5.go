package main

import (
	"fmt"
	"quota-manager/internal/models"
	"time"
)

// 过期配额数大于已使用配额数测试
func testExpireQuotasTask_ExpiredQuotaGreaterThanUsedQuota(ctx *TestContext) TestResult {
	startTime := time.Now()

	// 步骤1：清理之前的测试数据，确保测试环境干净
	cleanupMockQuotaStore(ctx)

	// 步骤2：创建测试用户
	user := createTestUser("test_user_expired_gt_used", "Test User Expired GT Used", 0)
	if err := ctx.DB.AuthDB.Create(user).Error; err != nil {
		return TestResult{
			Passed:   false,
			Message:  fmt.Sprintf("Failed to create user: %v", err),
			Duration: time.Since(startTime),
			TestName: "过期配额数大于已使用配额数测试",
		}
	}

	// 步骤3：获取时间参考点
	// 获取上个月月底最后一天 23:59:59 作为过期时间
	lastMonthEndTime := getLastMonthEndTime()
	// 获取下个月月底最后一天 23:59:59 作为未过期配额的过期时间
	nextMonthEndTime := getMonthEndTime(time.Now().Year(), time.Now().Month()+1)

	// 步骤4：创建多个不同有效期的配额，模拟过期配额数大于已使用配额数的场景
	// 过期的配额1：金额 100.0，过期时间为上个月月底最后一天 23:59:59
	_, err := createTestQuotaWithExpiry(ctx, user.ID, 100.0, lastMonthEndTime)
	if err != nil {
		return TestResult{
			Passed:   false,
			Message:  fmt.Sprintf("Failed to create expiring quota 1: %v", err),
			Duration: time.Since(startTime),
			TestName: "过期配额数大于已使用配额数测试",
		}
	}

	// 过期的配额2：金额 80.0
	lastMonthEndTime2 := lastMonthEndTime.Add(-1 * time.Hour)
	_, err = createTestQuotaWithExpiry(ctx, user.ID, 80.0, lastMonthEndTime2)
	if err != nil {
		return TestResult{
			Passed:   false,
			Message:  fmt.Sprintf("Failed to create expiring quota 2: %v", err),
			Duration: time.Since(startTime),
			TestName: "过期配额数大于已使用配额数测试",
		}
	}

	// 未过期的配额：金额 60.0，过期时间为下个月月底
	_, err = createTestQuotaWithExpiry(ctx, user.ID, 60.0, nextMonthEndTime)
	if err != nil {
		return TestResult{
			Passed:   false,
			Message:  fmt.Sprintf("Failed to create valid quota: %v", err),
			Duration: time.Since(startTime),
			TestName: "过期配额数大于已使用配额数测试",
		}
	}

	// 步骤5：初始化 MockQuotaStore 状态
	// 设置总配额：240.0 (100.0 + 80.0 + 60.0)
	ctx.MockQuotaStore.SetQuota(user.ID, 240.0)
	// 设置已使用配额：120.0 (模拟已使用配额情况)
	ctx.MockQuotaStore.SetUsed(user.ID, 120.0)

	// 步骤6：配置 MockQuotaStore 期望
	// 只清除调用记录，保留配额数据
	ctx.MockQuotaStore.ClearDeltaCalls()
	ctx.MockQuotaStore.ClearUsedDeltaCalls()
	ctx.MockQuotaStore.ExpectExpireQuotas(user.ID)

	// 步骤7：执行配额过期任务
	if err := executeExpireQuotasTask(ctx); err != nil {
		return TestResult{
			Passed:   false,
			Message:  fmt.Sprintf("Execute expireQuotasTask for user %s failed: %v", user.ID, err),
			Duration: time.Since(startTime),
			TestName: "过期配额数大于已使用配额数测试",
		}
	}

	// 验证月度配额使用记录
	lastMonth := time.Now().AddDate(0, -1, 0)
	yearMonth := lastMonth.Format("2006-01")

	// 定义期望的月度配额使用记录
	expectedMonthlyQuotaUsage := []MonthlyQuotaUsageExpectation{
		{
			UserID:            user.ID,
			YearMonth:         yearMonth,
			ExpectedUsedQuota: 120.0, // 从 Mock AiGateway 获取的已使用配额值
		},
	}

	// 验证月度配额使用记录是否正确创建
	if err := verifyMonthlyQuotaUsageRecords(ctx, expectedMonthlyQuotaUsage); err != nil {
		return TestResult{
			Passed:   false,
			Message:  fmt.Sprintf("Monthly quota usage verification failed: %v", err),
			Duration: time.Since(startTime),
			TestName: "Mixed Status Quotas Test",
		}
	}

	// 步骤8：验证配额过期后的数据一致性
	// 定义期望的测试结果
	expectedData := QuotaExpiryExpectation{
		// 配额表数据验证
		ValidQuotaCount:    1,     // 只有未过期的配额（60.0）保持有效
		ExpiredQuotaCount:  2,     // 两个配额过期（100.0 和 80.0）
		ValidQuotaAmount:   60.0,  // 未过期配额总金额
		ExpiredQuotaAmount: 180.0, // 过期配额总金额（100.0 + 80.0）

		// MockQuotaStore 数据验证
		MockQuotaStoreTotalQuota: 60.0, // 总配额为未过期配额60.0
		MockQuotaStoreUsedQuota:  0.0,  // 已使用配额被重置为0.0

		// MockQuotaStore 调用记录验证
		ExpectedDeltaCalls: []MockQuotaStoreDeltaCall{
			{EmployeeNumber: user.ID, Delta: -180.0}, // 过期配额总金额减少 180.0
		},
		ExpectedUsedDeltaCalls: []MockQuotaStoreUsedDeltaCall{
			{EmployeeNumber: user.ID, Delta: -120.0}, // 已使用配额被重置，delta为 -120.0
		},

		// 审计记录验证
		ExpectedAuditAmount:    -180.0,             // 过期配额总金额
		AllowedAuditOperations: []string{"EXPIRE"}, // 只允许 EXPIRE 操作

		// 配额记录完整性验证
		ExpectedQuotaRecords: []QuotaRecordExpectation{
			{Amount: 80.0, ExpiryDate: lastMonthEndTime2, Status: models.StatusExpired}, // 配额1已过期
			{Amount: 100.0, ExpiryDate: lastMonthEndTime, Status: models.StatusExpired}, // 配额2已过期
			{Amount: 60.0, ExpiryDate: nextMonthEndTime, Status: models.StatusValid},    // 配额3仍有效
		},
	}

	// 执行综合数据一致性验证
	if err := verifyQuotaExpiryDataConsistency(ctx, user.ID, expectedData); err != nil {
		return TestResult{
			Passed:   false,
			Message:  fmt.Sprintf("Data consistency verification failed: %v", err),
			Duration: time.Since(startTime),
			TestName: "过期配额数大于已使用配额数测试",
		}
	}

	// 步骤10：验证 AiGateway 同步状态
	// 验证 MockQuotaStore 是否正确同步了配额状态变化
	if !ctx.MockQuotaStore.VerifyQuotaExpired(user.ID) {
		return TestResult{
			Passed:   false,
			Message:  "MockQuotaStore did not receive correct quota expiration notifications",
			Duration: time.Since(startTime),
			TestName: "过期配额数大于已使用配额数测试",
		}
	}

	return TestResult{
		Passed:   true,
		Message:  "过期配额数大于已使用配额数测试成功：过期配额（180.0）大于已使用配额（120.0），已使用配额被重置为0.0，总配额调整为未过期配额60.0",
		Duration: time.Since(startTime),
		TestName: "过期配额数大于已使用配额数测试",
	}
}

// Expired quota less than used quota test
func testExpireQuotasTask_ExpiredQuotaLessThanUsedQuota(ctx *TestContext) TestResult {
	startTime := time.Now()

	// Clean up previous test data
	cleanupMockQuotaStore(ctx)

	// Create test user
	user := createTestUser("test_user_expired_lt_used", "Test User Expired LT Used", 0)
	if err := ctx.DB.AuthDB.Create(user).Error; err != nil {
		return TestResult{
			Passed:   false,
			Message:  fmt.Sprintf("Failed to create user: %v", err),
			Duration: time.Since(startTime),
			TestName: "Expired quota less than used quota test",
		}
	}

	// 步骤3：获取时间参考点
	// 获取上个月月底最后一天 23:59:59 作为过期时间
	lastMonthEndTime := getLastMonthEndTime()
	// 获取下个月月底最后一天 23:59:59 作为未过期配额的过期时间
	nextMonthEndTime := getMonthEndTime(time.Now().Year(), time.Now().Month()+1)

	// 步骤4：创建多个不同有效期的配额，模拟过期配额数小于已使用配额数的场景
	// 过期的配额：金额 60.0，过期时间为上个月月底最后一天 23:59:59
	_, err := createTestQuotaWithExpiry(ctx, user.ID, 60.0, lastMonthEndTime)
	if err != nil {
		return TestResult{
			Passed:   false,
			Message:  fmt.Sprintf("Failed to create expiring quota: %v", err),
			Duration: time.Since(startTime),
			TestName: "Expired quota less than used quota test",
		}
	}

	// 未过期的配额1：金额 100.0，过期时间为下个月月底
	_, err = createTestQuotaWithExpiry(ctx, user.ID, 100.0, nextMonthEndTime)
	if err != nil {
		return TestResult{
			Passed:   false,
			Message:  fmt.Sprintf("Failed to create valid quota 1: %v", err),
			Duration: time.Since(startTime),
			TestName: "Expired quota less than used quota test",
		}
	}

	// 未过期的配额2：金额 80.0，过期时间为下个月月底
	nextMonthEndTime2 := nextMonthEndTime.Add(1 * time.Hour)
	_, err = createTestQuotaWithExpiry(ctx, user.ID, 80.0, nextMonthEndTime2)
	if err != nil {
		return TestResult{
			Passed:   false,
			Message:  fmt.Sprintf("Failed to create valid quota 2: %v", err),
			Duration: time.Since(startTime),
			TestName: "Expired quota less than used quota test",
		}
	}

	// 步骤5：初始化 MockQuotaStore 状态
	// 设置总配额：240.0 (60.0 + 100.0 + 80.0)
	ctx.MockQuotaStore.SetQuota(user.ID, 240.0)
	// 设置已使用配额：150.0 (已使用配额大于即将过期的配额数60.0)
	ctx.MockQuotaStore.SetUsed(user.ID, 150.0)

	// 步骤6：配置 MockQuotaStore 期望
	// 只清除调用记录，保留配额数据
	ctx.MockQuotaStore.ClearDeltaCalls()
	ctx.MockQuotaStore.ClearUsedDeltaCalls()
	ctx.MockQuotaStore.ExpectExpireQuotas(user.ID)

	// 步骤7：执行配额过期任务
	if err := executeExpireQuotasTask(ctx); err != nil {
		return TestResult{
			Passed:   false,
			Message:  fmt.Sprintf("Execute expireQuotasTask for user %s failed: %v", user.ID, err),
			Duration: time.Since(startTime),
			TestName: "Expired quota less than used quota test",
		}
	}

	// 步骤8：验证配额过期后的数据一致性
	// 定义期望的测试结果
	expectedData := QuotaExpiryExpectation{
		// 配额表数据验证
		ValidQuotaCount:    2,     // 两个未过期的配额（100.0 和 80.0）保持有效
		ExpiredQuotaCount:  1,     // 一个配额过期（60.0）
		ValidQuotaAmount:   180.0, // 未过期配额总金额（100.0 + 80.0）
		ExpiredQuotaAmount: 60.0,  // 过期配额总金额（60.0）

		// MockQuotaStore 数据验证
		MockQuotaStoreTotalQuota: 90.0, // 总配额240.0减去已使用的150.0
		MockQuotaStoreUsedQuota:  0.0,  // 已使用配额被重置

		// MockQuotaStore 调用记录验证
		ExpectedDeltaCalls: []MockQuotaStoreDeltaCall{
			{EmployeeNumber: user.ID, Delta: -150.0}, // 总配额 delta 为 -150.0（240.0 - 90.0）
		},
		ExpectedUsedDeltaCalls: []MockQuotaStoreUsedDeltaCall{
			{EmployeeNumber: user.ID, Delta: -150.0}, // 已使用配额 delta 为 -150.0（已使用配额被重置）
		},

		// 审计记录验证
		ExpectedAuditAmount:    -60.0,              // 过期配额金额
		AllowedAuditOperations: []string{"EXPIRE"}, // 只允许 EXPIRE 操作

		// 配额记录完整性验证
		ExpectedQuotaRecords: []QuotaRecordExpectation{
			{Amount: 60.0, ExpiryDate: lastMonthEndTime, Status: models.StatusExpired}, // 过期配额
			{Amount: 100.0, ExpiryDate: nextMonthEndTime, Status: models.StatusValid},  // 未过期配额1
			{Amount: 80.0, ExpiryDate: nextMonthEndTime2, Status: models.StatusValid},  // 未过期配额2
		},
	}

	// 执行综合数据一致性验证
	if err := verifyQuotaExpiryDataConsistency(ctx, user.ID, expectedData); err != nil {
		return TestResult{
			Passed:   false,
			Message:  fmt.Sprintf("Data consistency verification failed: %v", err),
			Duration: time.Since(startTime),
			TestName: "Expired quota less than used quota test",
		}
	}

	// 步骤10：验证 AiGateway 同步状态
	// 验证 MockQuotaStore 是否正确同步了配额状态变化
	if !ctx.MockQuotaStore.VerifyQuotaExpired(user.ID) {
		return TestResult{
			Passed:   false,
			Message:  "MockQuotaStore did not receive correct quota expiration notifications",
			Duration: time.Since(startTime),
			TestName: "Expired quota less than used quota test",
		}
	}

	return TestResult{
		Passed:   true,
		Message:  "Expired quota less than used quota test Succeeded: 已使用配额（150.0）大于过期配额（60.0），配额计算逻辑正确",
		Duration: time.Since(startTime),
		TestName: "Expired quota less than used quota test",
	}
}

// Mixed consumption and expiry scenarios test
func testExpireQuotasTask_MixedConsumptionAndExpiry(ctx *TestContext) TestResult {
	startTime := time.Now()

	// Clean up previous test data
	cleanupMockQuotaStore(ctx)

	// Create test user
	user := createTestUser("test_user_mixed_consumption", "Test User Mixed Consumption", 0)
	if err := ctx.DB.AuthDB.Create(user).Error; err != nil {
		return TestResult{
			Passed:   false,
			Message:  fmt.Sprintf("Failed to create user: %v", err),
			Duration: time.Since(startTime),
			TestName: "Mixed consumption and expiry scenarios test",
		}
	}

	// 获取时间参考点
	// 获取上个月月底最后一天 23:59:59 作为过期时间
	lastMonthEndTime := getLastMonthEndTime()
	// 获取下个月月底最后一天 23:59:59 作为未过期配额的过期时间
	nextMonthEndTime := getMonthEndTime(time.Now().Year(), time.Now().Month()+1)

	// 创建多个配额以模拟混合消费和过期场景
	// 配额1：高消费，低剩余量，已过期
	_, err := createTestQuotaWithExpiry(ctx, user.ID, 150.0, lastMonthEndTime) // 已过期配额
	if err != nil {
		return TestResult{
			Passed:   false,
			Message:  fmt.Sprintf("Failed to create quota 1: %v", err),
			Duration: time.Since(startTime),
			TestName: "Mixed consumption and expiry scenarios test",
		}
	}

	// 配额2：中等消费，中等剩余量，已过期
	lastMonthEndTime2 := lastMonthEndTime.Add(-1 * time.Hour)
	_, err = createTestQuotaWithExpiry(ctx, user.ID, 300.0, lastMonthEndTime2) // 已过期配额
	if err != nil {
		return TestResult{
			Passed:   false,
			Message:  fmt.Sprintf("Failed to create quota 2: %v", err),
			Duration: time.Since(startTime),
			TestName: "Mixed consumption and expiry scenarios test",
		}
	}

	// 配额3：低消费，高剩余量，未过期
	_, err = createTestQuotaWithExpiry(ctx, user.ID, 500.0, nextMonthEndTime) // 未过期配额
	if err != nil {
		return TestResult{
			Passed:   false,
			Message:  fmt.Sprintf("Failed to create quota 3: %v", err),
			Duration: time.Since(startTime),
			TestName: "Mixed consumption and expiry scenarios test",
		}
	}

	// 配额4：完全消费，未过期
	nextMonthEndTime2 := nextMonthEndTime.Add(1 * time.Hour)
	_, err = createTestQuotaWithExpiry(ctx, user.ID, 100.0, nextMonthEndTime2) // 未过期配额
	if err != nil {
		return TestResult{
			Passed:   false,
			Message:  fmt.Sprintf("Failed to create quota 4: %v", err),
			Duration: time.Since(startTime),
			TestName: "Mixed consumption and expiry scenarios test",
		}
	}

	// 配额已经在 createTestQuotaWithExpiry 函数中保存到数据库，这里不需要再次保存

	// 初始化 MockQuotaStore 状态
	// 设置总配额：1050.0 (150.0 + 300.0 + 500.0 + 100.0)
	ctx.MockQuotaStore.SetQuota(user.ID, 1050.0)
	// 设置已使用配额：610.0 (模拟已使用配额情况)
	ctx.MockQuotaStore.SetUsed(user.ID, 610.0)

	// 配置 MockQuotaStore 期望
	// 只清除调用记录，保留配额数据
	ctx.MockQuotaStore.ClearDeltaCalls()
	ctx.MockQuotaStore.ClearUsedDeltaCalls()
	ctx.MockQuotaStore.ExpectExpireQuotas(user.ID)

	// Execute expiration task
	if err := executeExpireQuotasTask(ctx); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Execute expireQuotasTask for user %s failed: %v", user.ID, err)}
	}

	// 步骤8：验证配额过期后的数据一致性
	// 定义期望的测试结果
	expectedData := QuotaExpiryExpectation{
		// 配额表数据验证
		ValidQuotaCount:    2,     // 两个未过期的配额（500.0 和 100.0）保持有效
		ExpiredQuotaCount:  2,     // 两个配额过期（150.0 和 300.0）
		ValidQuotaAmount:   600.0, // 未过期配额总金额（500.0 + 100.0）
		ExpiredQuotaAmount: 450.0, // 过期配额总金额（150.0 + 300.0）

		// MockQuotaStore 数据验证
		MockQuotaStoreTotalQuota: 440.0, // 总配额1050.0减去已使用的610.0
		MockQuotaStoreUsedQuota:  0.0,   // 已使用配额被重置

		// MockQuotaStore 调用记录验证
		ExpectedDeltaCalls: []MockQuotaStoreDeltaCall{
			{EmployeeNumber: user.ID, Delta: -610.0}, // 总配额 delta 为 -610.0（1050.0 - 440.0）
		},
		ExpectedUsedDeltaCalls: []MockQuotaStoreUsedDeltaCall{
			{EmployeeNumber: user.ID, Delta: -610.0}, // 已使用配额 delta 为 -610.0（已使用配额被重置）
		},

		// 审计记录验证
		ExpectedAuditAmount:    -450.0,             // 过期配额总金额
		AllowedAuditOperations: []string{"EXPIRE"}, // 只允许 EXPIRE 操作

		// 配额记录完整性验证
		ExpectedQuotaRecords: []QuotaRecordExpectation{
			{Amount: 300.0, ExpiryDate: lastMonthEndTime2, Status: models.StatusExpired}, // 配额1已过期
			{Amount: 150.0, ExpiryDate: lastMonthEndTime, Status: models.StatusExpired},  // 配额2已过期
			{Amount: 500.0, ExpiryDate: nextMonthEndTime, Status: models.StatusValid},    // 配额3仍有效
			{Amount: 100.0, ExpiryDate: nextMonthEndTime2, Status: models.StatusValid},   // 配额4仍有效
		},
	}

	// 执行综合数据一致性验证
	if err := verifyQuotaExpiryDataConsistency(ctx, user.ID, expectedData); err != nil {
		return TestResult{
			Passed:   false,
			Message:  fmt.Sprintf("Data consistency verification failed: %v", err),
			Duration: time.Since(startTime),
			TestName: "Mixed consumption and expiry scenarios test",
		}
	}

	// 步骤9：验证 AiGateway 同步状态
	// 验证 MockQuotaStore 是否正确同步了配额状态变化
	if !ctx.MockQuotaStore.VerifyQuotaExpired(user.ID) {
		return TestResult{
			Passed:   false,
			Message:  "MockQuotaStore did not receive correct quota expiration notifications",
			Duration: time.Since(startTime),
			TestName: "Mixed consumption and expiry scenarios test",
		}
	}

	return TestResult{
		Passed:   true,
		Message:  "Mixed consumption and expiry scenarios test Succeeded: 混合消费和过期场景测试成功，过期配额（450.0）和未过期配额（600.0）处理正确，已使用配额（610.0）计算逻辑正确",
		Duration: time.Since(startTime),
		TestName: "Mixed consumption and expiry scenarios test",
	}
}
