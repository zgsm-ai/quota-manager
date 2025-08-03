# expireQuotasTask 配额过期任务测试计划

## 问题背景

线上环境发现一个问题：用户在月底过期的配额，在触发 `expireQuotasTask` 后，数据库中的相应记录没有被正确设置为 `expired` 状态。本测试计划旨在通过详细的测试用例来检查 `expireQuotasTask` 函数的执行结果是否正确。

## 核心问题分析

通过分析 `ExpireQuotas()` 函数实现，识别出以下潜在问题：

### 时间处理问题
- 使用 `time.Now().Truncate(time.Second)` 可能导致月底边界时间处理不准确
- 数据库时间与应用时间可能存在时区差异
- 月底最后一秒的配额可能无法正确过期

### 事务回滚问题
- AiGateway 同步失败会导致整个事务回滚，所有配额状态更新都会失败
- 多用户处理时，一个用户失败会影响所有用户

### 查询条件问题
- `expiry_date < now` 的比较可能存在精度问题
- 状态更新和查询条件可能不一致

## 简化测试用例计划

根据用户反馈优化，测试用采用直接分配已过期配额的方式，过期时间统一设置为上个月月底最后一天 23:59:59，每个测试用例最后都验证用户的有效配额。

### 1. 基本功能测试用例

#### 1.1 单用户配额过期测试
**测试函数**: `testExpireQuotasTaskBasic`
**测试目标**: 验证单个用户的单个配额能够正确过期，并确保所有相关数据和 AiGateway 同步正确
**测试步骤**:
1. **创建已过期的配额记录**（状态为 VALID，过期时间为上个月月底最后一天 23:59:59，金额为 100.0）
   - **⚠️ 重要注意事项**：创建过期配额记录时，不仅需要在数据库中创建记录，还需要同步更新 AiGateway 中的配额状态。这两个操作必须保持一致性，确保测试环境的准确性。
2. 设置 AiGateway Mock 初始状态：总配额 100.0，已使用配额 30.0
3. 执行 `expireQuotasTask` 函数
4. **全面验证条件**：
   - 验证配额状态从 `VALID` 变为 `EXPIRED`
   - 验证用户的有效配额数量为 0
   - 验证用户的过期配额数量为 1
   - 验证用户的有效配额金额为 0.0
   - 验证用户的过期配额金额为 100.0
   - 验证 AiGateway 总配额同步为 0.0（因为所有配额都过期了）
   - 验证 AiGateway 已使用配额同步为 0.0（已使用配额被重置）
   - 验证 AiGateway delta 调用记录：总配额 delta 为 -100.0
   - 验证 AiGateway used delta 调用记录：已使用配额 delta 为 -30.0
   - 验证配额审计记录生成正确，金额为 -100.0，操作类型为 EXPIRE
   - 验证配额记录完整性：1条过期记录，金额 100.0，状态 EXPIRED

#### 1.2 多用户配额过期测试
**测试函数**: `testExpireQuotasTaskMultiple`
**测试目标**: 验证多个用户的多个配额能够批量过期
**测试步骤**:
1. **创建多个用户的已过期配额记录**（过期时间为上个月月底最后一天 23:59:59）
   - **⚠️ 重要注意事项**：创建过期配额记录时，不仅需要在数据库中创建记录，还需要同步更新 AiGateway 中的配额状态。这两个操作必须保持一致性，确保测试环境的准确性。
2. 执行 `expireQuotasTask` 函数
3. 验证所有配额状态都更新为 `EXPIRED`
4. 验证每个用户的有效配额都为 0
5. 验证每个配额相关的审计记录正确生成（状态从 VALID 变为 EXPIRED）

#### 1.3 无过期配额测试
**测试函数**: `testExpireQuotasTaskEmpty`
**测试目标**: 验证没有过期配额时函数正常执行
**测试步骤**:
1. 创建用户，分配未过期的配额
2. 执行 `expireQuotasTask` 函数
3. 验证没有任何配额状态被修改
4. 验证用户的有效配额保持不变
5. 验证没有生成任何配额状态变更的审计记录

### 2. 异常情况测试用例

#### 2.1 AiGateway 同步失败测试
**测试函数**: `testExpireQuotasTaskAiGatewayFail`
**测试目标**: 验证 AiGateway 同步失败时的处理，确保事务回滚且所有数据保持原状
**测试步骤**:
1. **创建已过期的配额记录**（金额 120.0，过期时间为上个月月底最后一天 23:59:59，状态 VALID）
   - **⚠️ 重要注意事项**：创建过期配额记录时，不仅需要在数据库中创建记录，还需要同步更新 AiGateway 中的配额状态。这两个操作必须保持一致性，确保测试环境的准确性。
2. 设置 AiGateway Mock 初始状态：总配额 120.0，已使用配额 40.0
3. 配置 AiGateway Mock 服务器返回错误（模拟网络故障或 AiGateway 内部错误）
4. 执行 `expireQuotasTask` 函数
5. **全面验证条件**：
   - 验证事务正确回滚，配额状态没有被更新（仍为 VALID）
   - 验证用户的有效配额数量保持为 1
   - 验证用户的过期配额数量保持为 0
   - 验证用户的有效配额金额保持为 120.0
   - 验证用户的过期配额金额保持为 0.0
   - 验证 AiGateway 数据保持不变：
     - 总配额仍为 120.0
     - 已使用配额仍为 40.0
   - 验证没有 AiGateway delta 调用（因为事务回滚）
   - 验证没有 AiGateway used delta 调用（因为事务回滚）
   - 验证由于事务回滚，没有生成任何配额状态变更的审计记录
   - 验证配额记录完整性：1条有效记录，金额 120.0，状态 VALID

#### 2.2 部分失败处理测试
**测试函数**: `testExpireQuotasTaskPartialFail`
**测试目标**: 验证多用户处理时的部分失败情况，确保事务回滚且所有数据保持原状
**测试步骤**:
1. **创建多个用户的已过期配额记录**（用户1：金额 100.0，用户2：金额 150.0，过期时间为上个月月底最后一天 23:59:59，状态 VALID）
   - **⚠️ 重要注意事项**：创建过期配额记录时，不仅需要在数据库中创建记录，还需要同步更新 AiGateway 中的配额状态。这两个操作必须保持一致性，确保测试环境的准确性。
2. 设置 AiGateway Mock 初始状态：用户1总配额 100.0已使用 30.0，用户2总配额 150.0已使用 50.0
3. 配置某些用户的 AiGateway 响应为失败（例如用户2返回错误）
4. 执行 `expireQuotasTask` 函数
5. **全面验证条件**：
   - 验证事务回滚机制，所有配额状态都没有被更新（仍为 VALID）
   - 验证所有用户的有效配额数量保持不变（用户1为1，用户2为1）
   - 验证所有用户的过期配额数量保持不变（用户1为0，用户2为0）
   - 验证所有用户的有效配额金额保持不变（用户1为100.0，用户2为150.0）
   - 验证所有用户的过期配额金额保持不变（用户1为0.0，用户2为0.0）
   - 验证 AiGateway 数据保持不变：
     - 用户1总配额仍为 100.0，已使用配额仍为 30.0
     - 用户2总配额仍为 150.0，已使用配额仍为 50.0
   - 验证没有 AiGateway delta 调用（因为部分失败导致事务回滚）
   - 验证没有 AiGateway used delta 调用（因为部分失败导致事务回滚）
   - 验证由于事务回滚，没有生成任何配额状态变更的审计记录
   - 验证配额记录完整性：用户1和用户2各有1条有效记录，金额分别为100.0和150.0，状态VALID

### 3. 数据一致性测试用例
#### 3.1 幂等性测试
**测试函数**: `testExpireQuotasTaskIdempotency`
**测试目标**: 验证重复执行的幂等性，确保多次执行结果与单次执行一致且无副作用
**测试步骤**:
1. **创建已过期的配额记录**（金额 200.0，过期时间为上个月月底最后一天 23:59:59，状态 VALID）
   - **⚠️ 重要注意事项**：创建过期配额记录时，不仅需要在数据库中创建记录，还需要同步更新 AiGateway 中的配额状态。这两个操作必须保持一致性，确保测试环境的准确性。
2. 设置 AiGateway Mock 初始状态：总配额 200.0，已使用配额 80.0
3. 多次执行 `expireQuotasTask` 函数（例如连续执行3次）
4. **全面验证条件**：
   - 验证配额状态只更新一次（从VALID变为EXPIRED）
   - 验证用户的有效配额数量为 0
   - 验证用户的过期配额数量为 1
   - 验证用户的有效配额金额为 0.0
   - 验证用户的过期配额金额为 200.0
   - 验证 AiGateway 总配额同步为 0.0（所有配额都过期了）
   - 验证 AiGateway 已使用配额同步为 0.0（已使用配额被重置）
   - 验证 AiGateway delta 调用记录只有一次：总配额 delta 为 -200.0
   - 验证 AiGateway used delta 调用记录只有一次：已使用配额 delta 为 -80.0
   - 验证配额审计记录只生成一次，金额为 -200.0，操作类型为 EXPIRE
   - 验证第二次及后续执行不会产生副作用：
     - 没有额外的 AiGateway 调用
     - 没有额外的审计记录生成
     - 配额状态不会重复更新
     - 数据库数据保持不变
   - 验证配额记录完整性：1条过期记录，金额 200.0，状态 EXPIRED

### 4. 月底特定场景测试用例

#### 4.1 月底批量过期测试
**测试函数**: `testExpireQuotasTask_MonthEndBatchExpiry`
**测试目标**: 验证月底大量配额同时过期的处理，确保批量处理正确且数据一致性
**测试步骤**:
1. **创建多个用户，配额都设置为月底同一天过期**（用户1：金额 50.0，用户2：金额 75.0，用户3：金额 100.0，过期时间为上个月月底最后一天 23:59:59，状态 VALID）
   - **⚠️ 重要注意事项**：创建过期配额记录时，不仅需要在数据库中创建记录，还需要同步更新 AiGateway 中的配额状态。这两个操作必须保持一致性，确保测试环境的准确性。
2. 设置 AiGateway Mock 初始状态：用户1总配额 50.0已使用 10.0，用户2总配额 75.0已使用 25.0，用户3总配额 100.0已使用 40.0
3. 设置当前时间为下个月第一天
4. 执行 `expireQuotasTask` 函数
5. **全面验证条件**：
   - 验证批量处理性能（执行时间在合理范围内）
   - 验证所有配额状态正确更新为 EXPIRED
   - 验证每个用户的有效配额数量为 0
   - 验证每个用户的过期配额数量为 1
   - 验证每个用户的有效配额金额为 0.0
   - 验证每个用户的过期配额金额为原配额金额（用户1为50.0，用户2为75.0，用户3为100.0）
   - 验证 AiGateway 总配额同步为 0.0（所有配额都过期了）
   - 验证 AiGateway 已使用配额同步为 0.0（已使用配额被重置）
   - 验证 AiGateway delta 调用记录：
     - 用户1总配额 delta 为 -50.0
     - 用户2总配额 delta 为 -75.0
     - 用户3总配额 delta 为 -100.0
   - 验证 AiGateway used delta 调用记录：
     - 用户1已使用配额 delta 为 -10.0
     - 用户2已使用配额 delta 为 -25.0
     - 用户3已使用配额 delta 为 -40.0
   - 验证所有过期配额的状态变更审计记录正确生成（状态从 VALID 变为 EXPIRED）
   - 验证审计记录的时间戳准确性（记录精确的过期时间）
   - 验证配额记录完整性：每个用户都有1条过期记录，金额和状态正确

#### 4.2 不同月份天数差异测试
**测试函数**: `testExpireQuotasTask_MonthDayDifferences`
**测试目标**: 验证不同月份天数差异（28/29/30/31天）对配额过期逻辑的影响，确保过期处理在各种月份边界条件下都能正确工作
**测试步骤**:
1. 针对不同月份分别创建测试场景，每个场景独立测试：

   **测试场景1：2月到3月（28/29天）**
   - 创建配额，过期时间设置为2月底最后一天 23:59:59（金额 60.0，状态 VALID）
   - 设置 AiGateway Mock 初始状态：总配额 60.0，已使用配额 20.0
   - 设置当前时间为3月1日
   - **⚠️ 重要注意事项**：创建配额记录时，不仅需要在数据库中创建记录，还需要同步更新 AiGateway 中的配额状态。
   - 执行 `expireQuotasTask` 函数
   - 验证配额状态正确更新为 EXPIRED

   **测试场景2：4月到5月（30天）**
   - 创建配额，过期时间设置为4月底最后一天 23:59:59（金额 60.0，状态 VALID）
   - 设置 AiGateway Mock 初始状态：总配额 60.0，已使用配额 20.0
   - 设置当前时间为5月1日
   - **⚠️ 重要注意事项**：创建配额记录时，不仅需要在数据库中创建记录，还需要同步更新 AiGateway 中的配额状态。
   - 执行 `expireQuotasTask` 函数
   - 验证配额状态正确更新为 EXPIRED

   **测试场景3：7月到8月（31天）**
   - 创建配额，过期时间设置为7月底最后一天 23:59:59（金额 60.0，状态 VALID）
   - 设置 AiGateway Mock 初始状态：总配额 60.0，已使用配额 20.0
   - 设置当前时间为8月1日
   - **⚠️ 重要注意事项**：创建配额记录时，不仅需要在数据库中创建记录，还需要同步更新 AiGateway 中的配额状态。
   - 执行 `expireQuotasTask` 函数
   - 验证配额状态正确更新为 EXPIRED

2. **全面验证条件**（每个测试场景都需要验证）：
   - 验证对应月份的月底过期时间计算正确性：
     - 2月：平年28天或闰年29天的最后一天
     - 4月：30天的最后一天
     - 7月：31天的最后一天
   - 验证配额状态正确更新为 EXPIRED
   - 验证用户的有效配额数量为 0
   - 验证用户的过期配额数量为 1
   - 验证用户的有效配额金额为 0.0
   - 验证用户的过期配额金额为 60.0
   - 验证 AiGateway 总配额同步为 0.0（所有配额都过期了）
   - 验证 AiGateway 已使用配额同步为 0.0（已使用配额被重置）
   - 验证 AiGateway delta 调用记录：总配额 delta 为 -60.0
   - 验证 AiGateway used delta 调用记录：已使用配额 delta 为 -20.0
   - 验证配额状态变更审计记录正确生成
   - 验证审计记录中包含正确的月份时间信息
   - 验证配额记录完整性：1条过期记录，金额 60.0，状态 EXPIRED

3. **跨场景一致性验证**：
   - 验证所有月份场景下的过期处理逻辑一致
   - 验证不同月份天数的差异不影响过期判断的准确性
   - 验证边界时间处理在各种月份条件下都正确

### 5. 业务逻辑测试用例

#### 5.1 配额类型差异测试
**测试函数**: `testExpireQuotasTask_DifferentQuotaTypes`
**测试目标**: 验证不同类型配额的过期处理，确保配额类型不影响过期逻辑的正确性
**测试步骤**:
1. **创建不同类型的配额**（充值配额：金额 60.0，赠送配额：金额 40.0，活动配额：金额 80.0，过期时间为上个月月底最后一天 23:59:59，状态 VALID）
   - **⚠️ 重要注意事项**：创建过期配额记录时，不仅需要在数据库中创建记录，还需要同步更新 AiGateway 中的配额状态。这两个操作必须保持一致性，确保测试环境的准确性。
2. 设置 AiGateway Mock 初始状态：总配额 180.0，已使用配额 70.0
3. 执行 `expireQuotasTask` 函数
4. **全面验证条件**：
   - 验证所有类型配额都能正确过期（状态更新为 EXPIRED）
   - 验证配额类型不影响过期逻辑
   - 验证用户的有效配额数量为 0
   - 验证用户的过期配额数量为 3（三种类型各一个）
   - 验证用户的有效配额金额为 0.0
   - 验证用户的过期配额金额为 180.0（三种类型总和）
   - 验证 AiGateway 总配额同步为 0.0（所有配额都过期了）
   - 验证 AiGateway 已使用配额同步为 0.0（已使用配额被重置）
   - 验证 AiGateway delta 调用记录：总配额 delta 为 -180.0
   - 验证 AiGateway used delta 调用记录：已使用配额 delta 为 -70.0
   - 验证每种配额类型的用户有效配额为 0
   - 验证所有配额类型的状态变更审计记录正确生成
   - 验证审计记录中包含正确的配额类型信息
   - 验证配额记录完整性：3条过期记录，金额分别为60.0、40.0、80.0，状态EXPIRED

#### 5.2 配额状态组合测试
**测试函数**: `testExpireQuotasTask_MixedStatusQuotas`
**测试目标**: 验证混合状态配额的处理，确保只有VALID状态且已过期的配额被处理，并正确处理AiGateway已使用配额重置逻辑
**测试步骤**:
1. **创建用户，包含不同状态的配额**：
   - VALID状态配额1：金额 70.0，过期时间为上个月月底最后一天 23:59:59（应该被处理）
   - VALID状态配额2：金额 50.0，过期时间为下个月月底（不应该被处理）
   - EXPIRED状态配额：金额 30.0（不应该被处理）
   - **⚠️ 重要概念澄清**：数据库中的配额状态只有VALID和EXPIRED两种，不存在USED状态配额记录。USED配额是AiGateway中的概念，表示用户已使用的配额数量。
   - **⚠️ 重要注意事项**：创建配额记录时，不仅需要在数据库中创建记录，还需要同步更新 AiGateway 中的配额状态。这两个操作必须保持一致性，确保测试环境的准确性。
2. 设置 AiGateway Mock 初始状态：总配额 120.0（所有VALID配额的总和：70.0+50.0），已使用配额 40.0
3. 执行 `expireQuotasTask` 函数
4. **全面验证条件**：
   - 验证只有 VALID 状态且已过期的配额被处理（状态更新为 EXPIRED）
   - 验证其他状态配额不受影响：
     - 未过期的VALID配额保持VALID状态
     - EXPIRED状态配额保持EXPIRED状态
   - 验证用户的有效配额计算正确（只有未过期的VALID配额被计入，数量为1，金额为50.0）
   - 验证用户的过期配额数量为2（原有的EXPIRED配额+新过期的配额）
   - 验证用户的过期配额金额为100.0（原有的30.0+新过期的70.0）
   - 验证 AiGateway 总配额同步为50.0（只有未过期的VALID配额）
   - 验证 AiGateway 已使用配额同步重置为0.0（根据代码实现，配额过期时会重置AiGateway中的已使用配额）
   - 验证 AiGateway delta 调用记录：总配额 delta 为 -70.0（120.0 - 50.0）
   - 验证 AiGateway used delta 调用记录：已使用配额 delta 为 -40.0（已使用配额被重置）
   - 验证只有状态从VALID变为EXPIRED的配额生成了审计记录（1条记录，金额-70.0）
   - 验证审计记录准确反映了状态变更过程（VALID→EXPIRED）
   - 验证配额记录完整性：总共3条记录（无USED状态记录），状态和金额正确

### 6. 用户配额消费与过期关系测试用例

#### 6.1 过期配额数大于已使用配额数测试
**测试函数**: `testExpireQuotasTask_ExpiredQuotaGreaterThanUsedQuota`
**测试目标**: 验证用户已过期的配额数大于已使用配额数时的处理逻辑，确保过期不影响已使用配额的计算
**测试场景**: 用户有多个配额，部分过期，且已使用了部分配额
**测试步骤**:
1. **创建用户，分配多个不同有效期的配额**：
   - 即将过期的配额1：金额 100.0，过期时间为上个月月底最后一天 23:59:59
   - 即将过期的配额2：金额 80.0，过期时间为上个月月底最后一天 23:59:59
   - 未过期的配额：金额 60.0，过期时间为下个月月底
   - **⚠️ 重要注意事项**：创建配额记录时，不仅需要在数据库中创建记录，还需要同步更新 AiGateway 中的配额状态。这两个操作必须保持一致性，确保测试环境的准确性。
2. 设置 AiGateway Mock 初始状态：总配额 240.0，已使用配额 120.0
3. 用户已经使用了部分配额（已使用120.0，小于即将过期的配额总数180.0），根据消费优先级，已使用的120.0完全由即将过期的配额覆盖，未过期配额60.0未被消耗
4. 设置当前时间为过期时间之后
5. 执行 `expireQuotasTask` 函数
6. **全面验证条件**：
   - 验证过期配额状态正确更新为 EXPIRED（配额1和配额2）
   - 验证未过期配额状态保持为 VALID
   - 验证用户的有效配额数量为1（只有未过期的配额）
   - 验证用户的过期配额数量为2（配额1和配额2）
   - 验证用户的有效配额金额为60.0（只有未过期的配额）
   - 验证用户的过期配额金额为180.0（配额1和配额2的总和）
   - 验证 AiGateway 同步的总配额数量为60.0（未过期配额60.0，因为已使用的120.0完全由即将过期的配额180.0覆盖，未过期配额未被消耗）
   - 验证 AiGateway 同步的已使用配额数量为0.0（已使用配额被重置）
   - 验证 AiGateway delta 调用记录：总配额 delta 为 -180.0（过期配额的总和）
   - 验证 AiGateway used delta 调用记录：已使用配额 delta 为 -120.0（已使用配额被重置）
   - 验证配额审计记录准确反映了过期操作（2条记录，金额分别为-100.0和-80.0）
   - 验证用户的可用配额不受错误影响（有效配额60.0）
   - 验证配额记录完整性：3条记录，状态和金额正确

#### 6.2 过期配额数小于已使用配额数测试
**测试函数**: `testExpireQuotasTask_ExpiredQuotaLessThanUsedQuota`
**测试目标**: 验证用户已过期的配额数小于已使用配额数时的处理逻辑，确保部分使用配额的过期处理正确
**测试场景**: 用户有多个配额，部分过期，且已使用了部分配额，已使用配额数量大于即将过期的配额数量
**测试步骤**:
1. **创建用户，分配多个不同有效期的配额**：
   - 即将过期的配额：金额 60.0，过期时间为上个月月底最后一天 23:59:59
   - 未过期的配额1：金额 100.0，过期时间为下个月月底
   - 未过期的配额2：金额 80.0，过期时间为下个月月底
   - **⚠️ 重要注意事项**：创建配额记录时，不仅需要在数据库中创建记录，还需要同步更新 AiGateway 中的配额状态。这两个操作必须保持一致性，确保测试环境的准确性。
2. 设置 AiGateway Mock 初始状态：总配额 240.0，已使用配额 150.0
3. 用户已经使用了部分配额（已使用150.0，大于即将过期的配额数60.0）
4. 设置当前时间为过期时间之后
5. 执行 `expireQuotasTask` 函数
6. **全面验证条件**：
   - 验证过期配额状态正确更新为 EXPIRED
   - 验证未过期配额状态保持为 VALID
   - 验证用户的有效配额数量为2（两个未过期的配额）
   - 验证用户的过期配额数量为1（已过期的配额）
   - 验证用户的有效配额金额为180.0（两个未过期配额的总和）
   - 验证用户的过期配额金额为60.0（已过期的配额）
   - 验证 AiGateway 同步的总配额数量为90.0（总配额240.0减去已使用的150.0）
   - 验证 AiGateway 同步的已使用配额数量为0.0（根据代码实现，配额过期时会重置AiGateway中的已使用配额）
   - 验证 AiGateway delta 调用记录：总配额 delta 为 -150.0（240.0 - 90.0）
   - 验证 AiGateway used delta 调用记录：已使用配额 delta 为 -150.0（已使用配额被重置）
   - 验证配额审计记录准确反映了过期操作（1条记录，金额-60.0）
   - 验证用户的可用配额计算逻辑正确（有效配额90.0，已使用配额0.0，可用配额90.0）
   - 验证配额记录完整性：3条记录，状态和金额正确

#### 6.3 混合配额消费与过期场景测试
**测试函数**: `testExpireQuotasTask_MixedConsumptionAndExpiry`
**测试目标**: 验证复杂的配额消费与过期混合场景
**测试场景**: 用户有个人配额和部门配额，部分配额过期，有复杂的消费记录
**测试步骤**:
1. **创建用户，同时拥有个人配额和部门配额权限**
2. **分配多个不同时间点的配额，设置不同的过期时间**
   - **⚠️ 重要注意事项**：创建配额记录时，不仅需要在数据库中创建记录，还需要同步更新 AiGateway 中的配额状态。这两个操作必须保持一致性，确保测试环境的准确性。
3. 模拟用户在不同时间点的配额消费记录
4. 设置当前时间为某个关键时间点（部分配额刚过期）
5. 执行 `expireQuotasTask` 函数
6. 验证过期配额状态更新正确
7. 验证用户的总可用配额计算正确
8. 验证配额消费顺序逻辑正确
9. 验证部门配额和个人配额的过期处理独立正确
10. 验证 AiGateway 同步的配额数量准确
11. 验证所有相关的审计记录完整准确

#### 6.4 边界值配额消费测试
**测试函数**: `testExpireQuotasTask_BoundaryConsumptionScenarios`
**测试目标**: 验证配额消费与过期的边界值场景
**测试场景**: 测试配额消费数量与过期数量的各种边界组合
**测试步骤**:
1. **创建用户，设置配额消费数量等于过期配额数量的场景**
2. **创建配额消费数量略大于过期配额数量的场景**
3. **创建配额消费数量略小于过期配额数量的场景**
4. **创建配额消费数量为0的场景**（只有过期，无消费）
5. **创建过期配额数量为0的场景**（只有消费，无过期）
   - **⚠️ 重要注意事项**：创建配额记录时，不仅需要在数据库中创建记录，还需要同步更新 AiGateway 中的配额状态。这两个操作必须保持一致性，确保测试环境的准确性。
6. 分别执行 `expireQuotasTask` 函数
7. 验证每种边界情况下的配额计算逻辑正确
8. 验证 AiGateway 同步的准确性
9. 验证没有出现负数配额或异常数值
10. 验证配额状态转换的原子性

## 辅助函数设计

### 1. 统一过期时间设置函数
```go
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

// 创建已过期的测试配额数据
func createExpiredTestQuota(ctx *TestContext, userID, quotaID string) (*Quota, error) {
    expiryTime := getLastMonthEndTime()
    return createTestQuota(ctx, userID, quotaID, "VALID", expiryTime)
}

// 创建未过期的测试配额数据
func createValidTestQuota(ctx *TestContext, userID, quotaID string) (*Quota, error) {
    expiryTime := time.Now().AddDate(0, 1, 0) // 一个月后过期
    return createTestQuota(ctx, userID, quotaID, "VALID", expiryTime)
}

// 创建测试配额数据（通用函数）
func createTestQuota(ctx *TestContext, userID, quotaID string, status string, expiryTime time.Time) (*Quota, error) {
    quota := &Quota{
        ID:         quotaID,
        UserID:     userID,
        Status:     status,
        ExpiryDate: expiryTime,
        // 其他必要字段...
    }
    err := ctx.DB.Create(quota).Error
    if err != nil {
        return nil, err
    }
    return quota, nil
}
```

### 2. 全面数据验证辅助函数

#### 2.1 配额表数据验证
```go
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
        if quota.ExpiryDate.Unix() != expected.ExpiryDate.Unix() {
            return fmt.Errorf("quota record %d: expected expiry time %v, got %v", i, expected.ExpiryDate, quota.ExpiryDate)
        }
    }
    return nil
}

type QuotaRecordExpectation struct {
    Amount     float64
    ExpiryDate time.Time
    Status     string
}
```

#### 2.2 AiGateway 数据验证
```go
// 验证 AiGateway 总配额同步
func verifyAiGatewayTotalQuota(ctx *TestContext, userID string, expectedTotalQuota float64) error {
    // 从 Mock AiGateway 获取总配额
    actualTotalQuota := ctx.AiGatewayMock.GetTotalQuota(userID)
    if math.Abs(actualTotalQuota-expectedTotalQuota) > 0.0001 {
        return fmt.Errorf("AiGateway total quota mismatch: expected %f, got %f", expectedTotalQuota, actualTotalQuota)
    }
    return nil
}

// 验证 AiGateway 已使用配额同步
func verifyAiGatewayUsedQuota(ctx *TestContext, userID string, expectedUsedQuota float64) error {
    // 从 Mock AiGateway 获取已使用配额
    actualUsedQuota := ctx.AiGatewayMock.GetUsedQuota(userID)
    if math.Abs(actualUsedQuota-expectedUsedQuota) > 0.0001 {
        return fmt.Errorf("AiGateway used quota mismatch: expected %f, got %f", expectedUsedQuota, actualUsedQuota)
    }
    return nil
}

// 验证 AiGateway delta 调用记录
func verifyAiGatewayDeltaCalls(ctx *TestContext, expectedCalls []AiGatewayDeltaCall) error {
    actualCalls := ctx.AiGatewayMock.GetDeltaCalls()

    if len(actualCalls) != len(expectedCalls) {
        return fmt.Errorf("expected %d AiGateway delta calls, got %d", len(expectedCalls), len(actualCalls))
    }

    for i, expected := range expectedCalls {
        actual := actualCalls[i]
        if actual.UserID != expected.UserID {
            return fmt.Errorf("delta call %d: expected user_id %s, got %s", i, expected.UserID, actual.UserID)
        }
        if math.Abs(actual.Delta-expected.Delta) > 0.0001 {
            return fmt.Errorf("delta call %d: expected delta %f, got %f", i, expected.Delta, actual.Delta)
        }
        if actual.Type != expected.Type {
            return fmt.Errorf("delta call %d: expected type %s, got %s", i, expected.Type, actual.Type)
        }
    }
    return nil
}

// 验证 AiGateway used delta 调用记录
func verifyAiGatewayUsedDeltaCalls(ctx *TestContext, expectedCalls []AiGatewayUsedDeltaCall) error {
    actualCalls := ctx.AiGatewayMock.GetUsedDeltaCalls()

    if len(actualCalls) != len(expectedCalls) {
        return fmt.Errorf("expected %d AiGateway used delta calls, got %d", len(expectedCalls), len(actualCalls))
    }

    for i, expected := range expectedCalls {
        actual := actualCalls[i]
        if actual.UserID != expected.UserID {
            return fmt.Errorf("used delta call %d: expected user_id %s, got %s", i, expected.UserID, actual.UserID)
        }
        if math.Abs(actual.Delta-expected.Delta) > 0.0001 {
            return fmt.Errorf("used delta call %d: expected delta %f, got %f", i, expected.Delta, actual.Delta)
        }
    }
    return nil
}

type AiGatewayDeltaCall struct {
    UserID string
    Delta  float64
    Type   string // "total_quota"
}

type AiGatewayUsedDeltaCall struct {
    UserID string
    Delta  float64
}
```

#### 2.3 审计记录验证
```go
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
```

#### 2.4 综合数据一致性验证
```go
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

    // 2. 验证 AiGateway 数据
    if err := verifyAiGatewayTotalQuota(ctx, userID, expectedData.AiGatewayTotalQuota); err != nil {
        return fmt.Errorf("AiGateway total quota verification failed: %w", err)
    }

    if err := verifyAiGatewayUsedQuota(ctx, userID, expectedData.AiGatewayUsedQuota); err != nil {
        return fmt.Errorf("AiGateway used quota verification failed: %w", err)
    }

    // 3. 验证 AiGateway 调用记录
    if err := verifyAiGatewayDeltaCalls(ctx, expectedData.ExpectedDeltaCalls); err != nil {
        return fmt.Errorf("AiGateway delta calls verification failed: %w", err)
    }

    if err := verifyAiGatewayUsedDeltaCalls(ctx, expectedData.ExpectedUsedDeltaCalls); err != nil {
        return fmt.Errorf("AiGateway used delta calls verification failed: %w", err)
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
    ValidQuotaCount         int
    ExpiredQuotaCount       int
    ValidQuotaAmount        float64
    ExpiredQuotaAmount      float64
    AiGatewayTotalQuota     float64
    AiGatewayUsedQuota      float64
    ExpectedDeltaCalls      []AiGatewayDeltaCall
    ExpectedUsedDeltaCalls  []AiGatewayUsedDeltaCall
    ExpectedAuditAmount     float64
    AllowedAuditOperations  []string
    ExpectedQuotaRecords    []QuotaRecordExpectation
}
```

### 3. AiGateway Mock 辅助函数
```go
// 配置 AiGateway Mock 服务器响应
func setupAiGatewayMock(ctx *TestContext, shouldFail bool) {
    if shouldFail {
        ctx.AiGatewayMock.On("SyncQuota", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("int64")).
            Return(fmt.Errorf("AiGateway sync failed"))
    } else {
        ctx.AiGatewayMock.On("SyncQuota", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("int64")).
            Return(nil)
    }
}

// 验证 AiGateway 调用
func verifyAiGatewayCalls(ctx *TestContext, expectedCallCount int) error {
    if ctx.AiGatewayMock.CallCount != expectedCallCount {
        return fmt.Errorf("expected %d AiGateway calls, got %d", expectedCallCount, ctx.AiGatewayMock.CallCount)
    }
    return nil
}
```

### 4. 审计记录验证函数
```go
// 验证配额过期审计记录
func verifyQuotaExpiryAudit(ctx *TestContext, quotaID string, expectedOldStatus, expectedNewStatus string) error {
    var auditRecord AuditRecord
    err := ctx.DB.Where("quota_id = ? AND operation_type = ?", quotaID, "STATUS_CHANGE").
        Order("created_at DESC").First(&auditRecord).Error
    if err != nil {
        return err
    }

    if auditRecord.OldValue != expectedOldStatus || auditRecord.NewValue != expectedNewStatus {
        return fmt.Errorf("audit record mismatch: expected %s->%s, got %s->%s",
            expectedOldStatus, expectedNewStatus, auditRecord.OldValue, auditRecord.NewValue)
    }

    return nil
}
```

## 测试实现细节

### 测试文件结构
- **依赖**: 使用现有的 `TestContext` 结构
- **Mock**: 扩展 `MockQuotaStore` 功能

### 时间控制机制
- 创建时间模拟函数来控制 `time.Now()`
- 支持设置特定时间点进行测试
- 支持时区设置测试

### 数据验证机制
- 数据库状态验证函数
- AiGateway 调用验证函数
- 审计记录验证函数

### 测试清理机制
- 每个测试执行前的数据清理
- 测试执行后的状态恢复
- Mock 状态重置

## 测试集成

### 集成到主测试套件
在 `main.go` 的测试用例列表中添加前面提到的各种测试用例

### 测试依赖管理
- 确保测试执行顺序的正确性
- 管理测试间的数据依赖
- 处理测试并发执行的问题

## 测试环境要求

### 数据库要求
- 测试数据库与生产环境结构一致
- 预置测试数据
- 支持事务回滚

### AiGateway Mock要求
- 模拟正常和异常响应
- 记录调用历史
- 支持延迟和失败注入
