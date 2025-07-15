# Employee Sync Mock Server

这是一个用于 quota-manager 项目的 employee_sync 功能的 Mock 服务器，用于模拟 HR 系统的员工和部门数据 API。

## 功能特性

- 提供超过 50 个部门的层级结构数据
- 提供超过 200 个员工的分布式数据
- 支持 AES 加密和 base64 编码
- 模拟真实的 HR 系统 API 响应格式
- 部门具有完整的上下级关系
- 员工分布在不同的部门中

## API 端点

### 1. 获取员工数据
- **URL**: `/api/hr/employees`
- **Method**: GET
- **Response**: AES 加密的员工数据 (base64 编码)
- **加密密钥**: `test-hr-key-for-aes-256-gcm-32b!`

### 2. 获取部门数据
- **URL**: `/api/hr/departments`
- **Method**: GET
- **Response**: AES 加密的部门数据 (base64 编码)
- **加密密钥**: `test-dept-key-for-aes-256-g-32b!`

### 3. 服务状态
- **URL**: `/status`
- **Method**: GET
- **Response**: 服务状态信息（JSON 格式）

### 4. 健康检查
- **URL**: `/health`
- **Method**: GET
- **Response**: 健康状态信息

## 数据结构

### 员工数据结构 (HREmployee)
```json
{
  "badge": "员工编号",
  "Name": "员工姓名",
  "DepID": "部门ID（字符串格式）",
  "email": "邮箱地址",
  "TEL": "手机号码"
}
```

### 部门数据结构 (HRDepartment)
```json
{
  "Id": "部门ID（字符串格式）",
  "AdminId": "上级部门ID",
  "Name": "部门名称",
  "DepGrade": "部门级别",
  "DepartmentStatus": "部门状态"
}
```

## 部门层级结构

```
深信服科技 (ID: 1)
├── 研发中心 (ID: 2)
│   ├── AI研发部 (ID: 11)
│   │   ├── 机器学习团队 (ID: 35)
│   │   ├── 深度学习团队 (ID: 36)
│   │   ├── 自然语言处理团队 (ID: 37)
│   │   └── ...
│   ├── 安全研发部 (ID: 12)
│   ├── 网络研发部 (ID: 13)
│   └── ...
├── 产品中心 (ID: 3)
├── 销售中心 (ID: 4)
├── 市场中心 (ID: 5)
└── ...
```

## 启动服务器

### 方式一：直接运行
```bash
cd scripts/employee-sync-mock
go run main.go
```

### 方式二：构建后运行
```bash
cd scripts/employee-sync-mock
go build -o employee-sync-mock
./employee-sync-mock
```

### 方式三：使用启动脚本
```bash
cd scripts
./start_employee_sync_mock.sh
```

## 服务器配置

- **端口**: 8098
- **运行模式**: Release
- **员工数据加密密钥**: `test-hr-key-for-aes-256-gcm-32b!`
- **部门数据加密密钥**: `test-dept-key-for-aes-256-g-32b!`

## 测试数据统计

- **总部门数**: 66 个部门
- **总员工数**: 超过 200 名员工
- **部门层级**: 4 层组织结构
- **员工分布**: 员工分布在不同的部门中，每个部门有不同数量的员工

## 与 quota-manager 集成

在 `config_local.yaml` 中配置：

```yaml
employee_sync:
  enabled: true
  hr_url: "http://localhost:8098/api/hr/employees"
  hr_key: "test-hr-key-for-aes-256-gcm-32b!"
  dept_url: "http://localhost:8098/api/hr/departments"
  dept_key: "test-dept-key-for-aes-256-g-32b!"
```

## 注意事项

1. 该服务器仅用于开发和测试目的
2. 加密密钥是硬编码的，不适用于生产环境
3. 数据是模拟生成的，不包含真实的员工信息
4. 服务器启动后会在控制台显示生成的数据统计信息

## 故障排除

如果遇到端口冲突，可以修改 `main.go` 中的端口设置，或者确保端口 8099 没有被其他服务占用。