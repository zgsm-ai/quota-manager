# Quota Manager

基于 Go 语言和 Gin 框架的配额管理系统，用于管理用户配额充值策略。

## 功能特性

- **策略管理**: 支持单次充值和定时充值两种策略类型
- **条件匹配**: 支持复杂的函数式条件表达式
- **定时任务**: 基于 cron 表达式的定时策略执行
- **AiGateway 集成**: 与外部 AiGateway 服务集成进行配额操作
- **数据库支持**: 使用 PostgreSQL 存储数据
- **RESTful API**: 提供完整的策略管理 API

## 项目结构

```
quota-manager/
├── cmd/                    # 应用程序入口
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
│   ├── init_db.sql        # 数据库初始化
│   ├── generate_data.go   # 数据生成
│   └── start.sh           # 启动脚本
├── config.yaml            # 配置文件
├── go.mod                 # Go 模块文件
└── README.md              # 项目说明
```

## 数据库表结构

### 策略表 (quota_strategy)
- `id`: 策略ID
- `name`: 策略名称（唯一）
- `title`: 策略标题
- `type`: 策略类型（periodic/single）
- `amount`: 充值数量
- `model`: 模型名称
- `periodic_expr`: 定时表达式
- `condition`: 条件表达式
- `create_time`: 创建时间
- `update_time`: 更新时间

### 执行状态表 (quota_execute)
- `id`: 执行ID
- `strategy_id`: 策略ID
- `user`: 用户ID
- `batch_number`: 批次号
- `status`: 执行状态
- `create_time`: 创建时间
- `update_time`: 更新时间

### 用户信息表 (user_info)
- `id`: 用户ID
- `name`: 用户名
- `github_username`: GitHub用户名
- `email`: 邮箱
- `phone`: 手机号
- `github_star`: GitHub star项目列表
- `vip`: VIP等级
- `org`: 组织ID
- `register_time`: 注册时间
- `access_time`: 最后访问时间
- `create_time`: 创建时间
- `update_time`: 更新时间

## 条件表达式

支持以下条件函数：

- `match-user(user)`: 匹配特定用户
- `register-before(timestamp)`: 注册时间早于指定时间
- `access-after(timestamp)`: 最后访问时间晚于指定时间
- `github-star(project)`: 是否给指定项目点过star
- `quota-le(model, amount)`: 配额余量小于等于指定数量
- `is-vip(level)`: VIP等级大于等于指定级别
- `belong-to(org)`: 属于指定组织
- `and(condition1, condition2)`: 逻辑与
- `or(condition1, condition2)`: 逻辑或
- `not(condition)`: 逻辑非

### 条件表达式示例

```
# 给点过zgsm项目star的用户充值
github-star("zgsm")

# 给VIP用户且最近活跃的用户充值
and(is-vip(1), access-after("2024-05-01 00:00:00"))

# 给早期注册用户或VIP用户充值
or(register-before("2023-01-01 00:00:00"), is-vip(2))
```

## 快速开始

### 环境要求

- Go 1.21+
- PostgreSQL 12+

### 安装和运行

1. **克隆项目**
   ```bash
   git clone <repository-url>
   cd quota-manager
   ```

2. **配置数据库**

   修改 `config.yaml` 中的数据库配置：
   ```yaml
   database:
     host: "localhost"
     port: 5432
     user: "postgres"
     password: "password"
     dbname: "quota_manager"
     sslmode: "disable"
   ```

3. **使用启动脚本**
   ```bash
   chmod +x scripts/start.sh
   ./scripts/start.sh
   ```

4. **手动启动**

   如果不使用启动脚本，可以手动执行以下步骤：

   ```bash
   # 下载依赖
   go mod tidy

   # 初始化数据库
   psql -U postgres -f scripts/init_db.sql

   # 生成测试数据
   cd scripts && go run generate_data.go && cd ..

   # 启动 AiGateway 模拟服务
   cd ../aigateway-mock && go run main.go &

   # 启动主服务
   cd ../quota-manager && go run cmd/main.go
   ```

## API 接口

### 策略管理

- `POST /api/v1/strategies` - 创建策略
- `GET /api/v1/strategies` - 获取策略列表
- `GET /api/v1/strategies/:id` - 获取单个策略
- `PUT /api/v1/strategies/:id` - 更新策略
- `DELETE /api/v1/strategies/:id` - 删除策略
- `POST /api/v1/strategies/scan` - 手动触发策略扫描

### 健康检查

- `GET /health` - 服务健康检查

### 创建策略示例

```bash
curl -X POST http://localhost:8080/api/v1/strategies \
  -H "Content-Type: application/json" \
  -d '{
    "name": "test-strategy",
    "title": "测试策略",
    "type": "single",
    "amount": 10,
    "model": "gpt-3.5-turbo",
    "condition": "github-star(\"zgsm\")"
  }'
```

## AiGateway 模拟服务

项目包含一个 AiGateway 模拟服务，提供以下接口：

- `POST /v1/chat/completions/quota/refresh` - 刷新配额
- `GET /v1/chat/completions/quota` - 查询配额
- `POST /v1/chat/completions/quota/delta` - 增减配额

模拟服务运行在端口 1002。

## 配置说明

### 配置文件 (config.yaml)

```yaml
database:
  host: "pg"              # 数据库主机
  port: 1001              # 数据库端口
  user: "postgres"        # 数据库用户
  password: "password"    # 数据库密码
  dbname: "quota_manager" # 数据库名
  sslmode: "disable"      # SSL模式

aigateway:
  host: "aigateway"       # AiGateway主机
  port: 1002              # AiGateway端口
  admin_path: "/v1/chat/completions"  # 管理路径
  credential: "credential3"           # 认证凭据

server:
  port: 8080              # 服务端口
  mode: "debug"           # 运行模式

scheduler:
  scan_interval: "0 0 * * * *"  # 扫描间隔（每小时）
```

## 开发说明

### 添加新的条件函数

1. 在 `internal/condition/parser.go` 中添加新的表达式结构
2. 实现 `Evaluate` 方法
3. 在 `buildFunction` 方法中添加解析逻辑

### 扩展策略类型

1. 在 `internal/services/strategy.go` 中的 `ExecStrategy` 方法添加新类型处理
2. 更新数据模型和验证逻辑

## 测试

项目包含了完整的测试数据生成脚本，会创建：

- 20个测试用户（包含不同VIP等级、组织、GitHub star等）
- 7个测试策略（包含各种条件和类型）

## 日志

系统使用 zap 日志库，日志格式为 JSON，包含以下信息：

- 策略执行状态
- 用户充值记录
- 错误信息
- 系统状态

## 故障排除

### 常见问题

1. **数据库连接失败**
   - 检查 PostgreSQL 服务是否运行
   - 验证配置文件中的数据库连接信息

2. **AiGateway 连接失败**
   - 确保 AiGateway 模拟服务正在运行
   - 检查端口是否被占用

3. **策略不执行**
   - 检查 cron 表达式是否正确
   - 验证条件表达式语法
   - 查看日志了解详细错误信息

### 调试模式

设置环境变量启用调试模式：
```bash
export GIN_MODE=debug
```

## 许可证

MIT License