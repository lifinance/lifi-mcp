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
- **get-tokens**
  - Fetch all tokens known to the LI.FI services
  - Parameters: `chains`, `chainTypes`, `minPriceUSD`
  
- **get-token**
  - Get detailed information about a specific token
  - Parameters: `chain` (required), `token` (required)

#### Chain Information
- **get-chains**
  - Get information about all supported chains
  - Parameters: `chainTypes`
  
- **get-chain-by-id**
  - Find a chain by its numeric ID
  - Parameters: `id` (required)
  
- **get-chain-by-name**
  - Find a chain by name, key, or ID (case insensitive)
  - Parameters: `name` (required)

#### Cross-Chain Operations
- **get-quote**
  - Get a quote for a token transfer (cross-chain or same-chain)
  - Parameters: `fromChain`, `toChain`, `fromToken`, `toToken`, `fromAddress`, `fromAmount`, etc.
  
- **get-status**
  - Check the status of a cross-chain transfer
  - Parameters: `txHash` (required), `bridge`, `fromChain`, `toChain`
  
- **get-connections**
  - Returns all possible connections between chains
  - Parameters: `fromChain`, `toChain`, `fromToken`, `toToken`, `chainTypes`
  
- **get-tools**
  - Get available bridges and exchanges
  - Parameters: `chains`

#### Wallet Operations
- **get-wallet-address**
  - Get the Ethereum address for the loaded private key
  
- **get-native-token-balance**
  - Get the native token balance of a wallet
  - Parameters: `rpcUrl` (required), `address` (required)
  
- **get-token-balance**
  - Get the balance of a specific ERC20 token for a wallet
  - Parameters: `rpcUrl`, `tokenAddress`, `walletAddress`
  
- **get-allowance**
  - Check the allowance of an ERC20 token for a specific spender
  - Parameters: `rpcUrl`, `tokenAddress`, `ownerAddress`, `spenderAddress`

#### Transaction Operations (Keystore Required)

> **Note:** These tools are only available when the server is started with the `--keystore` flag.

- **execute-quote**
  - Execute a quote transaction using the stored private key
  - Parameters: `rpcUrl`, `transactionRequest`
  
- **approve-token**
  - Approve a specific amount of ERC20 tokens to be spent by another address
  - Parameters: `rpcUrl`, `tokenAddress`, `spenderAddress`, `amount`
  
- **transfer-token**
  - Transfer ERC20 tokens to another address
  - Parameters: `rpcUrl`, `tokenAddress`, `to`, `amount`
  
- **transfer-native**
  - Transfer native cryptocurrency to another address
  - Parameters: `rpcUrl`, `to`, `amount`

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

### Using as a Package

You can import the server in your Go projects:

#### Stdio Mode

```go
import "github.com/lifinance/lifi-mcp/server"

func main() {
    // Create a new server with version
    s := server.NewServer("1.0.0")
    
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
    // Create the LiFi MCP server
    lifiServer := server.NewServer("1.0.0")

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

### Docker

#### Running with Docker

You can run the LiFi MCP server using Docker:

```bash
docker run -i --rm ghcr.io/lifinance/lifi-mcp:latest
```

#### Docker Configuration with MCP

To integrate the Docker image with apps that support MCP:

```json
{
  "mcpServers": {
    "lifi": {
      "command": "docker",
      "args": [
        "run",
        "-i",
        "--rm",
        "ghcr.io/lifinance/lifi-mcp:latest"
      ]
    }
  }
}
```

## License

MIT
