# Quota Manager 新功能说明

## 概述

本次更新为 quota-manager 项目增加了以下新功能：

1. **用户 quota 到期时间管理**
2. **定时更新过期 quota**
3. **quota 转出功能**
4. **quota 转入功能**
5. **兑换码生成和验证机制**
6. **JWT Token 用户身份认证**
7. **API响应结构优化**
8. **数据库结构优化**

## 身份验证机制

### JWT Token 解析
- **移除请求体用户ID**：所有API接口不再从请求体获取用户ID
- **Token 头部解析**：从HTTP请求头中获取JWT token并解析用户信息
- **可配置Token字段**：支持自定义token字段名（默认为 `authorization`）

#### Token解析示例
```go
// 从JWT token中解析用户信息
type AuthUser struct {
    ID      string `json:"id"`      // 用户唯一ID
    Name    string `json:"name"`    // 用户姓名
    StaffID string `json:"staffID"` // 工号
    Github  string `json:"github"`  // GitHub账号
    Phone   string `json:"phone"`   // 手机号
}

// 使用标准Base64URL解码，无需验证签名
func ParseUserInfoFromToken(accessToken string) (*AuthUser, error)
```

#### 配置示例
```yaml
server:
  port: 8099
  mode: "debug"
  token_header: "authorization"  # 可自定义token字段名
```

## 新增数据表

### 1. quota 表
用于记录用户的 quota 总数，支持不同到期时间的 quota 分别记录。

| 字段名      | 字段类型           | 说明                 |
| ----------- | ------------------ | -------------------- |
| id          | SERIAL PRIMARY KEY | 自增主键             |
| user_id     | VARCHAR(255)       | 用户ID               |
| amount      | INT                | 配额数量             |
| expiry_date | TIMESTAMP NOT NULL | 配额到期时间         |
| status      | VARCHAR(20)        | 状态 (VALID/EXPIRED) |
| create_time | TIMESTAMP          | 创建时间             |
| update_time | TIMESTAMP          | 更新时间             |

### 2. quota_audit 表
用于记录每个用户每次增加（充值，官方赠送，或者别人赠送）的 quota 数量和到期时间。

| 字段名       | 字段类型           | 说明                                         |
| ------------ | ------------------ | -------------------------------------------- |
| id           | SERIAL PRIMARY KEY | 自增主键                                     |
| user_id      | VARCHAR(255)       | 用户ID                                       |
| amount       | INT                | 变动数量 (正负值)                            |
| operation    | VARCHAR(50)        | 操作类型 (RECHARGE/TRANSFER_IN/TRANSFER_OUT) |
| voucher_code | VARCHAR(1000)      | 兑换码 (当操作为转入/转出时)                 |
| related_user | VARCHAR(255)       | 关联用户ID (转出/转入对象)                   |
| expiry_date  | TIMESTAMP NOT NULL | 配额到期时间                                 |
| create_time  | TIMESTAMP          | 创建时间                                     |

### 3. voucher_redemption 表
用于跟踪已兑换的兑换码，防止重复兑换。

| 字段名      | 字段类型           | 说明     |
| ----------- | ------------------ | -------- |
| id          | SERIAL PRIMARY KEY | 自增主键 |
| voucher_code| VARCHAR(1000)      | 兑换码   |
| receiver_id | VARCHAR(255)       | 接收者ID |
| create_time | TIMESTAMP          | 创建时间 |

## 新增 API 接口

### 身份验证
所有API接口通过HTTP请求头获取JWT token，从中解析用户ID，无需在请求体中提供用户ID。

**请求头示例**：
```
Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...
```

### 1. 获取用户 quota 接口
**Endpoint**: `GET /api/v1/quota`

**Request Headers**:
```
Authorization: Bearer <jwt_token>
```

**Response**:
```json
{
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
```

### 2. quota 充值记录接口
**Endpoint**: `GET /api/v1/quota/audit?page=1&page_size=10`

**Request Headers**:
```
Authorization: Bearer <jwt_token>
```

**Response**:
```json
{
  "total": 25,
  "records": [
    {
      "amount": 100,
      "operation": "RECHARGE",
      "voucher_code": "",
      "related_user": "",
      "expiry_date": "2025-06-30T23:59:59Z",
      "create_time": "2025-05-15T10:00:00Z"
    },
    {
      "amount": -50,
      "operation": "TRANSFER_OUT",
      "voucher_code": "xxxxx",
      "related_user": "user456",
      "expiry_date": "2025-07-31T23:59:59Z",
      "create_time": "2025-05-10T14:30:00Z"
    }
  ]
}
```

### 3. quota 转出接口
**Endpoint**: `POST /api/v1/quota/transfer-out`

**Request Headers**:
```
Authorization: Bearer <jwt_token>
```

**Request Body**:
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

**Response**:
```json
{
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
```

### 4. quota 转入接口
**Endpoint**: `POST /api/v1/quota/transfer-in`

**Request Headers**:
```
Authorization: Bearer <jwt_token>
```

**Request Body**:
```json
{
  "voucher_code": "eyJnaXZlcl9pZCI6InVzZXIxMjMiLC..."
}
```

**Response**:
```json
{
  "giver_id": "user123",
  "giver_name": "张三",
  "giver_phone": "13800138000",
  "giver_github": "zhangsan",
  "receiver_id": "user456",
  "quota_list": [
    {
      "amount": 10,
      "expiry_date": "2025-06-30T23:59:59Z",
      "is_expired": false
    },
    {
      "amount": 20,
      "expiry_date": "2025-07-31T23:59:59Z",
      "is_expired": false
    }
  ],
  "voucher_code": "eyJnaXZlcl9pZCI6InVzZXIxMjMiLC...",
  "operation": "TRANSFER_IN",
  "amount": 30
}
```

## API 响应结构优化

### 结构化数据返回
- **移除描述字符串**：转账接口不再返回 `description` 字段
- **返回关键信息**：提供结构化数据供前端自行生成描述
- **统一响应格式**：所有接口返回一致的数据结构

### 关键信息字段
转账操作返回以下关键信息：
- `related_user`: 关联用户ID
- `operation`: 操作类型（TRANSFER_OUT/TRANSFER_IN）
- `quota_list`: 分离的配额列表，包含数量和到期时间
- `voucher_code`: 兑换码

## 兑换码机制

### 生成步骤
1. 包含字段：giver_id, giver_name, giver_phone, giver_github, receiver_id, quota_list, timestamp
2. 序列化为 JSON 字符串
3. 使用 HMAC-SHA256 和密钥生成签名
4. 将序列化字符串和签名拼接，进行 Base64URL 编码

### 验证步骤
1. Base64URL 解码
2. 按分隔符分割得到 JSON 字符串和签名
3. 使用相同密钥计算 HMAC-SHA256 签名并比较
4. 反序列化 JSON 字符串为结构体

## 数据库结构优化

### 全新项目设计原则
作为全新项目，采用最优的数据结构设计：

1. **强制时间约束**：所有时间字段使用 `NOT NULL` 约束
2. **类型安全**：使用直接类型而非指针类型
3. **数据完整性**：数据库层面保证数据完整性

### 主要优化
- `QuotaExecute.ExpiryDate`: `time.Time` + `NOT NULL`
- `QuotaAudit.ExpiryDate`: `time.Time` + `NOT NULL`
- `VoucherCode` 字段长度扩展至1000字符支持长兑换码

### 代码简化
- 移除指针传递复杂性
- 统一时间字段类型处理
- 减少内存分配和GC压力

## 定时任务

### 1. 策略扫描任务
- 频率：每小时执行一次
- 功能：扫描并执行充值策略

### 2. quota 过期任务
- 频率：每月1日00:01执行
- 功能：
  - 标记过期的 quota 为无效状态
  - 同步 AiGateway 中的 quota 数据
  - 调整用户的总 quota 和已使用 quota

## 配置更新

在 `config.yaml` 中新增配置：

```yaml
server:
  port: 8099
  mode: "debug"
  token_header: "authorization"  # 可自定义token字段名

voucher:
  signing_key: "your-secret-signing-key-at-least-32-bytes-long-for-security"
```

## 测试用例

新增了以下测试用例：

1. **兑换码生成和验证测试** - 测试兑换码的生成、验证和解码功能
2. **quota 转出测试** - 测试 quota 转出功能，包括数据库更新和审计记录
3. **quota 转入测试** - 测试 quota 转入功能，包括重复兑换防护
4. **quota 过期测试** - 测试 quota 过期处理逻辑
5. **quota 审计记录测试** - 测试审计记录的查询功能
6. **策略执行到期时间测试** - 测试策略执行时正确设置到期时间

## 运行测试

```bash
# 运行所有测试
cd test
go run main.go

# 启动 AiGateway 模拟服务
cd scripts/aigateway-mock
go run main.go
```

## 技术特性

### 1. JWT Token 认证
- 无签名验证的JWT解析
- 支持Bearer token格式
- 可配置token头部字段名

### 2. 类型安全设计
- 编译时类型检查
- 避免空指针错误
- 统一的时间类型处理

### 3. 性能优化
- 减少内存分配
- 降低GC压力
- 缓存友好的数据结构

### 4. 安全机制
- HMAC-SHA256签名验证
- 防重复兑换保护
- 密钥配置管理

## 注意事项

1. **JWT Token**: 确保token包含完整的用户信息（id, name, staffID, github, phone）
2. **兑换码密钥**: 签名密钥应该足够长（至少32字节）并保密
3. **数据库**: 所有时间字段均为必填，确保数据完整性
4. **API调用**: 所有接口都需要在请求头中提供有效的JWT token
5. **前端集成**: 前端需要根据返回的结构化数据生成描述信息
6. **用户信息**: 转账操作中的给出者信息完全从JWT token中获取，无需客户端提供
7. **配额分离**: 转出响应中不同有效期的配额会分别显示，便于前端处理