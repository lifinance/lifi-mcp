# LiFi MCP Server

> **Note:** This server provides **read-only** tools — it does not sign or broadcast transactions. Quote responses include unsigned `transactionRequest` objects that must be signed and submitted externally using your own wallet. Neither LI.FI nor the developers of this tool are responsible for any loss of funds resulting from the use of this freely available open-source software.

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
  - API key must be provided via `Authorization: Bearer` or `X-LiFi-Api-Key` header

#### Health Check

- **health-check** - Check server health and version
  - Returns server status and version information

#### Balance & Allowance Queries

- **get-native-token-balance** - Check ETH/MATIC/etc. balance
  - Parameters: `chain` (required, e.g., "1" or "ethereum"), `address` (required), `rpcUrl` (optional override)

- **get-token-balance** - Check ERC20 token balance
  - Parameters: `chain` (required), `tokenAddress`, `walletAddress` (required), `rpcUrl` (optional)

- **get-allowance** - Check token spending approval
  - **Important:** Verify allowance before swaps; if insufficient, approve tokens using your wallet
  - Parameters: `chain` (required), `tokenAddress`, `ownerAddress`, `spenderAddress` (required), `rpcUrl` (optional)

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
5. (external) Approve tokens using your wallet if allowance < amount
6. (external) Sign and broadcast transactionRequest using your wallet
7. get-status (txHash)           # Track cross-chain progress
```

## Example Prompts & Responses

These examples demonstrate actual tool responses from the LI.FI MCP server. Use them as reference for understanding the data formats and building integrations.

<details>
<summary><strong>🔗 Chain Information</strong></summary>

### Get Chain by Name

**Prompt:** "What are the details for Ethereum?"

**Tool:** `get-chain-by-name` with `name: "ethereum"`

**Response:**
```json
{
  "id": 1,
  "key": "eth",
  "name": "Ethereum",
  "nativeToken": {
    "address": "0x0000000000000000000000000000000000000000",
    "symbol": "ETH",
    "decimals": 18,
    "name": "ETH"
  },
  "metamask": {
    "chainId": "0x1",
    "blockExplorerUrls": ["https://etherscan.io/"],
    "chainName": "Ethereum Mainnet",
    "rpcUrls": ["https://ethereum-rpc.publicnode.com", "https://eth.drpc.org"]
  }
}
```

### Get Chain by ID

**Prompt:** "Look up chain ID 8453"

**Tool:** `get-chain-by-id` with `id: "8453"`

**Response:**
```json
{
  "id": 8453,
  "key": "bas",
  "name": "Base",
  "nativeToken": {
    "address": "0x0000000000000000000000000000000000000000",
    "symbol": "ETH",
    "decimals": 18,
    "name": "ETH"
  },
  "metamask": {
    "chainId": "0x2105",
    "blockExplorerUrls": ["https://basescan.org/"],
    "chainName": "Base",
    "rpcUrls": ["https://mainnet.base.org", "https://base-rpc.publicnode.com"]
  }
}
```

### Get Solana Chain Info

**Prompt:** "Get Solana chain details"

**Tool:** `get-chain-by-name` with `name: "solana"`

**Response:**
```json
{
  "id": 1151111081099710,
  "key": "sol",
  "name": "Solana",
  "nativeToken": {
    "address": "11111111111111111111111111111111",
    "symbol": "SOL",
    "decimals": 9,
    "name": "SOL"
  },
  "metamask": {
    "chainId": "1151111081099710",
    "blockExplorerUrls": ["https://solscan.io/", "https://solana.fm/", "https://explorer.solana.com/"],
    "chainName": "Solana",
    "rpcUrls": ["https://api.mainnet-beta.solana.com", "https://solana-rpc.publicnode.com"]
  }
}
```

</details>

<details>
<summary><strong>🪙 Token Information</strong></summary>

### Get Token Details

**Prompt:** "Get USDC token info on Ethereum"

**Tool:** `get-token` with `chain: "1"`, `token: "USDC"`

**Response:**
```json
{
  "address": "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48",
  "chainId": 1,
  "symbol": "USDC",
  "decimals": 6,
  "name": "USD Coin",
  "coinKey": "USDC",
  "logoURI": "https://raw.githubusercontent.com/trustwallet/assets/master/blockchains/ethereum/assets/0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48/logo.png",
  "priceUSD": "0.99969",
  "marketCapUSD": 70829343624,
  "volumeUSD24H": 19781184506,
  "tags": ["stablecoin"]
}
```

### Get Native Token (ETH)

**Prompt:** "Get ETH token info on Ethereum"

**Tool:** `get-token` with `chain: "1"`, `token: "0x0000000000000000000000000000000000000000"`

**Response:**
```json
{
  "address": "0x0000000000000000000000000000000000000000",
  "chainId": 1,
  "symbol": "ETH",
  "decimals": 18,
  "name": "ETH",
  "coinKey": "ETH",
  "logoURI": "https://raw.githubusercontent.com/trustwallet/assets/master/blockchains/ethereum/assets/0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2/logo.png",
  "priceUSD": "2237.76",
  "marketCapUSD": 269965295371,
  "volumeUSD24H": 43694691058,
  "tags": ["major_asset"]
}
```

</details>

<details>
<summary><strong>⛽ Gas Information</strong></summary>

### Get Gas Suggestion for a Chain

**Prompt:** "What's the current gas price on Ethereum?"

**Tool:** `get-gas-suggestion` with `chainId: "1"`

**Response:**
```json
{
  "recommended": {
    "token": {
      "address": "0x0000000000000000000000000000000000000000",
      "chainId": 1,
      "symbol": "ETH",
      "decimals": 18,
      "priceUSD": "2237.76"
    },
    "amount": "121970296389612",
    "amountUsd": "0.2744"
  },
  "limit": {
    "token": {
      "address": "0x0000000000000000000000000000000000000000",
      "chainId": 1,
      "symbol": "ETH",
      "decimals": 18,
      "priceUSD": "2237.76"
    },
    "amount": "22224592845459071",
    "amountUsd": "50.0000"
  },
  "available": true,
  "fromAmount": "0"
}
```

> **Note:** The `recommended.amountUsd` shows typical transaction cost (~$0.27), while `limit.amountUsd` shows maximum gas budget ($50).

</details>

<details>
<summary><strong>🔧 Discovery Tools</strong></summary>

### List Available Bridges and DEXes

**Prompt:** "What bridges and exchanges are available?"

**Tool:** `get-tools`

**Response (truncated):**
```json
{
  "bridges": [
    { "key": "across", "name": "AcrossV4" },
    { "key": "arbitrum", "name": "Arbitrum Bridge" },
    { "key": "cbridge", "name": "Celer cBridge" },
    { "key": "stargateV2", "name": "StargateV2 (Fast mode)" },
    { "key": "chainflip", "name": "Chainflip" },
    { "key": "near", "name": "NearIntents" },
    { "key": "lifiIntents", "name": "LI.FI Intents" }
  ],
  "exchanges": [
    { "key": "1inch", "name": "1inch" },
    { "key": "paraswap", "name": "Velora" },
    { "key": "odos", "name": "Odos" },
    { "key": "sushiswap", "name": "SushiSwap Aggregator" },
    { "key": "jupiter", "name": "Jupiter" },
    { "key": "kyberswap", "name": "Kyberswap" }
  ]
}
```

> **Usage Tip:** Use the `key` values in `get-quote` with `allowBridges` or `allowExchanges` to filter routes.

</details>

<details>
<summary><strong>💱 Quotes & Routing</strong></summary>

### Same-Chain Swap Quote (ETH → USDC on Ethereum)

**Prompt:** "Get a quote to swap 0.01 ETH to USDC on Ethereum"

**Tool:** `get-quote` with:
```json
{
  "fromChain": "1",
  "toChain": "1",
  "fromToken": "0x0000000000000000000000000000000000000000",
  "toToken": "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48",
  "fromAddress": "0xd8dA6BF26964aF9D7eEd9e03E53415D37aA96045",
  "fromAmount": "10000000000000000",
  "slippage": "0.03"
}
```

**Response (key fields):**
```json
{
  "type": "lifi",
  "tool": "sushiswap",
  "toolDetails": {
    "key": "sushiswap",
    "name": "SushiSwap Aggregator"
  },
  "action": {
    "fromToken": { "symbol": "ETH", "decimals": 18, "priceUSD": "2249.76" },
    "fromAmount": "10000000000000000",
    "toToken": { "symbol": "USDC", "decimals": 6, "priceUSD": "0.99969" },
    "slippage": 0.03
  },
  "estimate": {
    "toAmount": "22392397",
    "toAmountMin": "21720625",
    "fromAmountUSD": "22.4976",
    "toAmountUSD": "22.3855",
    "gasCosts": [{
      "amount": "66216217693968",
      "amountUSD": "0.1490",
      "token": { "symbol": "ETH" }
    }],
    "feeCosts": [{
      "name": "LIFI Fixed Fee",
      "amountUSD": "0.0562",
      "percentage": "0.0025"
    }]
  },
  "transactionRequest": {
    "to": "0x1231DEB6f5749EF6cE6943a275A1D3E7486F4EaE",
    "data": "0x736eac0b5a28cb9d...", // Truncated - full calldata in actual response
    "value": "0x2386f26fc10000",
    "from": "0xd8dA6BF26964aF9D7eEd9e03E53415D37aA96045",
    "chainId": 1,
    "gasPrice": "0xa7a9b10",
    "gasLimit": "0x778a1"
  }
}
```

> **Key Points:**
> - `estimate.toAmount`: Expected USDC output (22.39 USDC with 6 decimals)
> - `estimate.toAmountMin`: Minimum after slippage (21.72 USDC)
> - `transactionRequest.data`: Encoded calldata for the swap contract (required for execution)

### Cross-Chain Swap Quote (ETH on Ethereum → USDC on Base)

**Prompt:** "Bridge 0.01 ETH from Ethereum to USDC on Base"

**Tool:** `get-quote` with:
```json
{
  "fromChain": "1",
  "toChain": "8453",
  "fromToken": "0x0000000000000000000000000000000000000000",
  "toToken": "0x833589fCD6eDb6E08f4c7C32D4f71b54bdA02913",
  "fromAddress": "0xd8dA6BF26964aF9D7eEd9e03E53415D37aA96045",
  "fromAmount": "10000000000000000",
  "slippage": "0.03"
}
```

**Response (key fields):**
```json
{
  "type": "lifi",
  "tool": "near",
  "toolDetails": {
    "key": "near",
    "name": "NearIntents"
  },
  "action": {
    "fromChainId": 1,
    "toChainId": 8453,
    "fromToken": { "symbol": "ETH", "chainId": 1 },
    "toToken": { "symbol": "USDC", "chainId": 8453 }
  },
  "estimate": {
    "toAmount": "22293079",
    "toAmountMin": "21624286",
    "fromAmountUSD": "22.4976",
    "toAmountUSD": "22.2862",
    "executionDuration": 47,
    "gasCosts": [{
      "amountUSD": "0.0797",
      "token": { "symbol": "ETH" }
    }],
    "feeCosts": [
      { "name": "LIFI Fixed Fee", "amountUSD": "0.0562" },
      { "name": "NearIntents Protocol Fee", "amountUSD": "0.0022" }
    ]
  },
  "includedSteps": [
    { "type": "protocol", "tool": "feeCollection" },
    { "type": "cross", "tool": "near" }
  ],
  "transactionRequest": {
    "to": "0x1231DEB6f5749EF6cE6943a275A1D3E7486F4EaE",
    "data": "0x3110c7b900000000...", // Truncated - full calldata in actual response
    "value": "0x2386f26fc10000",
    "from": "0xd8dA6BF26964aF9D7eEd9e03E53415D37aA96045",
    "chainId": 1,
    "gasPrice": "0xa7a9b10",
    "gasLimit": "0x3fee7"
  }
}
```

> **Key Points:**
> - `executionDuration`: ~47 seconds for cross-chain transfer
> - `includedSteps`: Shows the route (fee collection → bridge via NearIntents)
> - `transactionRequest.data`: Essential calldata encoding the bridge parameters

</details>

<details>
<summary><strong>💰 Balance Queries</strong></summary>

### Check Native Token Balance

**Prompt:** "What's the ETH balance of vitalik.eth?"

**Tool:** `get-native-token-balance` with `chain: "1"`, `address: "0xd8dA6BF26964aF9D7eEd9e03E53415D37aA96045"`

**Response:**
```json
{
  "address": "0xd8dA6BF26964aF9D7eEd9e03E53415D37aA96045",
  "balance": "32115625288281011210",
  "chainId": "1",
  "decimals": 18,
  "tokenSymbol": "ETH"
}
```

> **Conversion:** 32115625288281011210 ÷ 10^18 = **~32.12 ETH**

### Check ERC20 Token Balance

**Prompt:** "How much USDC does this address have?"

**Tool:** `get-token-balance` with:
```json
{
  "chain": "1",
  "tokenAddress": "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48",
  "walletAddress": "0xd8dA6BF26964aF9D7eEd9e03E53415D37aA96045"
}
```

**Response:**
```json
{
  "balance": "4145288996",
  "chainId": "1",
  "decimals": 6,
  "tokenAddress": "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48",
  "tokenSymbol": "USDC",
  "walletAddress": "0xd8dA6BF26964aF9D7eEd9e03E53415D37aA96045"
}
```

> **Conversion:** 4145288996 ÷ 10^6 = **~4,145.29 USDC**

</details>

<details>
<summary><strong>📋 Common Token Addresses</strong></summary>

| Token | Chain | Address |
|-------|-------|---------|
| ETH (native) | Ethereum (1) | `0x0000000000000000000000000000000000000000` |
| ETH (native) | Base (8453) | `0x0000000000000000000000000000000000000000` |
| USDC | Ethereum (1) | `0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48` |
| USDC | Base (8453) | `0x833589fCD6eDb6E08f4c7C32D4f71b54bdA02913` |
| USDC | Solana | `EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v` |
| SOL (native) | Solana | `11111111111111111111111111111111` |
| USDT | Ethereum (1) | `0xdAC17F958D2ee523a2206206994597C13D831ec7` |
| DAI | Ethereum (1) | `0x6B175474E89094C44Da98b954EesC511AD7D82b3` |

> **Tip:** Use `get-token` with the symbol (e.g., "USDC") to dynamically fetch the correct address for any chain.

</details>

## Getting Started

### Installation

#### Using Go Install

```bash
go install github.com/lifinance/lifi-mcp@latest
```

#### Using Docker

```bash
# Build the image
docker build -t lifi-mcp .

# Run the server
docker run -p 8080:8080 lifi-mcp
```

Or with Docker Compose:

```bash
docker compose up --build
```

Override defaults via environment variables:

```bash
PORT=9090 LOG_LEVEL=debug docker compose up --build
```

The Docker image uses a multi-stage build (golang:1.24-alpine → distroless) producing an ~11MB image. It runs as non-root (UID 65532) with no shell or package manager in the runtime image.

### Usage

Start the MCP server:

```bash
lifi-mcp --port 8080
```

Available flags:

```bash
lifi-mcp --port 8080        # HTTP server port (default: 8080)
lifi-mcp --host 0.0.0.0     # HTTP server host (default: 0.0.0.0)
lifi-mcp --log-level debug  # Log level: debug, info, warn, error (default: info)
lifi-mcp --version          # Show version information
```

### API Key Configuration

API keys are passed per-request via HTTP headers (not server-side configuration). This enables multi-tenant deployments where each client uses their own key.

Include your LI.FI API key in requests using either header:

- `Authorization: Bearer your_api_key`
- `X-LiFi-Api-Key: your_api_key`

Without an API key, the server uses the public rate limit (200 req/2hr). With an API key, you get higher rate limits (200 req/min).

Use the `test-api-key` tool to verify your key is valid.

### Testing with MCP Inspector

Use the [MCP Inspector](https://github.com/modelcontextprotocol/inspector) to interactively test the server:

```bash
# Build and start the server
go build . && ./lifi-mcp --port 8080

# In another terminal, launch MCP Inspector pointing at the HTTP endpoint
npx @modelcontextprotocol/inspector --url http://localhost:8080/mcp
```

Opens a web UI at http://localhost:6274 where you can browse and test all available tools.

### Using as a Package

You can import the server in your Go projects:

```go
import (
    "fmt"
    "log/slog"
    "os"
    "time"

    "github.com/lifinance/lifi-mcp/server"
    mcpserver "github.com/mark3labs/mcp-go/server"
)

func main() {
    logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))

    // Create the LiFi MCP server (API keys are per-request, not server-side)
    s := server.NewServer("1.0.0", logger)

    // Create a Streamable HTTP server
    httpServer := mcpserver.NewStreamableHTTPServer(
        s.GetMCPServer(),
        mcpserver.WithEndpointPath("/mcp"),
        mcpserver.WithHeartbeatInterval(30*time.Second),
        mcpserver.WithStateLess(true),
        mcpserver.WithHTTPContextFunc(server.ExtractAPIKeyFromRequest),
    )

    // Start serving
    if err := httpServer.Start("0.0.0.0:8080"); err != nil {
        logger.Error("Server error", "error", err)
        os.Exit(1)
    }
}
```

### Usage with Model Context Protocol

To integrate this server with apps that support MCP, point them at the Streamable HTTP endpoint:

```json
{
  "mcpServers": {
    "lifi": {
      "url": "http://localhost:8080/mcp"
    }
  }
}
```

With an API key for higher rate limits:

```json
{
  "mcpServers": {
    "lifi": {
      "url": "http://localhost:8080/mcp",
      "headers": {
        "X-LiFi-Api-Key": "your_api_key"
      }
    }
  }
}
```

## Deployment

### Container Details

| Property | Value |
|----------|-------|
| Base image | `gcr.io/distroless/static-debian12:nonroot` |
| Image size | ~11 MB |
| User | `nonroot` (UID 65532) |
| Exposed port | 8080 (configurable via `--port`) |
| MCP endpoint | `/mcp` (POST) |
| Graceful shutdown | 10 seconds (responds to SIGTERM) |

### Building

```bash
docker build -t lifi-mcp .
```

### Running

```bash
docker run -p 8080:8080 lifi-mcp
```

Override flags by appending arguments (the entrypoint is the binary, CMD provides defaults):

```bash
docker run -p 9090:9090 lifi-mcp --port 9090 --log-level debug
```

### Environment

The server requires **no environment variables, secrets, or config files**. API keys are passed per-request by clients via `Authorization: Bearer <key>` or `X-LiFi-Api-Key: <key>` headers — nothing is stored server-side.

### Health Checking

The server doesn't expose a dedicated HTTP health endpoint. For orchestrator health checks:

- **TCP check** on port 8080 — confirms the server is listening
- **MCP call** — send an `initialize` request for a full end-to-end check:

```bash
curl -X POST http://localhost:8080/mcp \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"healthcheck","version":"1.0.0"}}}'
```

### Resource Requirements

The server is lightweight and stateless:

- **Memory**: 256 MB limit recommended (typical usage ~30-50 MB)
- **CPU**: 1 vCPU sufficient
- **Disk**: None (read-only filesystem compatible)
- **Network**: Outbound HTTPS to `li.quest` (LI.FI API)

### Container Security

- Runs as non-root user (UID 65532)
- No shell or package manager in runtime image
- No secrets baked into the image
- Compatible with `--read-only` filesystem flag
- Stripped binary (`-s -w -trimpath`)

## License

MIT
