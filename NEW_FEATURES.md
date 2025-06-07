# Quota Manager 新功能说明

## 概述

本次更新为 quota-manager 项目增加了以下新功能：

1. **用户 quota 到期时间管理**
2. **定时更新过期 quota**
3. **quota 转出功能**
4. **quota 转入功能**
5. **兑换码生成和验证机制**

## 新增数据表

### 1. quota 表
用于记录用户的 quota 总数，支持不同到期时间的 quota 分别记录。

| 字段名      | 字段类型           | 说明                 |
| ----------- | ------------------ | -------------------- |
| id          | SERIAL PRIMARY KEY | 自增主键             |
| user_id     | VARCHAR(255)       | 用户ID               |
| amount      | INT                | 配额数量             |
| expiry_date | TIMESTAMP          | 配额到期时间         |
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
| description  | TEXT               | 操作描述                                     |
| voucher_code | VARCHAR(255)       | 兑换码 (当操作为转入/转出时)                 |
| related_user | VARCHAR(255)       | 关联用户ID (转出/转入对象)                   |
| expiry_date  | TIMESTAMP          | 配额到期时间                                 |
| create_time  | TIMESTAMP          | 创建时间                                     |

### 3. voucher_redemption 表
用于跟踪已兑换的兑换码，防止重复兑换。

| 字段名      | 字段类型           | 说明     |
| ----------- | ------------------ | -------- |
| id          | SERIAL PRIMARY KEY | 自增主键 |
| voucher_code| VARCHAR(255)       | 兑换码   |
| receiver_id | VARCHAR(255)       | 接收者ID |
| create_time | TIMESTAMP          | 创建时间 |

## 新增 API 接口

### 1. 获取用户 quota 接口
**Endpoint**: `GET /api/v1/quota`

**Request**:
```json
{
  "user_id": "user123"
}
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
**Endpoint**: `GET /api/v1/quota/audit`

**Request**:
```json
{
  "user_id": "user123",
  "page": 1,
  "page_size": 10
}
```

**Response**:
```json
{
  "total": 25,
  "records": [
    {
      "amount": 100,
      "operation": "RECHARGE",
      "description": "每日充值策略",
      "expiry_date": "2025-06-30T23:59:59Z",
      "create_time": "2025-05-15T10:00:00Z"
    },
    {
      "amount": -50,
      "operation": "TRANSFER_OUT",
      "description": "您给用户user456转出总共50 Credit",
      "voucher_code": "xxxxx",
      "create_time": "2025-05-10T14:30:00Z"
    }
  ]
}
```

### 3. quota 转出接口
**Endpoint**: `POST /api/v1/quota/transfer-out`

**Request**:
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
  "voucher_code": "eyJnaXZlcl9pZCI6InVzZXIxMjMiLC...",
  "description": "您给用户user456转出总共30 Credit，兑换码:xxxxxxxx"
}
```

### 4. quota 转入接口
**Endpoint**: `POST /api/v1/quota/transfer-in`

**Request**:
```json
{
  "receiver_id": "user456",
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
  "description": "由用户张三给您转入总共30 Credit"
}
```

## 兑换码机制

### 生成步骤
1. 包含字段：giver_id, giver_name, giver_phone, giver_github, receiver_id, quota_amount, quota_expiry_time, timestamp
2. 序列化为 JSON 字符串
3. 使用 HMAC-SHA256 和密钥生成签名
4. 将序列化字符串和签名拼接，进行 Base64URL 编码

### 验证步骤
1. Base64URL 解码
2. 按分隔符分割得到 JSON 字符串和签名
3. 使用相同密钥计算 HMAC-SHA256 签名并比较
4. 反序列化 JSON 字符串为结构体

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

在 `config.yaml` 中新增兑换码签名密钥配置：

```yaml
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

# 生成测试数据
cd scripts
go run generate_data.go

# 启动 AiGateway 模拟服务
cd scripts/aigateway-mock
go run main.go
```

## 注意事项

1. 兑换码签名密钥应该足够长（至少32字节）并保密
2. quota 的到期时间默认设置为当月或下月的最后一天24点
3. 过期的 quota 不能被转入，但会在转入响应中标记为已过期
4. 所有 quota 变动都会记录在 quota_audit 表中用于审计
5. 定时任务会自动处理过期 quota 并同步 AiGateway 数据