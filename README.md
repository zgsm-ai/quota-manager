# Quota Manager

A comprehensive quota management system built with Go and Gin framework, featuring advanced quota allocation, transfer functionality, and JWT-based authentication.

## Features

### Core Functionality
- **Strategy Management**: One-time and periodic recharge strategy types
- **Strategy Status Control**: Enable/disable strategies with real-time control
- **Complex Condition Matching**: Advanced functional condition expressions
- **Scheduled Tasks**: Cron-based strategy execution
- **Quota Expiry Management**: Time-based quota management with automatic expiry handling
- **JWT Authentication**: Token-based user authentication and authorization

### Model Permission Management (New)
- **Employee Synchronization**: Automated HR system integration for employee and department data
- **Permission Management**: Fine-grained model access control for users and departments
- **Department Hierarchy**: Support for complex organizational structures with permission inheritance
- **Real-time Updates**: Automatic permission synchronization with AI Gateway
- **Audit Trail**: Comprehensive permission operation tracking

### Star Check Permission Management (New)
- **Star Check Control**: User and department-level GitHub star check toggle management
- **Fine-grained Control**: Support for disabling/enabling star checks for specific users or departments
- **Permission Inheritance**: Department hierarchy-based star check setting inheritance
- **Priority Management**: User settings take precedence over department settings
- **Unified Interface**: Shared unified query and sync interfaces with model permission management

### Advanced Quota Operations
- **Quota Transfer**: Secure quota transfer between users with voucher codes
- **Audit Trail**: Comprehensive quota operation tracking
- **Multiple Expiry Dates**: Support for quotas with different expiry times
- **Real-time Sync**: Integration with AiGateway service

### Security & Reliability
- **HMAC-SHA256 Voucher Security**: Cryptographically signed voucher codes
- **Duplicate Prevention**: Protection against duplicate voucher redemption
- **Transaction Safety**: Database transaction support for complex operations

## Architecture

### Project Structure

```
quota-manager/
├── cmd/                    # Application entry point
│   └── main.go
├── internal/               # Internal packages
│   ├── config/            # Configuration management
│   ├── database/          # Database connection
│   ├── models/            # Data models
│   ├── services/          # Business logic
│   ├── handlers/          # HTTP handlers
│   └── condition/         # Condition expression parsing
├── pkg/                   # Public packages
│   ├── aigateway/         # AiGateway client
│   └── logger/            # Logging
├── scripts/               # Script files
├── test/                  # Integration tests
├── config.yaml            # Configuration file
└── README.md              # Project documentation
```

### Database Schema

#### Core Tables

**Strategy Table (quota_strategy)**
- `id`: Strategy ID
- `name`: Strategy name (unique)
- `title`: Strategy title
- `type`: Strategy type (periodic/single)
- `amount`: Recharge amount
- `model`: Model name (optional)
- `periodic_expr`: Cron expression for periodic strategies
- `condition`: Condition expression
- `max_exec_per_user`: Maximum execution times per user (0 means unlimited)
- `expiry_days`: Valid days for the quota (optional, specifies how many days the quota will be valid from creation)
- `status`: Strategy status (BOOLEAN: true=enabled, false=disabled)
- `create_time`: Creation time
- `update_time`: Update time

**Quota Table (quota)**
- `id`: Quota ID
- `user_id`: User ID
- `amount`: Quota amount
- `expiry_date`: Quota expiry time (NOT NULL)
- `status`: Status (VALID/EXPIRED)
- `create_time`: Creation time
- `update_time`: Update time

**Quota Audit Table (quota_audit)**
- `id`: Audit ID
- `user_id`: User ID
- `amount`: Amount change (positive/negative)
- `operation`: Operation type (RECHARGE/TRANSFER_IN/TRANSFER_OUT)
- `voucher_code`: Voucher code (for transfers)
- `related_user`: Related user ID
- `strategy_name`: Strategy name (for recharge operations)
- `expiry_date`: Quota expiry time (NOT NULL)
- `details`: JSON details for complex operations
- `create_time`: Creation time

**Voucher Redemption Table (voucher_redemption)**
- `id`: Redemption ID
- `voucher_code`: Voucher code (unique)
- `receiver_id`: Receiver user ID
- `create_time`: Creation time

#### Supporting Tables

**Execution Status Table (quota_execute)**
- `id`: Execution ID
- `strategy_id`: Strategy ID
- `user_id`: User ID
- `batch_number`: Batch number
- `status`: Execution status
- `expiry_date`: Quota expiry time (NOT NULL)
- `create_time`: Creation time
- `update_time`: Update time

**User Information Table (auth_users)**
- `id`: User ID (UUID)
- `created_at`: Creation time
- `updated_at`: Update time
- `access_time`: Last access time
- `name`: Username
- `github_id`: GitHub ID
- `github_name`: GitHub username
- `email`: Email
- `phone`: Phone number
- `github_star`: GitHub starred projects (comma-separated list)
- `vip`: VIP level
- `company`: Company
- `location`: Location
- `user_code`: User code
- `external_accounts`: External accounts
- `employee_number`: Employee number
- `password`: Password
- `devices`: Devices (JSON)

#### Permission Management Tables (New)

**Employee Department Table (employee_department)**
- `id`: Record ID
- `employee_number`: Employee number (unique)
- `username`: Employee username
- `dept_full_level_names`: Department hierarchy path (array)
- `create_time`: Creation time
- `update_time`: Update time

**Model Whitelist Table (model_whitelist)**
- `id`: Whitelist ID
- `target_type`: Target type ('user' or 'department')
- `target_identifier`: Employee number for users, department name for departments
- `allowed_models`: List of allowed models (array)
- `create_time`: Creation time
- `update_time`: Update time

**Effective Permissions Table (effective_permissions)**
- `id`: Permission ID
- `employee_number`: Employee number (unique)
- `effective_models`: Currently effective model list (array)
- `whitelist_id`: Reference to source whitelist entry
- `create_time`: Creation time
- `update_time`: Update time

**Permission Audit Table (permission_audit)**
- `id`: Audit ID
- `operation`: Operation type ('employee_sync', 'whitelist_set', 'permission_updated', 'star_check_set', 'star_check_setting_update')
- `target_type`: Target type ('user' or 'department')
- `target_identifier`: Target identifier
- `details`: Operation details (JSON)
- `create_time`: Creation time

**Star Check Settings Table (star_check_settings)**
- `id`: Setting ID
- `target_type`: Target type ('user' or 'department')
- `target_identifier`: Employee number for users, department name for departments
- `enabled`: Whether star check is enabled (boolean)
- `create_time`: Creation time
- `update_time`: Update time

**Effective Star Check Settings Table (effective_star_check_settings)**
- `id`: Setting ID
- `employee_number`: Employee number (unique)
- `enabled`: Currently effective star check setting (boolean)
- `setting_id`: Reference to source setting entry
- `create_time`: Creation time
- `update_time`: Update time

## Authentication System

### JWT Token Authentication
All API endpoints require JWT token authentication via HTTP headers:

```bash
Authorization: Bearer <jwt_token>
```

The system extracts user information from JWT tokens without signature verification, supporting:
- User ID (`id`)
- Name (`name`)
- Staff ID (`staffID`)
- GitHub username (`github`)
- Phone number (`phone`)

### Configuration
```yaml
server:
  port: 8099
  mode: "release"  # gin mode: debug, release, test
  token_header: "authorization"
  timezone: "Asia/Shanghai"  # timezone setting, defaults to Beijing Time (UTC+8)

# Employee Synchronization Configuration (New)
employee_sync:
  enabled: true
  hr_url: "http://hr-system/api/employees"
  hr_key: "your-hr-api-key"
  dept_url: "http://hr-system/api/departments"
  dept_key: "your-dept-api-key"
```

**Employee Sync Configuration:**
- `enabled`: Enable/disable employee synchronization
- `hr_url`: HR system API endpoint for employee data
- `hr_key`: Authentication key for HR employee API
- `dept_url`: HR system API endpoint for department data
- `dept_key`: Authentication key for HR department API
- Synchronization runs daily at 1:00 AM automatically
- Manual sync can be triggered via API endpoint

**Timezone Configuration:**
- `timezone`: Application timezone, supports IANA timezone names
- Common timezones:
  - `Asia/Shanghai`: Beijing Time (UTC+8)
  - `UTC`: Coordinated Universal Time
  - `America/New_York`: New York Time
  - `Europe/London`: London Time
- If not set, defaults to `Asia/Shanghai`
- Application restart required after timezone modification

## API Documentation

### Response Format

All API endpoints return responses in the following standard format:

```json
{
  "code": "quota-manager.success",
  "message": "Operation successful",
  "success": true,
  "data": { ... }
}
```

**Response Fields:**
- `code`: Status code (string format with meaningful identifiers)
- `message`: Response message in English
- `success`: Operation success status (boolean)
- `data`: Response data (optional, omitted when null)

### Authentication
All endpoints require JWT token in request headers:
```
Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...
```

### Strategy Management

#### Create Strategy
- **POST** `/quota-manager/api/v1/strategies`
- **Request Body**:
```json
{
  "name": "test-strategy",
  "title": "Test Strategy",
  "type": "single",
  "amount": 10,
  "model": "gpt-3.5-turbo",
  "condition": "github-star(\"zgsm\")",
  "status": true
  }
  ```
```json
{
  "code": "quota-manager.success",
  "message": "Strategy created successfully",
  "success": true,
  "data": {
    "id": 1,
    "name": "test-strategy",
    "title": "Test Strategy",
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

### Health Check

#### Health Check
- **GET** `/quota-manager/health`
- **Response**:
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

### Model Permission Management APIs (New)

#### Set User Whitelist
- **POST** `/quota-manager/api/v1/model-permissions/user`
- **Request Body**:
```json
{
  "employee_number": "85054712",
  "models": ["gpt-4", "claude-3-opus"]
}
```

#### Set Department Whitelist
- **POST** `/quota-manager/api/v1/model-permissions/department`
- **Request Body**:
```json
{
  "department_name": "R&D_Center",
  "models": ["gpt-4", "deepseek-v3"]
}
```

**Model Permission Priority (High to Low):**
1. User-specific whitelist
2. Most specific department whitelist (child dept > parent dept)
3. No permissions (empty list)

### Star Check Permission Management APIs (New)

#### Set User Star Check Setting
- **POST** `/quota-manager/api/v1/star-check-permissions/user`
- **Request Body**:
```json
{
  "employee_number": "85054712",
  "enabled": true
}
```

#### Set Department Star Check Setting
- **POST** `/quota-manager/api/v1/star-check-permissions/department`
- **Request Body**:
```json
{
  "department_name": "R&D_Center",
  "enabled": false
}
```

**Star Check Permission Priority (High to Low):**
1. User-specific setting
2. Most specific department setting (child dept > parent dept)
3. Default setting (enabled)

### Quota Check Permission Management APIs (New)

#### Set User Quota Check Setting
- **POST** `/quota-manager/api/v1/quota-check-permissions/user`
- **Request Body**:
```json
{
  "employee_number": "85054712",
  "enabled": true
}
```

#### Set Department Quota Check Setting
- **POST** `/quota-manager/api/v1/quota-check-permissions/department`
- **Request Body**:
```json
{
  "department_name": "R&D_Center",
  "enabled": false
}
```

**Quota Check Permission Priority (High to Low):**
1. User-specific setting
2. Most specific department setting (child dept > parent dept)
3. Default setting (disabled)

### Unified Permission Query and Sync APIs (New)

#### Get Effective Permissions
- **GET** `/quota-manager/api/v1/effective-permissions?type=model&target_type=user&target_identifier=85054712`
- **GET** `/quota-manager/api/v1/effective-permissions?type=star-check&target_type=user&target_identifier=85054712`
- **GET** `/quota-manager/api/v1/effective-permissions?type=quota-check&target_type=user&target_identifier=85054712`
- **GET** `/quota-manager/api/v1/effective-permissions?type=model&target_type=department&target_identifier=R&D_Center`
- **GET** `/quota-manager/api/v1/effective-permissions?type=star-check&target_type=department&target_identifier=R&D_Center`
- **GET** `/quota-manager/api/v1/effective-permissions?type=quota-check&target_type=department&target_identifier=R&D_Center`

**Query Parameters:**
- `type`: Permission type, `model` (model permissions), `star-check` (star check permissions), or `quota-check` (quota check permissions)
- `target_type`: Target type, `user` or `department`
- `target_identifier`: Target identifier (employee number for users or department name for departments)

#### Trigger Employee Sync
- **POST** `/quota-manager/api/v1/employee-sync`

This interface will synchronize employee data and update model permissions, star check permissions, and quota check permissions.

#### Get Strategy List
- **GET** `/quota-manager/api/v1/strategies`
- **Query Parameters**:
  - `status=enabled|disabled|true|false` - Filter by status
- **Response**:
```json
{
  "code": "quota-manager.success",
  "message": "Strategies retrieved successfully",
  "success": true,
  "data": {
    "strategies": [...],
    "total": 5
  }
}
```

#### Get Single Strategy
- **GET** `/quota-manager/api/v1/strategies/:id`
- **Response**:
```json
{
  "code": "quota-manager.success",
  "message": "Strategy retrieved successfully",
  "success": true,
  "data": {
    "id": 1,
    "name": "test-strategy",
    "title": "Test Strategy",
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

#### Update Strategy
- **PUT** `/quota-manager/api/v1/strategies/:id`
- **Request Body**: Partial strategy object
- **Response**:
```json
{
  "code": "quota-manager.success",
  "message": "Strategy updated successfully",
  "success": true
}
```

#### Strategy Status Control
- **POST** `/quota-manager/api/v1/strategies/:id/enable` - Enable strategy
- **POST** `/quota-manager/api/v1/strategies/:id/disable` - Disable strategy
- **Response**:
```json
{
  "code": "quota-manager.success",
  "message": "Strategy enabled successfully",
  "success": true
}
```

#### Delete Strategy
- **DELETE** `/quota-manager/api/v1/strategies/:id`
- **Response**:
```json
{
  "code": "quota-manager.success",
  "message": "Strategy deleted successfully",
  "success": true
}
```

#### Manual Strategy Scan
- **POST** `/quota-manager/api/v1/strategies/scan`
- **Response**:
```json
{
  "code": "quota-manager.success",
  "message": "Strategy scan triggered successfully",
  "success": true
}
```

#### Get Strategy Execution Records
- **GET** `/quota-manager/api/v1/strategies/:id/executions`
- **Query Parameters**:
  - `page`: Page number (default: 1)
  - `page_size`: Page size (default: 10)
- **Response**:
```json
{
  "code": "quota-manager.success",
  "message": "Strategy execution records retrieved successfully",
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

### Quota Management

#### Get User Quota
- **GET** `/quota-manager/api/v1/quota`
- **Response**:
```json
{
  "code": "quota-manager.success",
  "message": "User quota retrieved successfully",
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

**Field Descriptions**:
- `total_quota`: Total available quota from AiGateway
- `used_quota`: Currently used quota from AiGateway
- `quota_list`: Array of quota items with different expiry dates
  - `amount`: Remaining quota amount after deducting used quota
  - `expiry_date`: Quota expiry timestamp

#### Get Quota Audit Records
- **GET** `/quota-manager/api/v1/quota/audit?page=1&page_size=10`
- **Query Parameters**:
  - `page`: Page number (default: 1)
  - `page_size`: Page size (default: 10)
- **Response**:
```json
{
  "code": "quota-manager.success",
  "message": "Quota audit records retrieved successfully",
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

**Field Descriptions**:
- `total`: Total number of audit records
- `records`: Array of audit records
  - `amount`: Quota change amount (positive for increase, negative for decrease)
  - `operation`: Operation type (RECHARGE/TRANSFER_IN/TRANSFER_OUT)
  - `voucher_code`: Voucher code for transfer operations
  - `related_user`: Related user ID for transfer operations
  - `strategy_name`: Strategy name for recharge operations
  - `expiry_date`: Quota expiry timestamp
  - `details`: Detailed operation information (JSON object)
  - `create_time`: Operation timestamp

#### Transfer Out Quota
- **POST** `/quota-manager/api/v1/quota/transfer-out`
- **Request Body**:
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
- **Response**:
```json
{
  "code": "quota-manager.success",
  "message": "Quota transferred out successfully",
  "success": true,
  "data": {
    "voucher_code": "ABCD1234EFGH5678",
    "total_amount": 30,
    "expiry_date": "2025-06-30T23:59:59Z"
  }
}
```

#### Transfer In Quota
- **POST** `/quota-manager/api/v1/quota/transfer-in`
- **Request Body**:
```json
{
  "voucher_code": "ABCD1234EFGH5678"
}
```
- **Response**:
```json
{
  "code": "quota-manager.success",
  "message": "Quota transferred in successfully",
  "success": true,
  "data": {
    "status": "success",
    "message": "Transfer completed successfully",
    "total_amount": 30,
    "transfer_details": [...]
  }
}
```

#### Get User Quota Audit Records (Admin)
- **GET** `/quota-manager/api/v1/quota/audit/:user_id?page=1&page_size=10`
- **Path Parameters**:
  - `user_id`: Target user ID (required)
- **Query Parameters**:
  - `page`: Page number (default: 1)
  - `page_size`: Page size (default: 10)
- **Response**:
```json
{
  "code": "quota-manager.success",
  "message": "User quota audit records retrieved successfully",
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
- **Response**:
```json
{
  "code": "quota-manager.success",
  "message": "Quota transferred out successfully",
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

**Field Descriptions**:
- `voucher_code`: Generated voucher code for the transfer
- `related_user`: Receiver user ID
- `operation`: Always "TRANSFER_OUT"
- `quota_list`: Array of transferred quota items
  - `amount`: Quota amount
  - `expiry_date`: Quota expiry timestamp

#### Transfer In Quota
- **POST** `/quota-manager/api/v1/quota/transfer-in`
- **Request Body**:
```json
{
  "voucher_code": "eyJnaXZlcl9pZCI6InVzZXIxMjMiLC..."
}
```
- **Response**:
```json
{
  "code": "quota-manager.success",
  "message": "Quota transferred in successfully",
  "success": true,
  "data": {
    "giver_id": "user123",
    "giver_name": "John Doe",
    "giver_phone": "13800138000",
    "giver_github": "johndoe",
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
    "message": "All quota transfers completed successfully"
  }
}
```

**Field Descriptions**:
- `giver_id`: Giver user ID
- `giver_name`: Giver display name
- `giver_phone`: Giver phone number
- `giver_github`: Giver GitHub username
- `giver_github_star`: Giver's starred projects (comma-separated list, transferred to receiver)
- `receiver_id`: Receiver user ID
- `quota_list`: Array of transfer results
  - `amount`: Quota amount
  - `expiry_date`: Quota expiry timestamp
  - `is_expired`: Whether the quota has expired
  - `success`: Whether the transfer was successful
  - `failure_reason`: Reason for failure (if any)
- `voucher_code`: Original voucher code
- `operation`: Always "TRANSFER_IN"
- `amount`: Total successfully transferred amount
- `status`: Transfer status (SUCCESS/PARTIAL_SUCCESS/FAILED/ALREADY_REDEEMED)
- `message`: Status description

#### Merge User Quota
- **POST** `/quota-manager/api/v1/quota/merge`
- **Request Body**:
```json
{
  "main_user_id": "user123",
  "other_user_id": "user456"
}
```
- **Response**:
```json
{
  "code": "quota-manager.success",
  "message": "Quota merged successfully",
  "success": true,
  "data": {
    "main_user_id": "user123",
    "other_user_id": "user456",
    "amount": 30,
    "operation": "MERGE_QUOTA",
    "status": "SUCCESS",
    "message": "Quota merged successfully"
  }
}
```

**Field Descriptions**:
- `main_user_id`: Main user ID (user who will receive the merged quota)
- `other_user_id`: Other user ID (user whose quota will be merged)
- `amount`: Total amount of quota merged
- `operation`: Operation type (always "MERGE_QUOTA")
- `status`: Operation status (SUCCESS/FAILED)
- `message`: Status message

**Important Notes**:
- This operation merges all valid quotas from the other user to the main user
- The main user and other user cannot be the same
- Only valid quotas (status = VALID and amount > 0) will be merged
- If the main user already has quotas with the same expiry date and status, the amounts will be added together
- The original quotas from the other user will be deleted after successful merge
- This operation is performed within a database transaction for data consistency

### Health Check
- **GET** `/quota-manager/health`
- **Response**:
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

### Error Responses

All error responses follow the same format:

```json
{
  "code": "quota-manager.bad_request",
  "message": "Invalid request parameters",
  "success": false
}
```

**Common Error Codes:**
- `quota-manager.bad_request`: Bad Request - Invalid request parameters
- `quota-manager.unauthorized`: Unauthorized - Authentication failed
- `quota-manager.token_invalid`: Token Invalid - Invalid or missing JWT token
- `quota-manager.strategy_not_found`: Strategy Not Found - Strategy with specified ID not found
- `quota-manager.invalid_strategy_id`: Invalid Strategy ID - Strategy ID format is invalid
- `quota-manager.insufficient_quota`: Insufficient Quota - User does not have enough quota
- `quota-manager.voucher_invalid`: Voucher Invalid - Voucher code is invalid or malformed
- `quota-manager.voucher_expired`: Voucher Expired - Voucher code has expired
- `quota-manager.voucher_already_redeemed`: Voucher Already Redeemed - Voucher has been used
- `quota-manager.quota_transfer_failed`: Quota Transfer Failed - Failed to transfer quota
- `quota-manager.strategy_create_failed`: Strategy Create Failed - Failed to create strategy
- `quota-manager.strategy_update_failed`: Strategy Update Failed - Failed to update strategy
- `quota-manager.strategy_delete_failed`: Strategy Delete Failed - Failed to delete strategy
- `quota-manager.database_error`: Database Error - Database operation failed
- `quota-manager.aigateway_error`: AiGateway Error - AiGateway service error
- `quota-manager.internal_error`: Internal Server Error - Unexpected server error

## Condition Expressions

The system supports complex condition expressions for strategy targeting:

### Available Functions

- `access-after(timestamp)`: Last access after specified time
- `and(condition1, condition2)`: Logical AND
- `belong-to(org1, org2)`: Belongs to specified organization or department. When `employee_sync.enabled = true`, checks if user belongs to the department via employee_department table using their EmployeeNumber. Supports both Chinese and English department names. Falls back to Company field when employee sync is disabled or employee number is empty.
- `false()`: Always returns false (no users will match)
- `github-star(project)`: Whether user has starred the specified project (checks against user's starred projects list)
- `is-vip(level)`: VIP level greater than or equal to specified level
- `match-user("user1", "user2", ...)`: Check if the current user's ID is present in the provided list of IDs (supports multiple parameters)
- `not(condition)`: Logical NOT
- `or(condition1, condition2)`: Logical OR
- `quota-le(model, amount)`: Quota balance less than or equal to amount
- `register-before(timestamp)`: Registration before specified time
- `true()`: Always returns true (all users will match)

### Examples

```
# Always execute for all users (replaces empty condition)
true()

# Never execute for any users (useful for temporarily disabling)
false()

# Recharge users who have starred the zgsm project
github-star("zgsm")

# Recharge VIP users who are recently active
and(is-vip(1), access-after("2024-05-01 00:00:00"))

# Recharge early registered users or VIP users
or(register-before("2023-01-01 00:00:00"), is-vip(2))

# Recharge users in specific department (supports Chinese and English names)
belong-to("技术部")       # Chinese department name
belong-to("Tech_Group_1", "Tech_Group_2")   # English department name

# Recharge specific user IDs
match-user("user123", "user456")

# Combine department with other conditions
and(belong-to("R&D_Center"), is-vip(2))

# Complex condition with true/false functions
or(and(is-vip(3), true()), and(false(), github-star("project")))
```

## Voucher System

### Voucher Code Generation
1. Create voucher data with giver info, receiver ID, and quota list
2. Serialize to JSON and add timestamp
3. Generate HMAC-SHA256 signature with secret key
4. Combine JSON and signature, encode with Base64URL

### Voucher Code Validation
1. Base64URL decode
2. Split JSON data and signature
3. Verify HMAC-SHA256 signature
4. Deserialize JSON to voucher data
5. Check for duplicate redemption

### Configuration
```yaml
voucher:
  signing_key: "your-secret-signing-key-at-least-32-bytes-long-for-security"
```

## Scheduled Tasks

### Strategy Execution Task
- **Frequency**: Every hour
- **Function**: Scan and execute recharge strategies

### Quota Expiry Task
- **Frequency**: First day of every month at 00:01
- **Function**:
  - Mark expired quotas as invalid
  - Sync quota data with AiGateway
  - Adjust user total and used quotas

## Quick Start

### Requirements

- Go 1.21+
- PostgreSQL 12+

### Installation

1. **Clone Repository**
   ```bash
   git clone <repository-url>
   cd quota-manager
   ```

2. **Configure Database**
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

   The system uses a separated database architecture:
   - `database`: Contains quota-related tables (quota_strategy, quota_execute, quota, quota_audit, voucher_redemption)
   - `auth_database`: Contains user authentication data (auth_users table)

3. **Start Services**
   ```bash
   # Use startup script (recommended)
   chmod +x scripts/start.sh
   ./scripts/start.sh

   # Or manual startup
   go mod tidy
   psql -U postgres -f scripts/init_db.sql
   cd scripts && go run generate_data.go && cd ..
   cd scripts/aigateway-mock && go run main.go &
   cd ../../ && go run cmd/main.go
   ```

## AiGateway Integration

### Mock Service
The project includes a complete AiGateway mock service (`scripts/aigateway-mock/`) providing:

- `POST /v1/chat/completions/quota/refresh` - Refresh quota
- `GET /v1/chat/completions/quota` - Query quota
- `POST /v1/chat/completions/quota/delta` - Modify quota
- `GET /v1/chat/completions/quota/used` - Query used quota
- `POST /v1/chat/completions/quota/used/delta` - Modify used quota

### Configuration
```yaml
aigateway:
  host: "localhost"
  port: 1002
  admin_path: "/v1/chat/completions/quota"
  auth_header: "x-admin-key"
  auth_value: "12345678"
```

## Configuration

### Complete Configuration File
```yaml
server:
  port: 8099
  mode: "release"  # gin mode: debug, release, test
  token_header: "authorization"
  timezone: "Asia/Shanghai"  # timezone setting, defaults to Beijing Time (UTC+8)

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

## Testing

### Integration Tests
The project provides comprehensive integration tests:

```bash
# Run integration tests
cd test
chmod +x run_tests.sh
./run_tests.sh

# Or run manually
go run main.go
```

### Test Coverage
- **Condition Expression Tests**: All supported functions and logic
- **Strategy Type Tests**: One-time and periodic strategies
- **Status Control Tests**: Enable/disable functionality
- **Quota Transfer Tests**: Transfer out/in operations
- **Audit Trail Tests**: Record tracking and querying
- **Expiry Management Tests**: Time-based quota handling
- **AiGateway Integration**: Normal and failure scenarios

### API Testing
```bash
# Test API endpoints
chmod +x scripts/test_api.sh
./scripts/test_api.sh
```

### Test Data Generation
```bash
# Generate test data
cd scripts
go run generate_data.go
```

## Development

### Adding Condition Functions
1. Add expression structure in `internal/condition/parser.go`
2. Implement `Evaluate` method
3. Add parsing logic in `buildFunction` method

### Extending Strategy Types
1. Add type handling in `ExecStrategy` method
2. Update data model and validation

### Database Migrations
Use GORM auto-migration or manual SQL scripts in `scripts/init_db.sql`

## Troubleshooting

### Common Issues

1. **Database Connection Failure**
   - Verify PostgreSQL service is running
   - Check connection parameters in config.yaml
   - Ensure database exists and is accessible

2. **AiGateway Connection Failure**
   - Verify mock service is running on port 1002
   - Check authorization credentials
   - Ensure no port conflicts

3. **Strategy Not Executing**
   - Verify strategy status is enabled (`true`)
   - Check cron expression syntax
   - Validate condition expression
   - Review logs for detailed errors

4. **JWT Token Issues**
   - Ensure token contains required fields (id, name, etc.)
   - Verify token header name in configuration
   - Check token format (Bearer prefix)

### Debug Mode
```bash
export GIN_MODE=debug
```

### Logging
The system uses structured JSON logging with zap, providing:
- Strategy execution status
- Quota operation tracking
- Error information with stack traces
- Performance metrics

## Security Considerations

1. **Voucher Security**: Uses HMAC-SHA256 for voucher code integrity
2. **Token Validation**: JWT parsing without signature verification (ensure secure token source)
3. **Database Security**: Use secure database credentials and SSL connections
4. **API Security**: All endpoints require authentication
5. **Key Management**: Store signing keys securely and rotate regularly

## Performance

### Optimizations
- Efficient database indexing
- Connection pooling
- Batch processing for large operations
- Memory-efficient data structures

### Monitoring
- Structured logging for observability
- Health check endpoints
- Database query optimization
- Resource usage tracking

## License

MIT License