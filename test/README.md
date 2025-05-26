# Quota Manager Integration Tests

This directory contains the complete integration test suite for the Quota Manager.

## Test Coverage

### Condition Expression Tests
- ✅ Empty condition expression
- ✅ `match-user(user)` - Match specific user
- ✅ `register-before(timestamp)` - Registration time before specified time
- ✅ `access-after(timestamp)` - Last access time after specified time
- ✅ `github-star(project)` - Whether starred the specified project
- ✅ `quota-le(model, amount)` - Quota balance less than or equal to specified amount
- ✅ `is-vip(level)` - VIP level greater than or equal to specified level
- ✅ `belong-to(org)` - Belongs to specified organization
- ✅ `and(condition1, condition2)` - Logical AND
- ✅ `or(condition1, condition2)` - Logical OR
- ✅ `not(condition)` - Logical NOT
- ✅ Complex nested condition expressions

### Strategy Type Tests
- ✅ Single recharge strategy (single) - Execute once per user
- ✅ Periodic recharge strategy (periodic) - Can be executed repeatedly

### Strategy Status Tests
- ✅ Enabled strategy execution
- ✅ Disabled strategy non-execution
- ✅ Dynamic enable/disable strategy

### AiGateway Integration Tests
- ✅ Normal request handling
- ✅ Request failure handling and error status recording

### Batch Processing Tests
- ✅ Multi-user batch processing
- ✅ Condition filtering and execution verification

## Quick Start

### Prerequisites

1. **Go 1.21+**
2. **PostgreSQL 12+**
3. **Database Configuration** - Ensure PostgreSQL is running and accessible

### Running Tests

1. **Using Script (Recommended)**
   ```bash
   cd test
   chmod +x run_tests.sh
   ./run_tests.sh
   ```

2. **Manual Run**
   ```bash
   # Set environment variables (optional)
   export POSTGRES_HOST=localhost
   export POSTGRES_PORT=5432
   export POSTGRES_USER=postgres
   export POSTGRES_PASSWORD=password
   export POSTGRES_DB=quota_manager

   # Enter test directory
   cd test

   # Run tests
   go run integration_main.go
   ```

### Environment Variable Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| POSTGRES_HOST | localhost | PostgreSQL host address |
| POSTGRES_PORT | 5432 | PostgreSQL port |
| POSTGRES_USER | postgres | Database username |
| POSTGRES_PASSWORD | password | Database password |
| POSTGRES_DB | quota_manager | Database name |

## Test Architecture

### Test Context (TestContext)
- **DB**: Database connection
- **StrategyService**: Strategy service instance
- **Gateway**: AiGateway client
- **MockServer**: Successful mock server
- **FailServer**: Failure mock server

### Mock Services
Tests use built-in HTTP mock servers to simulate AiGateway behavior:
- **Success Server**: Simulates normal API responses
- **Failure Server**: Simulates API failure scenarios

### Test Data Management
- Clear database before each test
- Create specific test users and strategies
- Independent quota storage simulation

## Test Process

Each test case follows this process:

1. **Clear Data** - Ensure clean test environment
2. **Configure Test Data** - Create necessary users and strategies
3. **Trigger Strategy Execution** - Call strategy service to execute strategy
4. **Check Execution Results** - Verify database records and status

## Output Example

```
=== Quota Manager Integration Tests ===
Running test: Clear Data Test
✅ Clear Data Test - Passed (0.05s)
Running test: Condition Expression-Empty Condition Test
✅ Condition Expression-Empty Condition Test - Passed (0.03s)
Running test: Condition Expression-Match User Test
✅ Condition Expression-Match User Test - Passed (0.02s)
...

=== Test Results Summary ===
Total tests: 18
Passed tests: 18
Failed tests: 0
Total duration: 2.45s
Success rate: 100.0%

🎉 All tests passed!
```

## Troubleshooting

### Common Issues

1. **Database Connection Failure**
   - Check if PostgreSQL is running
   - Verify environment variable settings
   - Check database user permissions

2. **Port Conflicts**
   - Tests use dynamic port allocation, usually no conflicts
   - If issues occur, restart the test

3. **Dependency Issues**
   - Run `go mod tidy` to update dependencies
   - Check if Go version meets requirements

### Debug Mode

Add more log output in test code to debug issues:

```go
// Add debug information in test functions
fmt.Printf("Debug info: %+v\n", result)
```

## Extending Tests

### Adding New Condition Expression Tests

1. Add new test function in `integration_main.go`
2. Create test data following existing patterns
3. Execute strategy and verify results
4. Register new test in `runAllTests`

### Adding New Strategy Type Tests

1. Create corresponding test strategy
2. Verify specific execution logic
3. Check database state changes

## Performance Considerations

- Tests use short-interval periodic strategies (every minute) to reduce test time
- Batch tests limit user count (10) to maintain test speed
- Mock servers run in memory to avoid network latency