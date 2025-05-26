# 配额管理器集成测试

这个目录包含配额管理器的完整集成测试套件。

## 测试覆盖范围

### 条件表达式测试
- ✅ 空条件表达式
- ✅ `match-user(user)` - 匹配特定用户
- ✅ `register-before(timestamp)` - 注册时间早于指定时间
- ✅ `access-after(timestamp)` - 最后访问时间晚于指定时间
- ✅ `github-star(project)` - 是否星标指定项目
- ✅ `quota-le(model, amount)` - 配额余额小于等于指定数量
- ✅ `is-vip(level)` - VIP等级大于等于指定级别
- ✅ `belong-to(org)` - 属于指定组织
- ✅ `and(condition1, condition2)` - 逻辑与
- ✅ `or(condition1, condition2)` - 逻辑或
- ✅ `not(condition)` - 逻辑非
- ✅ 复杂嵌套条件表达式

### 策略类型测试
- ✅ 单次充值策略（single） - 每个用户只执行一次
- ✅ 定时充值策略（periodic） - 可重复执行

### 策略状态测试
- ✅ 启用状态策略执行
- ✅ 禁用状态策略不执行
- ✅ 动态启用/禁用策略

### AiGateway集成测试
- ✅ 正常请求处理
- ✅ 请求失败处理和错误状态记录

### 批量处理测试
- ✅ 多用户批量处理
- ✅ 条件筛选和执行验证

## 快速开始

### 先决条件

1. **Go 1.21+**
2. **PostgreSQL 12+**
3. **数据库配置** - 确保PostgreSQL正在运行并且可以连接

### 运行测试

1. **使用脚本运行（推荐）**
   ```bash
   cd test
   chmod +x run_tests.sh
   ./run_tests.sh
   ```

2. **手动运行**
   ```bash
   # 设置环境变量（可选）
   export POSTGRES_HOST=localhost
   export POSTGRES_PORT=5432
   export POSTGRES_USER=postgres
   export POSTGRES_PASSWORD=password
   export POSTGRES_DB=quota_manager

   # 进入测试目录
   cd test

   # 运行测试
   go run integration_main.go
   ```

### 环境变量配置

| 变量 | 默认值 | 描述 |
|-----|--------|------|
| POSTGRES_HOST | localhost | PostgreSQL主机地址 |
| POSTGRES_PORT | 5432 | PostgreSQL端口 |
| POSTGRES_USER | postgres | 数据库用户名 |
| POSTGRES_PASSWORD | password | 数据库密码 |
| POSTGRES_DB | quota_manager | 数据库名称 |

## 测试架构

### 测试上下文（TestContext）
- **DB**: 数据库连接
- **StrategyService**: 策略服务实例
- **Gateway**: AiGateway客户端
- **MockServer**: 成功的模拟服务器
- **FailServer**: 失败的模拟服务器

### 模拟服务
测试使用内置的HTTP模拟服务器来模拟AiGateway的行为：
- **成功服务器**: 模拟正常的API响应
- **失败服务器**: 模拟API失败情况

### 测试数据管理
- 每个测试开始前清空数据库
- 创建特定的测试用户和策略
- 独立的配额存储模拟

## 测试流程

每个测试用例遵循以下流程：

1. **清空数据** - 确保测试环境干净
2. **配置测试数据** - 创建必要的用户和策略
3. **触发策略执行** - 调用策略服务执行策略
4. **检查执行结果** - 验证数据库记录和状态

## 输出示例

```
=== 配额管理器集成测试 ===
运行测试: 清空数据测试
✅ 清空数据测试 - 通过 (0.05s)
运行测试: 条件表达式-空条件测试
✅ 条件表达式-空条件测试 - 通过 (0.03s)
运行测试: 条件表达式-match-user测试
✅ 条件表达式-match-user测试 - 通过 (0.02s)
...

=== 测试结果摘要 ===
总测试数: 18
通过测试: 18
失败测试: 0
总耗时: 2.45s
成功率: 100.0%

🎉 所有测试都通过了！
```

## 故障排除

### 常见问题

1. **数据库连接失败**
   - 检查PostgreSQL是否运行
   - 验证环境变量设置
   - 检查数据库用户权限

2. **端口占用**
   - 测试使用动态端口分配，通常不会有冲突
   - 如果出现问题，重启测试即可

3. **依赖问题**
   - 运行 `go mod tidy` 更新依赖
   - 检查Go版本是否满足要求

### 调试模式

可以在测试代码中添加更多日志输出来调试问题：

```go
// 在测试函数中添加调试信息
fmt.Printf("调试信息: %+v\n", result)
```

## 扩展测试

### 添加新的条件表达式测试

1. 在 `integration_main.go` 中添加新的测试函数
2. 按照现有模式创建测试数据
3. 执行策略并验证结果
4. 在 `runAllTests` 中注册新测试

### 添加新的策略类型测试

1. 创建相应的测试策略
2. 验证特定的执行逻辑
3. 检查数据库状态变化

## 性能考虑

- 测试使用短间隔的定时策略（每分钟）以减少测试时间
- 批量测试限制用户数量（10个）以保持测试速度
- 模拟服务器在内存中运行，避免网络延迟