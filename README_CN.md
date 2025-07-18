# Quota Manager - 配额管理系统

使用 Go 和 Gin 框架构建的综合配额管理系统，具有先进的配额分配、转账功能和基于 JWT 的身份验证。

## 功能特性

### 核心功能
- **策略管理**：一次性和周期性充值策略类型
- **策略状态控制**：实时启用/禁用策略控制
- **复杂条件匹配**：高级函数条件表达式
- **定时任务**：基于 Cron 的策略执行
- **配额过期管理**：基于时间的配额管理，自动过期处理
- **JWT 认证**：基于令牌的用户认证和授权

### 模型权限管理（新增）
- **员工同步**：自动化 HR 系统集成，同步员工和部门数据
- **权限管理**：用户和部门的细粒度模型访问控制
- **部门层级**：支持复杂组织结构的权限继承
- **实时更新**：与 AI Gateway 的自动权限同步
- **审计跟踪**：全面的权限操作跟踪

### Star 检查权限管理（新增）
- **Star 检查控制**：用户和部门级别的 GitHub Star 检查开关管理
- **细粒度控制**：支持针对特定用户或部门禁用/启用 Star 检查
- **权限继承**：部门层级的 Star 检查设置继承
- **优先级管理**：用户设置优先于部门设置
- **统一接口**：与模型权限管理共享统一的查询和同步接口

### 高级配额操作
- **配额转账**：使用兑换码在用户间安全转账配额
- **审计跟踪**：全面的配额操作跟踪
- **多种过期日期**：支持不同过期时间的配额
- **实时同步**：与 AiGateway 服务集成

### 安全性和可靠性
- **HMAC-SHA256 兑换码安全**：加密签名的兑换码
- **重复防护**：防止重复兑换兑换码
- **事务安全**：支持复杂操作的数据库事务

## 架构

### 项目结构

```
quota-manager/
├── cmd/                    # 应用程序入口点
│   └── main.go
├── internal/               # 内部包
│   ├── config/            # 配置管理
│   ├── database/          # 数据库连接
│   ├── models/            # 数据模型
│   ├── services/          # 业务逻辑
│   ├── handlers/          # HTTP 处理器
│   └── condition/         # 条件表达式解析
├── pkg/                   # 公共包
│   ├── aigateway/         # AiGateway 客户端
│   └── logger/            # 日志记录
├── scripts/               # 脚本文件
├── test/                  # 集成测试
├── config.yaml            # 配置文件
└── README.md              # 项目文档
```

### 数据库模式

#### 核心表

**策略表 (quota_strategy)**
- `id`: 策略 ID
- `name`: 策略名称（唯一）
- `title`: 策略标题
- `type`: 策略类型（periodic/single）
- `amount`: 充值金额
- `model`: 模型名称（可选）
- `periodic_expr`: 周期性策略的 Cron 表达式
- `condition`: 条件表达式
- `status`: 策略状态（布尔值：true=启用，false=禁用）
- `create_time`: 创建时间
- `update_time`: 更新时间

**配额表 (quota)**
- `id`: 配额 ID
- `user_id`: 用户 ID
- `amount`: 配额数量
- `expiry_date`: 配额过期时间（NOT NULL）
- `status`: 状态（VALID/EXPIRED）
- `create_time`: 创建时间
- `update_time`: 更新时间

**配额审计表 (quota_audit)**
- `id`: 审计 ID
- `user_id`: 用户 ID
- `amount`: 数量变化（正数/负数）
- `operation`: 操作类型（RECHARGE/TRANSFER_IN/TRANSFER_OUT）
- `voucher_code`: 兑换码（用于转账）
- `related_user`: 相关用户 ID
- `strategy_name`: 策略名称（用于充值操作）
- `expiry_date`: 配额过期时间（NOT NULL）
- `details`: 复杂操作的 JSON 详细信息
- `create_time`: 创建时间

**兑换码兑换表 (voucher_redemption)**
- `id`: 兑换 ID
- `voucher_code`: 兑换码（唯一）
- `receiver_id`: 接收方用户 ID
- `create_time`: 创建时间

#### 支持表

**执行状态表 (quota_execute)**
- `id`: 执行 ID
- `strategy_id`: 策略 ID
- `user_id`: 用户 ID
- `batch_number`: 批次号
- `status`: 执行状态
- `expiry_date`: 配额过期时间（NOT NULL）
- `create_time`: 创建时间
- `update_time`: 更新时间

**用户信息表 (auth_users)**
- `id`: 用户 ID（UUID）
- `created_at`: 创建时间
- `updated_at`: 更新时间
- `access_time`: 最后访问时间
- `name`: 用户名
- `github_id`: GitHub ID
- `github_name`: GitHub 用户名
- `email`: 邮箱
- `phone`: 电话号码
- `github_star`: GitHub 星标项目列表（逗号分隔）
- `vip`: VIP 等级
- `company`: 公司
- `location`: 位置
- `user_code`: 用户代码
- `external_accounts`: 外部账户
- `employee_number`: 员工编号
- `password`: 密码
- `devices`: 设备（JSON）

#### 权限管理表（新增）

**员工部门表 (employee_department)**
- `id`: 记录 ID
- `employee_number`: 员工编号（唯一）
- `username`: 员工用户名
- `dept_full_level_names`: 部门层级路径（数组）
- `create_time`: 创建时间
- `update_time`: 更新时间

**模型白名单表 (model_whitelist)**
- `id`: 白名单 ID
- `target_type`: 目标类型（'user' 或 'department'）
- `target_identifier`: 用户的员工编号，部门的部门名称
- `allowed_models`: 允许的模型列表（数组）
- `create_time`: 创建时间
- `update_time`: 更新时间

**有效权限表 (effective_permissions)**
- `id`: 权限 ID
- `employee_number`: 员工编号（唯一）
- `effective_models`: 当前有效的模型列表（数组）
- `whitelist_id`: 源白名单条目的引用
- `create_time`: 创建时间
- `update_time`: 更新时间

**权限审计表 (permission_audit)**
- `id`: 审计 ID
- `operation`: 操作类型（'employee_sync', 'whitelist_set', 'permission_updated', 'star_check_set', 'star_check_setting_update'）
- `target_type`: 目标类型（'user' 或 'department'）
- `target_identifier`: 目标标识符
- `details`: 操作详细信息（JSON）
- `create_time`: 创建时间

**Star 检查设置表 (star_check_settings)**
- `id`: 设置 ID
- `target_type`: 目标类型（'user' 或 'department'）
- `target_identifier`: 用户的员工编号，部门的部门名称
- `enabled`: Star 检查是否启用（布尔值）
- `create_time`: 创建时间
- `update_time`: 更新时间

**有效 Star 检查设置表 (effective_star_check_settings)**
- `id`: 设置 ID
- `employee_number`: 员工编号（唯一）
- `enabled`: 当前有效的 Star 检查设置（布尔值）
- `setting_id`: 源设置条目的引用
- `create_time`: 创建时间
- `update_time`: 更新时间

## 认证系统

### JWT 令牌认证
所有 API 端点都需要通过 HTTP 头进行 JWT 令牌认证：

```bash
Authorization: Bearer <jwt_token>
```

系统从 JWT 令牌中提取用户信息，无需签名验证，支持：
- 用户 ID (`id`)
- 姓名 (`name`)
- 员工 ID (`staffID`)
- GitHub 用户名 (`github`)
- 电话号码 (`phone`)

### 配置
```yaml
server:
  port: 8099
  mode: "release"  # gin 模式：debug, release, test
  token_header: "authorization"
  timezone: "Asia/Shanghai"  # 时区设置，默认为北京时间 (UTC+8)

# 员工同步配置（新增）
employee_sync:
  enabled: true
  hr_url: "http://hr-system/api/employees"
  hr_key: "your-hr-api-key"
  dept_url: "http://hr-system/api/departments"
  dept_key: "your-dept-api-key"
```

**员工同步配置：**
- `enabled`: 启用/禁用员工同步
- `hr_url`: 员工数据的 HR 系统 API 端点
- `hr_key`: HR 员工 API 的认证密钥
- `dept_url`: 部门数据的 HR 系统 API 端点
- `dept_key`: HR 部门 API 的认证密钥
- 同步每天凌晨 1:00 自动运行
- 可通过 API 端点手动触发同步

**时区配置：**
- `timezone`: 应用程序时区，支持 IANA 时区名称
- 常用时区：
  - `Asia/Shanghai`: 北京时间 (UTC+8)
  - `UTC`: 协调世界时
  - `America/New_York`: 纽约时间
  - `Europe/London`: 伦敦时间
- 如果不设置，默认使用 `Asia/Shanghai`
- 修改时区后需要重启应用生效

## API 文档

### 响应格式

所有 API 端点都返回以下标准格式的响应：

```json
{
  "code": "quota-manager.success",
  "message": "操作成功",
  "success": true,
  "data": { ... }
}
```

**响应字段：**
- `code`: 状态码（具有有意义标识符的字符串格式）
- `message`: 响应消息
- `success`: 操作成功状态（布尔值）
- `data`: 响应数据（可选，为空时省略）

### 认证
所有端点都需要在请求头中包含 JWT 令牌：
```
Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...
```

### 策略管理

#### 创建策略
- **POST** `/quota-manager/api/v1/strategies`
- **请求体**:
```json
{
  "name": "test-strategy",
  "title": "测试策略",
  "type": "single",
  "amount": 10,
  "model": "gpt-3.5-turbo",
  "condition": "github-star(\"zgsm\")",
  "status": true
}
```
- **响应**:
```json
{
  "code": "quota-manager.success",
  "message": "策略创建成功",
  "success": true,
  "data": {
    "id": 1,
    "name": "test-strategy",
    "title": "测试策略",
    "type": "single",
    "amount": 10,
    "model": "gpt-3.5-turbo",
    "condition": "github-star(\"zgsm\")",
    "status": true,
    "create_time": "2025-01-15T10:00:00Z",
    "update_time": "2025-01-15T10:00:00Z"
  }
}
```

### 健康检查

#### 健康检查
- **GET** `/quota-manager/health`
- **响应**:
```json
{
  "code": "quota-manager.success",
  "message": "Service is running",
  "success": true,
  "data": {
    "status": "ok"
  }
}
```

### 模型权限管理 API（新增）

#### 设置用户白名单
- **POST** `/quota-manager/api/v1/model-permissions/user`
- **请求体**:
```json
{
  "employee_number": "85054712",
  "models": ["gpt-4", "claude-3-opus"]
}
```

#### 设置部门白名单
- **POST** `/quota-manager/api/v1/model-permissions/department`
- **请求体**:
```json
{
  "department_name": "研发中心",
  "models": ["gpt-4", "deepseek-v3"]
}
```

**模型权限优先级（从高到低）：**
1. 用户特定白名单
2. 最具体的部门白名单（子部门 > 父部门）
3. 无权限（空列表）

### Star 检查权限管理 API（新增）

#### 设置用户 Star 检查开关
- **POST** `/quota-manager/api/v1/star-check-permissions/user`
- **请求体**:
```json
{
  "employee_number": "85054712",
  "enabled": true
}
```

#### 设置部门 Star 检查开关
- **POST** `/quota-manager/api/v1/star-check-permissions/department`
- **请求体**:
```json
{
  "department_name": "研发中心",
  "enabled": false
}
```

**Star 检查权限优先级（从高到低）：**
1. 用户特定设置
2. 最具体的部门设置（子部门 > 父部门）
3. 默认设置（启用）

### 配额检查权限管理 API（新增）

#### 设置用户配额检查开关
- **POST** `/quota-manager/api/v1/quota-check-permissions/user`
- **请求体**:
```json
{
  "employee_number": "85054712",
  "enabled": true
}
```

#### 设置部门配额检查开关
- **POST** `/quota-manager/api/v1/quota-check-permissions/department`
- **请求体**:
```json
{
  "department_name": "研发中心",
  "enabled": false
}
```

**配额检查权限优先级（从高到低）：**
1. 用户特定设置
2. 最具体的部门设置（子部门 > 父部门）
3. 默认设置（禁用）

### 统一权限查询和同步 API（新增）

#### 获取有效权限
- **GET** `/quota-manager/api/v1/effective-permissions?type=model&target_type=user&target_identifier=85054712`
- **GET** `/quota-manager/api/v1/effective-permissions?type=star-check&target_type=user&target_identifier=85054712`
- **GET** `/quota-manager/api/v1/effective-permissions?type=quota-check&target_type=user&target_identifier=85054712`
- **GET** `/quota-manager/api/v1/effective-permissions?type=model&target_type=department&target_identifier=研发中心`
- **GET** `/quota-manager/api/v1/effective-permissions?type=star-check&target_type=department&target_identifier=研发中心`
- **GET** `/quota-manager/api/v1/effective-permissions?type=quota-check&target_type=department&target_identifier=研发中心`

**查询参数说明：**
- `type`: 权限类型，`model` (模型权限)、`star-check` (Star 检查权限) 或 `quota-check` (配额检查权限)
- `target_type`: 目标类型，`user` 或 `department`
- `target_identifier`: 目标标识符（用户的员工编号或部门名称）

#### 触发员工同步
- **POST** `/quota-manager/api/v1/employee-sync`

此接口将同步员工数据并更新模型权限、Star 检查权限和配额检查权限。

#### 获取策略列表
- **GET** `/quota-manager/api/v1/strategies`
- **查询参数**:
  - `status=enabled|disabled|true|false` - 按状态过滤
- **响应**:
```json
{
  "code": "quota-manager.success",
  "message": "策略获取成功",
  "success": true,
  "data": {
    "strategies": [...],
    "total": 5
  }
}
```

#### 获取单个策略
- **GET** `/quota-manager/api/v1/strategies/:id`
- **响应**:
```json
{
  "code": "quota-manager.success",
  "message": "策略获取成功",
  "success": true,
  "data": {
    "id": 1,
    "name": "test-strategy",
    "title": "测试策略",
    "type": "single",
    "amount": 10,
    "model": "gpt-3.5-turbo",
    "condition": "github-star(\"zgsm\")",
    "status": true,
    "create_time": "2025-01-15T10:00:00Z",
    "update_time": "2025-01-15T10:00:00Z"
  }
}
```

#### 更新策略
- **PUT** `/quota-manager/api/v1/strategies/:id`
- **请求体**: 部分策略对象
- **响应**:
```json
{
  "code": "quota-manager.success",
  "message": "策略更新成功",
  "success": true
}
```

#### 策略状态控制
- **POST** `/quota-manager/api/v1/strategies/:id/enable` - 启用策略
- **POST** `/quota-manager/api/v1/strategies/:id/disable` - 禁用策略
- **响应**:
```json
{
  "code": "quota-manager.success",
  "message": "策略启用成功",
  "success": true
}
```

#### 删除策略
- **DELETE** `/quota-manager/api/v1/strategies/:id`
- **响应**:
```json
{
  "code": "quota-manager.success",
  "message": "策略删除成功",
  "success": true
}
```

#### 手动策略扫描
- **POST** `/quota-manager/api/v1/strategies/scan`
- **响应**:
```json
{
  "code": "quota-manager.success",
  "message": "策略扫描触发成功",
  "success": true
}
```

#### 获取策略执行记录
- **GET** `/quota-manager/api/v1/strategies/:id/executions`
- **查询参数**:
  - `page`: 页码（默认: 1）
  - `page_size`: 页面大小（默认: 10）
- **响应**:
```json
{
  "code": "quota-manager.success",
  "message": "策略执行记录获取成功",
  "success": true,
  "data": {
    "total": 15,
    "records": [
      {
        "id": 1,
        "strategy_id": 1,
        "strategy_name": "test-strategy",
        "execution_time": "2025-01-15T10:00:00Z",
        "status": "SUCCESS",
        "processed_users": 5,
        "failed_users": 0,
        "details": {...},
        "error_message": ""
      }
    ]
  }
}
```

### 配额管理

#### 获取用户配额
- **GET** `/quota-manager/api/v1/quota`
- **响应**:
```json
{
  "code": "quota-manager.success",
  "message": "用户配额获取成功",
  "success": true,
  "data": {
    "total_quota": 150,
    "used_quota": 50,
    "quota_list": [
      {
        "amount": 50,
        "expiry_date": "2025-06-30T23:59:59Z"
      },
      {
        "amount": 100,
        "expiry_date": "2025-07-31T23:59:59Z"
      }
    ]
  }
}
```

**字段描述**:
- `total_quota`: 来自 AiGateway 的总可用配额
- `used_quota`: 来自 AiGateway 的当前已使用配额
- `quota_list`: 不同过期日期的配额项数组
  - `amount`: 扣除已使用配额后的剩余配额数量
  - `expiry_date`: 配额过期时间戳

#### 获取配额审计记录
- **GET** `/quota-manager/api/v1/quota/audit?page=1&page_size=10`
- **查询参数**:
  - `page`: 页码（默认：1）
  - `page_size`: 页面大小（默认：10）
- **响应**:
```json
{
  "code": "quota-manager.success",
  "message": "配额审计记录获取成功",
  "success": true,
  "data": {
    "total": 25,
    "records": [
      {
        "amount": 100,
        "operation": "RECHARGE",
        "voucher_code": "",
        "related_user": "",
        "strategy_name": "vip-daily-bonus",
        "expiry_date": "2025-06-30T23:59:59Z",
        "details": {...},
        "create_time": "2025-05-15T10:00:00Z"
      }
    ]
  }
}
```

**字段描述**:
- `total`: 审计记录总数
- `records`: 审计记录数组
  - `amount`: 配额变化数量（正数表示增加，负数表示减少）
  - `operation`: 操作类型（RECHARGE/TRANSFER_IN/TRANSFER_OUT）
  - `voucher_code`: 转账操作的兑换码
  - `related_user`: 转账操作的相关用户 ID
  - `strategy_name`: 充值操作的策略名称
  - `expiry_date`: 配额过期时间戳
  - `details`: 详细操作信息（JSON 对象）
  - `create_time`: 操作时间戳

#### 转出配额
- **POST** `/quota-manager/api/v1/quota/transfer-out`
- **请求体**:
```json
{
  "receiver_id": "user456",
  "quota_list": [
    {
      "amount": 10,
      "expiry_date": "2025-06-30T23:59:59Z"
    },
    {
      "amount": 20,
      "expiry_date": "2025-07-31T23:59:59Z"
    }
  ]
}
```
- **响应**:
```json
{
  "code": "quota-manager.success",
  "message": "配额转出成功",
  "success": true,
  "data": {
    "voucher_code": "eyJnaXZlcl9pZCI6InVzZXIxMjMiTC...",
    "related_user": "user456",
    "operation": "TRANSFER_OUT",
    "quota_list": [
      {
        "amount": 10,
        "expiry_date": "2025-06-30T23:59:59Z"
      },
      {
        "amount": 20,
        "expiry_date": "2025-07-31T23:59:59Z"
      }
    ]
  }
}
```

**字段描述**:
- `voucher_code`: 为转账生成的兑换码
- `related_user`: 接收方用户 ID
- `operation`: 始终为 "TRANSFER_OUT"
- `quota_list`: 转账的配额项数组
  - `amount`: 配额数量
  - `expiry_date`: 配额过期时间戳

#### 转入配额
- **POST** `/quota-manager/api/v1/quota/transfer-in`
- **请求体**:
```json
{
  "voucher_code": "eyJnaXZlcl9pZCI6InVzZXIxMjMiLC..."
}
```
- **响应**:
```json
{
  "code": "quota-manager.success",
  "message": "配额转入成功",
  "success": true,
  "data": {
    "giver_id": "user123",
    "giver_name": "张三",
    "giver_phone": "13800138000",
    "giver_github": "zhangsan",
    "receiver_id": "user456",
    "quota_list": [
      {
        "amount": 10,
        "expiry_date": "2025-06-30T23:59:59Z",
        "is_expired": false,
        "success": true
      },
      {
        "amount": 20,
        "expiry_date": "2025-07-31T23:59:59Z",
        "is_expired": false,
        "success": true
      }
    ],
    "voucher_code": "eyJnaXZlcl9pZCI6InVzZXIxMjMiLC...",
    "operation": "TRANSFER_IN",
    "amount": 30,
    "status": "SUCCESS",
    "message": "所有配额转账成功完成"
  }
}
```

**字段描述**:
- `giver_id`: 转出方用户 ID
- `giver_name`: 转出方显示名称
- `giver_phone`: 转出方电话号码
- `giver_github`: 转出方 GitHub 用户名
- `giver_github_star`: 转出方的星标项目列表（逗号分隔，会传递给接收方）
- `receiver_id`: 接收方用户 ID
- `quota_list`: 转账结果数组
  - `amount`: 配额数量
  - `expiry_date`: 配额过期时间戳
  - `is_expired`: 配额是否已过期
  - `success`: 转账是否成功
  - `failure_reason`: 失败原因（如有）
- `voucher_code`: 原始兑换码
- `operation`: 始终为 "TRANSFER_IN"
- `amount`: 成功转账的总数量
- `status`: 转账状态（SUCCESS/PARTIAL_SUCCESS/FAILED/ALREADY_REDEEMED）
- `message`: 状态描述

#### 获取用户配额审计记录（管理员）
- **GET** `/quota-manager/api/v1/quota/audit/:user_id?page=1&page_size=10`
- **路径参数**:
  - `user_id`: 目标用户 ID（必需）
- **查询参数**:
  - `page`: 页码（默认: 1）
  - `page_size`: 页面大小（默认: 10）
- **响应**:
```json
{
  "code": "quota-manager.success",
  "message": "用户配额审计记录获取成功",
  "success": true,
  "data": {
    "total": 25,
    "records": [
      {
        "amount": 100,
        "operation": "RECHARGE",
        "voucher_code": "",
        "related_user": "",
        "strategy_name": "vip-daily-bonus",
        "expiry_date": "2025-06-30T23:59:59Z",
        "details": {...},
        "create_time": "2025-05-15T10:00:00Z"
      }
    ]
  }
}
```

### 健康检查
- **GET** `/quota-manager/health`
- **响应**:
```json
{
  "code": "quota-manager.success",
  "message": "服务运行中",
  "success": true,
  "data": {
    "status": "ok"
  }
}
```

### 错误响应

所有错误响应都遵循相同格式：

```json
{
  "code": "quota-manager.bad_request",
  "message": "无效的请求参数",
  "success": false
}
```

**常见错误码：**
- `quota-manager.bad_request`: 错误请求 - 无效的请求参数
- `quota-manager.unauthorized`: 未授权 - 认证失败
- `quota-manager.token_invalid`: 令牌无效 - 无效或缺少 JWT 令牌
- `quota-manager.not_found`: 未找到 - 资源不存在
- `quota-manager.strategy_not_found`: 策略未找到 - 指定 ID 的策略不存在
- `quota-manager.invalid_strategy_id`: 策略 ID 无效 - 策略 ID 格式无效
- `quota-manager.insufficient_quota`: 配额不足 - 用户配额不够
- `quota-manager.voucher_invalid`: 兑换码无效 - 兑换码无效或格式错误
- `quota-manager.voucher_expired`: 兑换码过期 - 兑换码已过期
- `quota-manager.voucher_already_redeemed`: 兑换码已使用 - 兑换码已被使用
- `quota-manager.quota_transfer_failed`: 配额转账失败 - 配额转账失败
- `quota-manager.strategy_create_failed`: 策略创建失败 - 策略创建失败
- `quota-manager.user_not_found`: 用户未找到 - 指定的用户不存在
- `quota-manager.strategy_update_failed`: 策略更新失败 - 策略更新失败
- `quota-manager.strategy_delete_failed`: 策略删除失败 - 策略删除失败
- `quota-manager.database_error`: 数据库错误 - 数据库操作失败
- `quota-manager.aigateway_error`: AiGateway 错误 - AiGateway 服务错误
- `quota-manager.internal_error`: 内部服务器错误 - 意外的服务器错误

## 条件表达式

系统支持复杂的条件表达式用于策略定位：

### 可用函数

- `access-after(timestamp)`: 指定时间后的最后访问
- `and(condition1, condition2)`: 逻辑与
- `belong-to(org1, org2)`: 属于指定组织或部门。当 `employee_sync.enabled = true` 时，通过 employee_department 表使用用户的员工编号检查用户是否属于该部门。支持中英文部门名称。当员工同步被禁用或员工编号为空时，回退到使用 Company 字段。
- `false()`: 始终返回 false（没有用户匹配）
- `github-star(project)`: 检查用户是否为指定项目加星（从用户的星标项目列表中匹配）
- `is-vip(level)`: VIP 等级大于或等于指定等级
- `match-user("user1", "user2", ...)`: 匹配当前用户ID是否在提供的ID列表中（支持多个参数）
- `not(condition)`: 逻辑非
- `or(condition1, condition2)`: 逻辑或
- `quota-le(model, amount)`: 配额余额小于或等于数量
- `register-before(timestamp)`: 指定时间前注册
- `true()`: 始终返回 true（所有用户匹配）

### 示例

```
# 始终为所有用户执行（替换空条件）
true()

# 从不为任何用户执行（用于临时禁用）
false()

# 为给 zgsm 项目加星的用户充值
github-star("zgsm")

# 为最近活跃的 VIP 用户充值
and(is-vip(1), access-after("2024-05-01 00:00:00"))

# 为早期注册用户或 VIP 用户充值
or(register-before("2023-01-01 00:00:00"), is-vip(2))

# 为特定用户ID充值
match-user("user123", "user456")

# 为特定部门的用户充值（支持中英文名称）
belong-to("技术部")       # 中文部门名称
belong-to("Tech_Group_1", "Tech_Group_2")   # 支持多个参数

# 结合部门与其他条件
and(belong-to("研发中心"), is-vip(2))

# 使用 true/false 函数的复杂条件
or(and(is-vip(3), true()), and(false(), github-star("project")))
```

## 兑换码系统

### 兑换码生成
1. 创建包含转出方信息、接收方 ID 和配额列表的兑换码数据
2. 序列化为 JSON 并添加时间戳
3. 使用密钥生成 HMAC-SHA256 签名
4. 组合 JSON 和签名，使用 Base64URL 编码

### 兑换码验证
1. Base64URL 解码
2. 分离 JSON 数据和签名
3. 验证 HMAC-SHA256 签名
4. 将 JSON 反序列化为兑换码数据
5. 检查重复兑换

### 配置
```yaml
voucher:
  signing_key: "your-secret-signing-key-at-least-32-bytes-long-for-security"
```

## 定时任务

### 策略执行任务
- **频率**: 每小时
- **功能**: 扫描并执行充值策略

### 配额过期任务
- **频率**: 每月第一天 00:01
- **功能**:
  - 将过期配额标记为无效
  - 与 AiGateway 同步配额数据
  - 调整用户总配额和已使用配额

## 快速开始

### 系统要求

- Go 1.21+
- PostgreSQL 12+

### 安装

1. **克隆仓库**
   ```bash
   git clone <repository-url>
   cd quota-manager
   ```

2. **配置数据库**
   ```yaml
   database:
     host: "localhost"
     port: 5432
     user: "postgres"
     password: "password"
     dbname: "quota_manager"
     sslmode: "disable"

   auth_database:
     host: "localhost"
     port: 5432
     user: "postgres"
     password: "password"
     dbname: "auth"
     sslmode: "disable"
   ```

   系统使用分离的数据库架构：
   - `database`: 包含配额相关表（quota_strategy, quota_execute, quota, quota_audit, voucher_redemption）
   - `auth_database`: 包含用户认证数据（auth_users 表）

3. **启动服务**
   ```bash
   # 使用启动脚本（推荐）
   chmod +x scripts/start.sh
   ./scripts/start.sh

   # 或手动启动
   go mod tidy
   psql -U postgres -f scripts/init_db.sql
   cd scripts && go run generate_data.go && cd ..
   cd scripts/aigateway-mock && go run main.go &
   cd ../../ && go run cmd/main.go
   ```

## AiGateway 集成

### 模拟服务
项目包含完整的 AiGateway 模拟服务（`scripts/aigateway-mock/`），提供：

- `POST /v1/chat/completions/quota/refresh` - 刷新配额
- `GET /v1/chat/completions/quota` - 查询配额
- `POST /v1/chat/completions/quota/delta` - 修改配额
- `GET /v1/chat/completions/quota/used` - 查询已使用配额
- `POST /v1/chat/completions/quota/used/delta` - 修改已使用配额

### 配置
```yaml
aigateway:
  host: "localhost"
  port: 1002
  admin_path: "/v1/chat/completions/quota"
  auth_header: "x-admin-key"
  auth_value: "12345678"
```

## 配置

### 完整配置文件
```yaml
server:
  port: 8099
  mode: "release"  # gin 模式：debug, release, test
  token_header: "authorization"
  timezone: "Asia/Shanghai"  # 时区设置，默认为北京时间 (UTC+8)

database:
  host: "localhost"
  port: 5432
  user: "postgres"
  password: "password"
  dbname: "quota_manager"
  sslmode: "disable"

auth_database:
  host: "localhost"
  port: 5432
  user: "postgres"
  password: "password"
  dbname: "auth"
  sslmode: "disable"

aigateway:
  host: "127.0.0.1"
  port: 8002
  admin_path: "/v1/chat/completions/quota"
  auth_header: "x-admin-key"
  auth_value: "12345678"

voucher:
  signing_key: "your-secret-signing-key-at-least-32-bytes-long-for-security"

log:
  level: "debug"
```

## 测试

### 集成测试
项目提供全面的集成测试：

```bash
# 运行集成测试
cd test
chmod +x run_tests.sh
./run_tests.sh

# 或手动运行
go run main.go
```

### 测试覆盖范围
- **条件表达式测试**: 所有支持的函数和逻辑
- **策略类型测试**: 一次性和周期性策略
- **状态控制测试**: 启用/禁用功能
- **配额转账测试**: 转出/转入操作
- **审计跟踪测试**: 记录跟踪和查询
- **过期管理测试**: 基于时间的配额处理
- **AiGateway 集成**: 正常和失败场景

### API 测试
```bash
# 测试 API 端点
chmod +x scripts/test_api.sh
./scripts/test_api.sh
```

### 测试数据生成
```bash
# 生成测试数据
cd scripts
go run generate_data.go
```

## 开发

### 添加条件函数
1. 在 `internal/condition/parser.go` 中添加表达式结构
2. 实现 `Evaluate` 方法
3. 在 `buildFunction` 方法中添加解析逻辑

### 扩展策略类型
1. 在 `ExecStrategy` 方法中添加类型处理
2. 更新数据模型和验证

### 数据库迁移
使用 GORM 自动迁移或 `scripts/init_db.sql` 中的手动 SQL 脚本

## 故障排除

### 常见问题

1. **数据库连接失败**
   - 验证 PostgreSQL 服务是否运行
   - 检查 config.yaml 中的连接参数
   - 确保数据库存在且可访问

2. **AiGateway 连接失败**
   - 验证模拟服务是否在端口 1002 上运行
   - 检查授权凭据
   - 确保没有端口冲突

3. **策略不执行**
   - 验证策略状态是否启用（`true`）
   - 检查 cron 表达式语法
   - 验证条件表达式
   - 查看日志获取详细错误信息

4. **JWT 令牌问题**
   - 确保令牌包含必需字段（id, name 等）
   - 验证配置中的令牌头名称
   - 检查令牌格式（Bearer 前缀）

### 调试模式
```bash
export GIN_MODE=debug
```

### 日志记录
系统使用带有 zap 的结构化 JSON 日志记录，提供：
- 策略执行状态
- 配额操作跟踪
- 带有堆栈跟踪的错误信息
- 性能指标

## 安全考虑

1. **兑换码安全**: 使用 HMAC-SHA256 确保兑换码完整性
2. **令牌验证**: JWT 解析无需签名验证（确保令牌源安全）
3. **数据库安全**: 使用安全的数据库凭据和 SSL 连接
4. **API 安全**: 所有端点都需要认证
5. **密钥管理**: 安全存储签名密钥并定期轮换

## 性能

### 优化
- 高效的数据库索引
- 连接池
- 大型操作的批处理
- 内存高效的数据结构

### 监控
- 结构化日志记录便于观察
- 健康检查端点
- 数据库查询优化
- 资源使用跟踪

## 许可证

MIT 许可证