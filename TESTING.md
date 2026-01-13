# Testing Documentation

## Mock OpenWRT Factory Reset Implementation

This project now includes a comprehensive mock OpenWRT implementation for testing provisioning without requiring actual hardware.

### Mock Client Features

The `MockClient` in [internal/ssh/mock.go](internal/ssh/mock.go) simulates a factory reset OpenWRT device with:

- **Factory reset state**: Includes default packages like `firewall4`, `dnsmasq`, `dropbear`, etc.
- **UCI command simulation**: Handles `uci set`, `uci add_list`, `uci commit`
- **Package management**: Simulates `opkg install` and `opkg remove`
- **Board.json generation**: Returns realistic board.json for different device models
- **Command tracking**: Records all executed commands for verification
- **Failure simulation**: Can be configured to fail on specific commands

### Running Tests

Run all tests:
```bash
go test ./...
```

Run provision tests with verbose output:
```bash
go test -v ./internal/provision/
```

### Test Coverage

The test suite in [internal/provision/provision_test.go](internal/provision/provision_test.go) includes:

1. **TestFactoryResetProvisionBasic**: Tests basic system configuration provisioning
2. **TestFactoryResetWithPackages**: Tests package installation and removal
3. **TestFactoryResetWithConditionals**: Tests conditional configuration based on device tags
4. **TestFactoryResetVerifyDevice**: Tests device model verification
5. **TestFactoryResetCommandFailure**: Tests error handling and rollback
6. **TestFactoryResetMultipleDevices**: Tests device-specific configuration
7. **TestFactoryResetBoardJSON**: Tests board.json parsing for multiple device models

### Example: Using the Mock Client

```go
// Create a mock client simulating a factory reset EdgeRouter X
mockClient := ssh.NewMockClient("ubnt,edgerouter-x")

// Execute commands against the mock
output, err := mockClient.Execute("cat /etc/board.json")

// Verify UCI state after provisioning
hostname := mockClient.GetUCIValue("system", "system", "hostname")

// Check executed commands
commands := mockClient.GetExecutedCommands()
```

### SSH Executor Interface

Both real SSH clients and mock clients implement the `ssh.SSHExecutor` interface:

```go
type SSHExecutor interface {
    Execute(command string) (string, error)
    ExecuteWithError(command string) (string, error)
    Close() error
}
```

This allows seamless substitution of mock clients in tests without modifying production code.

## Benefits

- **No hardware required**: Test provisioning logic without physical devices
- **Fast execution**: Tests complete in milliseconds
- **Deterministic**: No network issues or timing dependencies
- **Easy debugging**: Full visibility into executed commands and state changes
- **Comprehensive coverage**: Test error conditions that are hard to reproduce on real hardware
