# Quota Manager

A quota management system based on Go language and Gin framework, used for managing user quota recharge strategies.

## Features

- **Strategy Management**: Supports both one-time and periodic recharge strategy types
- **Strategy Status Control**: Supports enabling/disabling strategies, only enabled strategies will be executed
- **Condition Matching**: Supports complex functional condition expressions
- **Scheduled Tasks**: Scheduled strategy execution based on cron expressions
- **AiGateway Integration**: Integration with external AiGateway service for quota operations
- **Database Support**: Uses PostgreSQL for data storage
- **RESTful API**: Provides complete strategy management API

## Project Structure

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
│   ├── start.sh           # Startup script
│   ├── init_db.sql        # Database initialization
│   ├── generate_data.go   # Data generation
│   └── test_api.sh        # API testing script
├── config.yaml            # Configuration file
├── go.mod                 # Go module file
└── README.md              # Project documentation
```

## Database Schema

### Strategy Table (quota_strategy)
- `id`: Strategy ID
- `name`: Strategy name (unique)
- `title`: Strategy title
- `type`: Strategy type (periodic/single)
- `amount`: Recharge amount
- `model`: Model name
- `periodic_expr`: Periodic expression
- `condition`: Condition expression
- `status`: Strategy status (BOOLEAN: true=enabled, false=disabled)
- `create_time`: Creation time
- `update_time`: Update time

### Execution Status Table (quota_execute)
- `id`: Execution ID
- `strategy_id`: Strategy ID
- `user`: User ID
- `batch_number`: Batch number
- `status`: Execution status
- `create_time`: Creation time
- `update_time`: Update time

### User Information Table (user_info)
- `id`: User ID
- `name`: Username
- `github_username`: GitHub username
- `email`: Email
- `phone`: Phone number
- `github_star`: GitHub star project list
- `vip`: VIP level
- `org`: Organization ID
- `register_time`: Registration time
- `access_time`: Last access time
- `create_time`: Creation time
- `update_time`: Update time

## Strategy Status Management

### Status Types
- `true`: Enabled status, strategy will be executed normally
- `false`: Disabled status, strategy will not be executed

### Status Control
- Only strategies with status `true` will be executed during scheduled scans
- Initial status can be specified when creating a strategy, defaults to `true` (enabled)
- Strategies can be dynamically enabled or disabled via API
- Supports filtering strategy list by status

## Condition Expressions

Supports the following condition functions:

- `match-user(user)`: Match specific user
- `register-before(timestamp)`: Registration time before specified time
- `access-after(timestamp)`: Last access time after specified time
- `github-star(project)`: Whether starred the specified project
- `quota-le(model, amount)`: Quota balance less than or equal to specified amount
- `is-vip(level)`: VIP level greater than or equal to specified level
- `belong-to(org)`: Belongs to specified organization
- `and(condition1, condition2)`: Logical AND
- `or(condition1, condition2)`: Logical OR
- `not(condition)`: Logical NOT

### Condition Expression Examples

```
# Recharge users who starred the zgsm project
github-star("zgsm")

# Recharge VIP users who are recently active
and(is-vip(1), access-after("2024-05-01 00:00:00"))

# Recharge early registered users or VIP users
or(register-before("2023-01-01 00:00:00"), is-vip(2))
```

## Quick Start

### Requirements

- Go 1.21+
- PostgreSQL 12+

### Installation and Running

1. **Clone Project**
   ```bash
   git clone <repository-url>
   cd quota-manager
   ```

2. **Configure Database**

   Modify database configuration in `config.yaml`:
   ```yaml
   database:
     host: "localhost"
     port: 5432
     user: "postgres"
     password: "password"
     dbname: "quota_manager"
     sslmode: "disable"
   ```

3. **Use Startup Script**
   ```bash
   chmod +x scripts/start.sh
   ./scripts/start.sh
   ```

4. **Manual Startup**

   If not using the startup script, you can manually execute the following steps:

   ```bash
   # Download dependencies
   go mod tidy

   # Initialize database
   psql -U postgres -f scripts/init_db.sql

   # Generate test data
   cd scripts && go run generate_data.go && cd ..

   # Start AiGateway mock service
   cd scripts/aigateway-mock && go run main.go &

   # Start main service
   cd ../../ && go run cmd/main.go
   ```

## API Endpoints

### Strategy Management

- `POST /api/v1/strategies` - Create strategy
- `GET /api/v1/strategies` - Get strategy list
  - Supports query parameter `?status=enabled` or `?status=true` to get enabled strategies
  - Supports query parameter `?status=disabled` or `?status=false` to get disabled strategies
- `GET /api/v1/strategies/:id` - Get single strategy
- `PUT /api/v1/strategies/:id` - Update strategy
- `DELETE /api/v1/strategies/:id` - Delete strategy
- `POST /api/v1/strategies/:id/enable` - Enable strategy
- `POST /api/v1/strategies/:id/disable` - Disable strategy
- `POST /api/v1/strategies/scan` - Manually trigger strategy scan

### Health Check

- `GET /health` - Service health check

### Create Strategy Example

```bash
curl -X POST http://localhost:8080/api/v1/strategies \
  -H "Content-Type: application/json" \
  -d '{
    "name": "test-strategy",
    "title": "Test Strategy",
    "type": "single",
    "amount": 10,
    "model": "gpt-3.5-turbo",
    "condition": "github-star(\"zgsm\")",
    "status": true
  }'
```

### Strategy Status Management Examples

```bash
# Enable strategy
curl -X POST http://localhost:8080/api/v1/strategies/1/enable

# Disable strategy
curl -X POST http://localhost:8080/api/v1/strategies/1/disable

# Get enabled strategies (supports multiple parameters)
curl http://localhost:8080/api/v1/strategies?status=enabled
curl http://localhost:8080/api/v1/strategies?status=true

# Get disabled strategies (supports multiple parameters)
curl http://localhost:8080/api/v1/strategies?status=disabled
curl http://localhost:8080/api/v1/strategies?status=false

# Update strategy status
curl -X PUT http://localhost:8080/api/v1/strategies/1 \
  -H "Content-Type: application/json" \
  -d '{"status": false}'
```

## AiGateway Mock Service

The project includes an AiGateway mock service located in the `scripts/aigateway-mock/` directory that provides the following endpoints:

- `POST /v1/chat/completions/quota/refresh` - Refresh quota
- `GET /v1/chat/completions/quota` - Query quota
- `POST /v1/chat/completions/quota/delta` - Increase/decrease quota

The mock service runs on port 1002.

## Configuration

### Configuration File (config.yaml)

```yaml
database:
  host: "pg"              # Database host
  port: 1001              # Database port
  user: "postgres"        # Database user
  password: "password"    # Database password
```

## Development Notes

### Adding New Condition Functions

1. Add new expression structure in `internal/condition/parser.go`
2. Implement `Evaluate` method
3. Add parsing logic in `buildFunction` method

### Extending Strategy Types

1. Add new type handling in `ExecStrategy` method in `internal/services/strategy.go`
2. Update data model and validation logic

## Testing

### Data Generation Script

The project includes complete test data generation script located in `scripts/generate_data.go` that creates:

- 20 test users (including different VIP levels, organizations, GitHub stars, etc.)
- 7 test strategies (including various conditions and types, some enabled, some disabled)

### API Testing Script

Additionally, there is an API testing script `scripts/test_api.sh` for testing the API endpoints.

### Integration Tests

The project provides a complete integration test suite located in the `test/` directory, including the following features:

#### Test Coverage
- **Condition Expression Tests**: Covers all supported condition functions and nested logic
- **Strategy Type Tests**: One-time and periodic recharge strategies
- **Strategy Status Tests**: Enable/disable status control
- **AiGateway Integration Tests**: Normal requests and failure handling
- **Batch Processing Tests**: Multi-user batch operation verification

#### Running Integration Tests

```bash
# Enter test directory
cd test

# Run using script (recommended)
chmod +x run_tests.sh
./run_tests.sh

# Or run manually
go run integration_main.go
```

#### Environment Configuration

Tests support the following environment variable configuration:
- `POSTGRES_HOST`: Database host (default: localhost)
- `POSTGRES_PORT`: Database port (default: 5432)
- `POSTGRES_USER`: Database user (default: postgres)
- `POSTGRES_PASSWORD`: Database password (default: password)
- `POSTGRES_DB`: Database name (default: quota_manager)

For detailed test instructions, please refer to [test/README.md](test/README.md).

## Logging

System uses zap logging library, logging format is JSON, containing the following information:

- Strategy execution status
- Strategy status change
- User recharge record
- Error information
- System status

## Troubleshooting

### Common Issues

1. **Database Connection Failure**
   - Check if PostgreSQL service is running
   - Verify database connection information in config.yaml

2. **AiGateway Connection Failure**
   - Ensure AiGateway mock service is running
   - Check if port is occupied

3. **Strategy Not Executing**
   - Check if strategy status is `true` (enabled)
   - Check if cron expression is correct
   - Verify condition expression syntax
   - Check logs for detailed error information

### Debug Mode

Set environment variable to enable debug mode:
```bash
export GIN_MODE=debug
```

## License

MIT License