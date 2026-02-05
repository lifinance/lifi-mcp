# Agent Guidelines for LiFi MCP Server

## Build & Test Commands
```bash
go build .                    # Build the main binary
go test ./... -v             # Run all tests (currently no tests)
go test ./... -run TestName  # Run a single test by name
go mod tidy                  # Clean up dependencies
gofmt -w .                   # Format all Go files
go vet ./...                 # Run static analysis
```

## Code Style Guidelines
- **Go Version**: 1.24.0+ required, using mcp-go v0.39.1+
- **Imports**: Group stdlib, external deps, then local packages with blank lines
- **Error Handling**: Always return errors up the call stack, wrap with context using fmt.Errorf
- **Naming**: Use camelCase for functions/variables, PascalCase for exported types/funcs
- **Constants**: Define at package level, use UPPER_SNAKE_CASE for ABI definitions
- **API Keys**: Never log or expose API keys; passed per-request via context, never stored in server state
- **HTTP Client**: Use standard http.Client with context for API calls to li.quest
- **Ethereum**: Use go-ethereum for blockchain interactions, validate addresses with common.IsHexAddress
- **MCP Server**: Use mcp.ParseString/ParseInt for args, return (*mcp.CallToolResult, error) from handlers