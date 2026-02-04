# LiFi MCP Server

> ⚠️ **IMPORTANT SECURITY DISCLAIMER** ⚠️
> 
> **DO NOT use this tool with your main wallet keystore or wallets containing significant funds!**
> 
> This tool is for testing and experimental purposes only. There is potential for loss of funds due to:
> - Software bugs or security vulnerabilities
> - Transaction errors or misconfigurations
> - Network issues or smart contract failures
> 
> Neither LI.FI nor the developers of this tool are responsible for any loss of funds resulting from the use of this freely available open-source software.
> 
> **Use at your own risk with test wallets only.**

This MCP server integrates with the [LI.FI API](https://li.quest) to provide cross-chain swap functionality across multiple liquidity pools and bridges via the Model Context Protocol (MCP).

The Model Context Protocol (MCP) is a protocol for AI model integration, allowing AI models to access external tools and data sources.

## Components

### Tools

#### Token Information

- **get-tokens** - Retrieve all tokens supported by LI.FI
  - Use to discover available tokens before swaps
  - Parameters: `chains` (e.g., "1,137"), `chainTypes` (e.g., "EVM,SVM"), `minPriceUSD`

- **get-token** - Get details about a specific token
  - Parameters: `chain` (required, e.g., "1" or "ethereum"), `token` (required, address or symbol)

#### Chain Information

- **get-chains** - List all supported blockchain networks
  - Returns chain IDs, names, RPC URLs, block explorers
  - Parameters: `chainTypes` (e.g., "EVM")

- **get-chain-by-id** - Look up chain by numeric ID
  - Parameters: `id` (required, e.g., "1" for Ethereum, "137" for Polygon)

- **get-chain-by-name** - Look up chain by name (case-insensitive)
  - Parameters: `name` (required, e.g., "ethereum", "polygon", "arbitrum")

#### Quote & Swap (Primary Workflow)

- **get-quote** ⭐ - Get the best route for a swap (PRIMARY TOOL)
  - Returns route, fees, estimated time, and `transactionRequest` for execution
  - Required: `fromChain`, `toChain`, `fromToken`, `toToken`, `fromAddress`, `fromAmount`
  - Optional: `toAddress`, `slippage` (e.g., "0.03" for 3%), `order` (RECOMMENDED/FASTEST/CHEAPEST/SAFEST)
  - Optional filters: `allowBridges`, `allowExchanges`

- **get-status** - Track cross-chain transfer progress
  - Parameters: `txHash` (required), `bridge`, `fromChain`, `toChain`

- **get-routes** - Get multiple route options for comparison
  - Unlike get-quote, returns several alternatives to choose from
  - Use with `get-step-transaction` to execute a specific route
  - Required: `fromChainId`, `toChainId`, `fromTokenAddress`, `toTokenAddress`, `fromAddress`, `fromAmount`

- **get-step-transaction** - Convert a route step to executable transaction
  - Parameters: `step` (required, object from get-routes response)

- **get-quote-with-calls** - Quote with custom contract calls (Zaps)
  - Execute complex DeFi operations: bridge + deposit/stake in one transaction
  - Required: `fromChain`, `toChain`, `fromToken`, `toToken`, `fromAddress`, `fromAmount`, `contractCalls`

#### Discovery & Routing

- **get-connections** - Check available swap routes between chains
  - Use to verify if a route exists before calling get-quote
  - Parameters: `fromChain`, `toChain`, `fromToken`, `toToken`, `chainTypes`, `allowBridges`

- **get-tools** - List available bridges and DEXes
  - Returns keys (for API calls) and names (human-readable)
  - Parameters: `chains` (array of chain IDs to filter)

#### Gas Information

- **get-gas-prices** - Current gas prices for all supported chains
  - Returns fast/standard/slow prices in gwei

- **get-gas-suggestion** - Detailed gas parameters for one chain
  - Parameters: `chainId` (required)

#### API Key Testing

- **test-api-key** - Verify API key is valid
  - Returns key status and rate limit information
  - Requires API key configured via `LIFI_API_KEY` env var or `--api-key` flag

#### Balance & Allowance Queries

- **get-native-token-balance** - Check ETH/MATIC/etc. balance
  - Parameters: `chain` (required, e.g., "1" or "ethereum"), `address` (required), `rpcUrl` (optional override)

- **get-token-balance** - Check ERC20 token balance
  - Parameters: `chain` (required), `tokenAddress`, `walletAddress` (required), `rpcUrl` (optional)

- **get-allowance** - Check token spending approval
  - **Important:** Verify allowance before swaps; if insufficient, call approve-token
  - Parameters: `chain` (required), `tokenAddress`, `ownerAddress`, `spenderAddress` (required), `rpcUrl` (optional)

#### Transaction Operations (Keystore Required)

> **Note:** These tools require the `--keystore` flag at startup.

- **get-wallet-address** - Get address of loaded wallet

- **execute-quote** - Sign and broadcast a swap transaction
  - **Workflow:** get-quote → get-allowance → approve-token (if needed) → execute-quote
  - Parameters: `chain` (required), `transactionRequest` (required, from get-quote), `rpcUrl` (optional)

- **approve-token** - Approve ERC20 spending for swaps
  - Required before swapping ERC20 tokens (not needed for native tokens)
  - Parameters: `chain` (required), `tokenAddress`, `spenderAddress`, `amount` (required), `rpcUrl` (optional)

- **transfer-token** - Direct ERC20 transfer (no swap)
  - Parameters: `chain` (required), `tokenAddress`, `to`, `amount` (required), `rpcUrl` (optional)

- **transfer-native** - Direct native token transfer
  - Parameters: `chain` (required), `to`, `amount` (in wei, required), `rpcUrl` (optional)

### Common Chain IDs

| Chain | ID | Native Token |
|-------|-----|--------------|
| Ethereum | 1 | ETH |
| Polygon | 137 | MATIC |
| Arbitrum | 42161 | ETH |
| Optimism | 10 | ETH |
| BSC | 56 | BNB |
| Avalanche | 43114 | AVAX |
| Base | 8453 | ETH |

### Example Workflow: Cross-Chain Swap

```
1. get-chains                    # Find RPC URLs and chain IDs
2. get-token (chain, symbol)     # Get token addresses
3. get-quote (...)               # Get best route and transactionRequest
4. get-allowance (...)           # Check if approval needed
5. approve-token (...)           # If allowance < amount
6. execute-quote (...)           # Execute the swap
7. get-status (txHash)           # Track cross-chain progress
```

## Getting Started

### Installation

#### Using Go Install

```bash
go install github.com/lifinance/lifi-mcp@latest
```

### Usage

Start the MCP server:

```bash
lifi-mcp
```

With keystore for transaction capabilities:

```bash
lifi-mcp --keystore <keystore-name> --password <keystore-password>
```

Check the version:

```bash
lifi-mcp --version
```

### API Key Configuration

For higher rate limits (200 req/min vs 200 req/2hr), configure a LI.FI API key:

#### Environment Variable (Recommended)

```bash
export LIFI_API_KEY=your_api_key
./lifi-mcp
```

#### Command-Line Flag

```bash
./lifi-mcp --api-key=your_api_key
```

The `--api-key` flag overrides the environment variable if both are set.

#### Testing Your API Key

Use the `test-api-key` tool to verify your key is valid.

### Testing with MCP Inspector

Use the [MCP Inspector](https://github.com/modelcontextprotocol/inspector) to interactively test the server:

```bash
# Build the server
go build .

# Launch with MCP Inspector (without API key)
npx @modelcontextprotocol/inspector ./lifi-mcp

# Launch with API key
npx @modelcontextprotocol/inspector -e LIFI_API_KEY=your_key ./lifi-mcp
```

Opens a web UI at http://localhost:6274 where you can browse and test all available tools.

### Using as a Package

You can import the server in your Go projects:

#### Stdio Mode

```go
import "github.com/lifinance/lifi-mcp/server"

func main() {
    // Create a new server with version and optional API key
    s := server.NewServer("1.0.0", "your_api_key") // or "" for no API key

    // Start the server in stdio mode
    if err := s.ServeStdio(); err != nil {
        log.Fatalf("Server error: %v", err)
    }
}
```

#### In-Process Mode

For in-process usage with the mcp-go client library:

```go
import (
    "context"
    "log"
    
    "github.com/mark3labs/mcp-go/client"
    "github.com/mark3labs/mcp-go/client/transport"
    "github.com/lifinance/lifi-mcp/server"
)

func main() {
    // Create the LiFi MCP server with optional API key
    lifiServer := server.NewServer("1.0.0", "") // or "your_api_key" for higher rate limits

    // Create an in-process transport using the server's MCPServer
    inProcessTransport := transport.NewInProcessTransport(lifiServer.GetMCPServer())

    // Create an MCP client using the in-process transport
    mcpClient := client.NewMCPClient(inProcessTransport)

    // Start the transport
    ctx := context.Background()
    if err := mcpClient.Connect(ctx); err != nil {
        log.Fatalf("Failed to connect: %v", err)
    }
    defer mcpClient.Close()

    // Initialize the client
    if err := mcpClient.Initialize(ctx); err != nil {
        log.Fatalf("Failed to initialize: %v", err)
    }

    // List available tools
    tools, err := mcpClient.ListTools(ctx)
    if err != nil {
        log.Fatalf("Failed to list tools: %v", err)
    }

    // Use the tools...
    result, err := mcpClient.CallTool(ctx, "get-chain-by-name", map[string]any{
        "name": "ethereum",
    })
    if err != nil {
        log.Fatalf("Failed to call tool: %v", err)
    }
}
```

### Wallet Management

The server will search for keystore files in the standard Ethereum keystore directory:
- Linux: ~/.ethereum/keystore
- macOS: ~/Library/Ethereum/keystore
- Windows: %APPDATA%\Ethereum\keystore

### Usage with Model Context Protocol

To integrate this server with apps that support MCP:

```json
{
  "mcpServers": {
    "lifi": {
      "command": "lifi-mcp",
      "args": []
    }
  }
}
```

With keystore for transaction capabilities:

```json
{
  "mcpServers": {
    "lifi": {
      "command": "lifi-mcp",
      "args": ["--keystore", "your-keystore", "--password", "your-password"]
    }
  }
}
```

## License

MIT
