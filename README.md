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
- `github_star`: GitHub star project list
- `vip`: VIP level
- `company`: Company
- `location`: Location
- `user_code`: User code
- `external_accounts`: External accounts
- `employee_number`: Employee number
- `password`: Password
- `devices`: Devices (JSON)

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
  mode: "debug"
  token_header: "authorization"  # Customizable token header name
```

## API Documentation

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
- **Response**: Strategy object with generated ID

#### Get Strategy List
- **GET** `/quota-manager/api/v1/strategies`
- **Query Parameters**:
  - `status=enabled|disabled|true|false` - Filter by status
- **Response**:
```json
{
  "strategies": [...],
  "total": 5
}
```

#### Get Single Strategy
- **GET** `/quota-manager/api/v1/strategies/:id`
- **Response**: Strategy object

#### Update Strategy
- **PUT** `/quota-manager/api/v1/strategies/:id`
- **Request Body**: Partial strategy object
- **Response**:
```json
{
  "message": "strategy updated successfully"
}
```

#### Strategy Status Control
- **POST** `/quota-manager/api/v1/strategies/:id/enable` - Enable strategy
- **POST** `/quota-manager/api/v1/strategies/:id/disable` - Disable strategy
- **Response**:
```json
{
  "message": "strategy enabled/disabled successfully"
}
```

#### Delete Strategy
- **DELETE** `/quota-manager/api/v1/strategies/:id`
- **Response**:
```json
{
  "message": "strategy deleted successfully"
}
```

#### Manual Strategy Scan
- **POST** `/quota-manager/api/v1/strategies/scan`
- **Response**:
```json
{
  "message": "strategy scan triggered"
}
```

### Quota Management

#### Get User Quota
- **GET** `/quota-manager/api/v1/quota`
- **Response**:
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
```

**Field Descriptions**:
- `giver_id`: Giver user ID
- `giver_name`: Giver display name
- `giver_phone`: Giver phone number
- `giver_github`: Giver GitHub username
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

### Health Check
- **GET** `/quota-manager/health`
- **Response**:
```json
{
  "status": "ok"
}
```

## Condition Expressions

The system supports complex condition expressions for strategy targeting:

### Available Functions

- `match-user(user)`: Match specific user
- `register-before(timestamp)`: Registration before specified time
- `access-after(timestamp)`: Last access after specified time
- `github-star(project)`: Whether starred the specified project
- `quota-le(model, amount)`: Quota balance less than or equal to amount
- `is-vip(level)`: VIP level greater than or equal to specified level
- `belong-to(org)`: Belongs to specified organization
- `and(condition1, condition2)`: Logical AND
- `or(condition1, condition2)`: Logical OR
- `not(condition)`: Logical NOT

### Examples

```
# Recharge users who starred the zgsm project
github-star("zgsm")

# Recharge VIP users who are recently active
and(is-vip(1), access-after("2024-05-01 00:00:00"))

# Recharge early registered users or VIP users
or(register-before("2023-01-01 00:00:00"), is-vip(2))
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
  mode: "debug"
  token_header: "authorization"

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