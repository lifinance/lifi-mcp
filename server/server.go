package server

import (
	"context"
	"fmt"
	"log/slog"
	"runtime/debug"
	"sync"

	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

const (
	BaseURL = "https://li.quest"
)

// Server represents the LiFi MCP server (multi-tenant, stateless)
type Server struct {
	mcpServer  *mcpserver.MCPServer
	httpClient *HTTPClient
	version    string
	logger     *slog.Logger
}

// NewServer creates a new LiFi MCP server instance
func NewServer(version string, logger *slog.Logger) *Server {
	if logger == nil {
		logger = slog.Default()
	}

	s := &Server{
		version:    version,
		httpClient: NewHTTPClient(logger),
		logger:     logger,
	}

	// Create the MCP server
	s.mcpServer = mcpserver.NewMCPServer(
		"lifi-mcp",
		version,
	)

	// Register tools
	s.registerTools()

	return s
}

// GetMCPServer returns the underlying MCP server for in-process transport
func (s *Server) GetMCPServer() *mcpserver.MCPServer {
	return s.mcpServer
}

// withPanicRecovery wraps a handler with panic recovery to prevent server crashes
func (s *Server) withPanicRecovery(handler mcpserver.ToolHandlerFunc) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (result *mcp.CallToolResult, err error) {
		defer func() {
			if r := recover(); r != nil {
				stack := debug.Stack()
				s.logger.Error("Handler panic recovered",
					"panic", r,
					"stack", string(stack),
				)
				result = mcp.NewToolResultError(fmt.Sprintf("internal error: handler panic: %v", r))
				err = nil // Don't return error, return a tool result instead
			}
		}()
		return handler(ctx, request)
	}
}

// registerTools registers all available tools with the MCP server
func (s *Server) registerTools() {
	// Health check tool for orchestration
	s.mcpServer.AddTool(mcp.NewTool("health-check",
		mcp.WithDescription("Check the health status of the LiFi MCP server. Returns server version and API connectivity status. Use this for health monitoring and orchestration."),
	), s.withPanicRecovery(s.healthCheckHandler))

	// LiFi API tools - Token Information
	s.mcpServer.AddTool(mcp.NewTool("get-tokens",
		mcp.WithDescription("Retrieve a list of all tokens supported by LI.FI across multiple chains. Use this to discover available tokens before executing swaps. Returns token addresses, symbols, decimals, and price information. Can filter by chain or minimum price to reduce response size."),
		mcp.WithString("chains", mcp.Description("Comma-separated chain IDs to filter tokens (e.g., '1,137,42161' for Ethereum, Polygon, Arbitrum). Omit for all chains.")),
		mcp.WithString("chainTypes", mcp.Description("Filter by chain type: 'EVM' for Ethereum-compatible chains, 'SVM' for Solana. Comma-separated for multiple (e.g., 'EVM,SVM').")),
		mcp.WithString("minPriceUSD", mcp.Description("Minimum token price in USD to filter out low-value tokens (e.g., '0.01' for tokens worth at least 1 cent).")),
	), s.withPanicRecovery(s.getTokensHandler))

	s.mcpServer.AddTool(mcp.NewTool("get-token",
		mcp.WithDescription("Get detailed information about a specific token including its address, symbol, decimals, and current price. Use this to verify token details before a swap or to look up a token by its symbol."),
		mcp.WithString("chain", mcp.Description("Chain identifier - either numeric ID (e.g., '1' for Ethereum, '137' for Polygon) or name (e.g., 'ethereum', 'polygon')."), mcp.Required()),
		mcp.WithString("token", mcp.Description("Token identifier - either contract address (e.g., '0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48' for USDC) or symbol (e.g., 'USDC'). Use '0x0000000000000000000000000000000000000000' for native tokens."), mcp.Required()),
	), s.withPanicRecovery(s.getTokenHandler))

	// LiFi API tools - Quote & Swap (Primary workflow tools)
	s.mcpServer.AddTool(mcp.NewTool("get-quote",
		mcp.WithDescription("Get a quote for swapping or bridging tokens. This is the PRIMARY tool for initiating any token swap. Returns the best route including expected output amount, fees, estimated time, and a transactionRequest object. For ERC20 tokens, you may need to approve tokens first if allowance is insufficient."),
		mcp.WithString("fromChain", mcp.Description("Source chain ID (e.g., '1' for Ethereum, '137' for Polygon, '42161' for Arbitrum, '10' for Optimism)."), mcp.Required()),
		mcp.WithString("toChain", mcp.Description("Destination chain ID. Use same as fromChain for same-chain swaps, different for cross-chain bridges."), mcp.Required()),
		mcp.WithString("fromToken", mcp.Description("Source token address. Use '0x0000000000000000000000000000000000000000' for native tokens (ETH, MATIC, etc.) or the ERC20 contract address."), mcp.Required()),
		mcp.WithString("toToken", mcp.Description("Destination token address. Use '0x0000000000000000000000000000000000000000' for native tokens or the ERC20 contract address on the destination chain."), mcp.Required()),
		mcp.WithString("fromAddress", mcp.Description("Sender's wallet address (0x...). This address must have sufficient balance and approve ERC20 tokens if needed."), mcp.Required()),
		mcp.WithString("fromAmount", mcp.Description("Amount to swap in the token's smallest unit (wei). For example, to swap 1 USDC (6 decimals), use '1000000'. For 1 ETH (18 decimals), use '1000000000000000000'."), mcp.Required()),
		mcp.WithString("toAddress", mcp.Description("Recipient's wallet address. Defaults to fromAddress if not specified. Use for sending tokens to a different address.")),
		mcp.WithString("slippage", mcp.Description("Maximum acceptable slippage as a decimal (e.g., '0.03' for 3%, '0.005' for 0.5%). Higher values increase success rate but may result in worse rates.")),
		mcp.WithString("integrator", mcp.Description("Your integrator identifier for tracking and fee sharing. Contact LI.FI for an integrator ID.")),
		mcp.WithString("order", mcp.Description("Route optimization preference: 'RECOMMENDED' (balanced), 'FASTEST' (minimize time), 'CHEAPEST' (minimize fees), 'SAFEST' (most reliable bridges).")),
		mcp.WithArray("allowBridges", mcp.Description("Whitelist specific bridges (e.g., ['stargate', 'hop', 'across']). Use get-tools to see available bridges.")),
		mcp.WithArray("allowExchanges", mcp.Description("Whitelist specific DEXes (e.g., ['uniswap', 'sushiswap', '1inch']). Use get-tools to see available exchanges.")),
	), s.withPanicRecovery(s.getQuoteHandler))

	s.mcpServer.AddTool(mcp.NewTool("get-status",
		mcp.WithDescription("Check the status of an in-progress or completed cross-chain transfer. Use this to track bridge transactions which can take minutes to hours. Returns status (PENDING, DONE, FAILED), source/destination transaction hashes, and any error messages."),
		mcp.WithString("txHash", mcp.Description("The transaction hash from the source chain."), mcp.Required()),
		mcp.WithString("bridge", mcp.Description("Bridge name used for the transfer (e.g., 'stargate', 'hop'). Speeds up status lookup if known.")),
		mcp.WithString("fromChain", mcp.Description("Source chain ID. Helps identify the correct transaction if txHash exists on multiple chains.")),
		mcp.WithString("toChain", mcp.Description("Destination chain ID. Required for some bridges to track the receiving transaction.")),
	), s.withPanicRecovery(s.getStatusHandler))

	// LiFi API tools - Chain Information
	s.mcpServer.AddTool(mcp.NewTool("get-chains",
		mcp.WithDescription("Get a list of all blockchain networks supported by LI.FI. Returns chain IDs, names, native tokens, RPC URLs, and block explorer URLs. Use this to discover available chains or get RPC URLs for blockchain interactions."),
		mcp.WithString("chainTypes", mcp.Description("Filter by chain type: 'EVM' for Ethereum-compatible chains (Ethereum, Polygon, Arbitrum, etc.), 'SVM' for Solana. Comma-separated for multiple.")),
	), s.withPanicRecovery(s.getChainsHandler))

	s.mcpServer.AddTool(mcp.NewTool("get-connections",
		mcp.WithDescription("Discover which token pairs can be swapped between chains. Use this to check if a specific swap route exists before calling get-quote. Returns available bridges and their supported tokens for the specified route."),
		mcp.WithString("fromChain", mcp.Description("Source chain ID (e.g., '1' for Ethereum). Omit to see connections from all chains.")),
		mcp.WithString("toChain", mcp.Description("Destination chain ID. Omit to see connections to all chains.")),
		mcp.WithString("fromToken", mcp.Description("Source token address to filter connections for a specific token.")),
		mcp.WithString("toToken", mcp.Description("Destination token address to filter for specific token pairs.")),
		mcp.WithString("chainTypes", mcp.Description("Filter by chain type: 'EVM', 'SVM', or comma-separated combination.")),
		mcp.WithArray("allowBridges", mcp.Description("Filter to show only specific bridges (e.g., ['stargate', 'hop']).")),
	), s.withPanicRecovery(s.getConnectionsHandler))

	s.mcpServer.AddTool(mcp.NewTool("get-tools",
		mcp.WithDescription("List all available bridges and DEX aggregators that LI.FI can route through. Use this to discover which protocols are available or to get bridge/exchange names for filtering in get-quote. Returns key (identifier to use in API calls) and name (human-readable)."),
		mcp.WithArray("chains", mcp.Description("Filter to show only tools available on specific chains (e.g., ['1', '137'] for Ethereum and Polygon).")),
	), s.withPanicRecovery(s.getToolsHandler))

	// LiFi API tools - Advanced Routing
	s.mcpServer.AddTool(mcp.NewTool("get-routes",
		mcp.WithDescription("Get multiple route options for a swap to compare alternatives. Unlike get-quote which returns the single best route, this returns several options ranked by the specified order preference. Useful when you want to show users multiple choices or when the best route fails. Use get-step-transaction to get executable transaction data for a chosen route."),
		mcp.WithString("fromChainId", mcp.Description("Source chain ID (e.g., '1' for Ethereum, '137' for Polygon)."), mcp.Required()),
		mcp.WithString("toChainId", mcp.Description("Destination chain ID for cross-chain swaps, or same as fromChainId for same-chain."), mcp.Required()),
		mcp.WithString("fromTokenAddress", mcp.Description("Source token contract address. Use '0x0000000000000000000000000000000000000000' for native tokens."), mcp.Required()),
		mcp.WithString("toTokenAddress", mcp.Description("Destination token contract address on the target chain."), mcp.Required()),
		mcp.WithString("fromAddress", mcp.Description("Sender's wallet address (0x...)."), mcp.Required()),
		mcp.WithString("fromAmount", mcp.Description("Amount in the token's smallest unit (wei). E.g., '1000000' for 1 USDC."), mcp.Required()),
		mcp.WithString("toAddress", mcp.Description("Recipient wallet address. Defaults to fromAddress.")),
		mcp.WithString("slippage", mcp.Description("Maximum slippage as decimal (e.g., '0.03' for 3%).")),
		mcp.WithString("order", mcp.Description("How to rank routes: 'RECOMMENDED', 'FASTEST', 'CHEAPEST', or 'SAFEST'.")),
	), s.withPanicRecovery(s.getRoutesHandler))

	s.mcpServer.AddTool(mcp.NewTool("get-quote-with-calls",
		mcp.WithDescription("Get a quote that includes custom smart contract calls on the destination chain (also known as 'Zaps'). This enables complex DeFi operations in a single transaction: bridge tokens AND deposit into a vault, stake in a protocol, or interact with any contract. The contract calls execute atomically after the bridge completes."),
		mcp.WithString("fromChain", mcp.Description("Source chain ID (e.g., '1' for Ethereum)."), mcp.Required()),
		mcp.WithString("toChain", mcp.Description("Destination chain ID where contract calls will execute."), mcp.Required()),
		mcp.WithString("fromToken", mcp.Description("Source token address to bridge."), mcp.Required()),
		mcp.WithString("toToken", mcp.Description("Intermediate token on destination chain (before contract calls)."), mcp.Required()),
		mcp.WithString("fromAddress", mcp.Description("Sender's wallet address."), mcp.Required()),
		mcp.WithString("fromAmount", mcp.Description("Amount in smallest unit (wei)."), mcp.Required()),
		mcp.WithArray("contractCalls", mcp.Description("Array of contract calls to execute on destination. Each object needs: 'toContractAddress' (target contract), 'toContractCallData' (encoded function call), 'toContractGasLimit' (gas for this call)."), mcp.Required()),
		mcp.WithString("slippage", mcp.Description("Maximum slippage as decimal (e.g., '0.03' for 3%).")),
	), s.withPanicRecovery(s.getQuoteWithCallsHandler))

	s.mcpServer.AddTool(mcp.NewTool("get-step-transaction",
		mcp.WithDescription("Convert a route step from get-routes into executable transaction data. Use this when you've chosen a specific route from get-routes and need the transaction to sign and send. Returns the same transactionRequest format as get-quote."),
		mcp.WithObject("step", mcp.Description("A step object from the get-routes response. Pass the entire step object including its 'action', 'estimate', and other properties."), mcp.Required()),
	), s.withPanicRecovery(s.getStepTransactionHandler))

	// LiFi API tools - Gas Information
	s.mcpServer.AddTool(mcp.NewTool("get-gas-prices",
		mcp.WithDescription("Get current gas prices for all supported EVM chains. Returns fast/standard/slow gas prices in gwei. Useful for estimating transaction costs before executing swaps or for monitoring network congestion."),
	), s.withPanicRecovery(s.getGasPricesHandler))

	s.mcpServer.AddTool(mcp.NewTool("get-gas-suggestion",
		mcp.WithDescription("Get recommended gas parameters for a specific chain including base fee, priority fee, and estimated costs. More detailed than get-gas-prices for a single chain. Useful when constructing transactions manually."),
		mcp.WithString("chainId", mcp.Description("Chain ID to get gas suggestions for (e.g., '1' for Ethereum, '137' for Polygon)."), mcp.Required()),
	), s.withPanicRecovery(s.getGasSuggestionHandler))

	// LiFi API tools - API Key Testing
	s.mcpServer.AddTool(mcp.NewTool("test-api-key",
		mcp.WithDescription("Test if the LI.FI API key provided in the request header is valid. Returns key status and rate limit information. Requires API key to be passed via Authorization header (Bearer token) or X-LiFi-Api-Key header."),
	), s.withPanicRecovery(s.testApiKeyHandler))

	// LiFi API tools - Chain Lookup
	s.mcpServer.AddTool(mcp.NewTool("get-chain-by-id",
		mcp.WithDescription("Look up chain details by numeric chain ID. Returns chain name, native token info, RPC URLs, and block explorer. Use this to convert a chain ID to human-readable information or to get RPC URLs."),
		mcp.WithString("id", mcp.Description("Numeric chain ID (e.g., '1' for Ethereum, '137' for Polygon, '42161' for Arbitrum, '10' for Optimism, '56' for BSC)."), mcp.Required()),
	), s.withPanicRecovery(s.getChainByIdHandler))

	s.mcpServer.AddTool(mcp.NewTool("get-chain-by-name",
		mcp.WithDescription("Look up chain details by name or key. Performs case-insensitive matching against chain name, key, or ID. Use this when you know the chain name but need its ID or RPC URL."),
		mcp.WithString("name", mcp.Description("Chain name (e.g., 'Ethereum', 'Polygon'), key (e.g., 'eth', 'pol'), or ID as string (e.g., '1')."), mcp.Required()),
	), s.withPanicRecovery(s.getChainByNameHandler))

	// Blockchain interaction tools - Balance & Allowance Queries (read-only, no signing required)
	s.mcpServer.AddTool(mcp.NewTool("get-native-token-balance",
		mcp.WithDescription("Check the native token balance (ETH, MATIC, etc.) of any wallet address. Returns the balance in wei (smallest unit) along with the token symbol and decimals."),
		mcp.WithString("chain", mcp.Description("Chain identifier - either numeric ID (e.g., '1' for Ethereum, '137' for Polygon) or name (e.g., 'ethereum', 'polygon'). The RPC URL is looked up automatically."), mcp.Required()),
		mcp.WithString("rpcUrl", mcp.Description("Optional: Custom RPC endpoint URL. If provided, overrides the default RPC for the chain. Use this if you have your own RPC endpoint (e.g., Alchemy, Infura).")),
		mcp.WithString("address", mcp.Description("Wallet address to check balance for (0x... format, 42 characters)."), mcp.Required()),
	), s.withPanicRecovery(s.getNativeTokenBalanceHandler))

	s.mcpServer.AddTool(mcp.NewTool("get-token-balance",
		mcp.WithDescription("Check the ERC20 token balance of any wallet address. Returns the balance in the token's smallest unit along with symbol and decimals. Use this before swaps to verify sufficient balance."),
		mcp.WithString("chain", mcp.Description("Chain identifier - either numeric ID (e.g., '1' for Ethereum) or name (e.g., 'ethereum'). The RPC URL is looked up automatically."), mcp.Required()),
		mcp.WithString("rpcUrl", mcp.Description("Optional: Custom RPC endpoint URL. Overrides the default RPC for the chain.")),
		mcp.WithString("tokenAddress", mcp.Description("ERC20 token contract address (0x... format). Get from get-token or get-tokens."), mcp.Required()),
		mcp.WithString("walletAddress", mcp.Description("Wallet address to check balance for (0x... format)."), mcp.Required()),
	), s.withPanicRecovery(s.getTokenBalanceHandler))

	s.mcpServer.AddTool(mcp.NewTool("get-allowance",
		mcp.WithDescription("Check how many ERC20 tokens a spender is approved to use on behalf of an owner. IMPORTANT: Before executing a swap with ERC20 tokens, verify the allowance is >= the swap amount. If insufficient, the user must approve tokens first. The spender address for LI.FI swaps is returned in the get-quote response."),
		mcp.WithString("chain", mcp.Description("Chain identifier - either numeric ID (e.g., '1' for Ethereum) or name (e.g., 'ethereum'). The RPC URL is looked up automatically."), mcp.Required()),
		mcp.WithString("rpcUrl", mcp.Description("Optional: Custom RPC endpoint URL. Overrides the default RPC for the chain.")),
		mcp.WithString("tokenAddress", mcp.Description("ERC20 token contract address to check allowance for."), mcp.Required()),
		mcp.WithString("ownerAddress", mcp.Description("Wallet address that owns the tokens (the 'fromAddress' in swaps)."), mcp.Required()),
		mcp.WithString("spenderAddress", mcp.Description("Contract address that would spend the tokens. For LI.FI swaps, use the address from transactionRequest.to in the get-quote response."), mcp.Required()),
	), s.withPanicRecovery(s.getAllowanceHandler))
}

// Chain data structures
type ChainData struct {
	Chains []Chain `json:"chains"`
}

type Chain struct {
	ID             int          `json:"id"`
	Key            string       `json:"key"`
	Name           string       `json:"name"`
	NativeToken    Token        `json:"nativeToken"`
	NativeCurrency Token        `json:"nativeCurrency"`
	Metamask       MetamaskInfo `json:"metamask"`
}

type MetamaskInfo struct {
	ChainId           string   `json:"chainId"`
	BlockExplorerUrls []string `json:"blockExplorerUrls"`
	ChainName         string   `json:"chainName"`
	RpcUrls           []string `json:"rpcUrls"`
}

type Token struct {
	Address  string `json:"address"`
	Symbol   string `json:"symbol"`
	Decimals int    `json:"decimals"`
	Name     string `json:"name"`
}

// Cache for chain data with mutex protection
var (
	chainsCache            ChainData
	chainsCacheInitialized bool
	chainsCacheMu          sync.RWMutex
)

// ERC20 ABI for token interactions
const ERC20ABI = `[
	{
		"constant": true,
		"inputs": [{"name": "_owner", "type": "address"}],
		"name": "balanceOf",
		"outputs": [{"name": "balance", "type": "uint256"}],
		"type": "function"
	},
	{
		"constant": false,
		"inputs": [
			{"name": "_to", "type": "address"},
			{"name": "_value", "type": "uint256"}
		],
		"name": "transfer",
		"outputs": [{"name": "", "type": "bool"}],
		"type": "function"
	},
	{
		"constant": false,
		"inputs": [
			{"name": "_spender", "type": "address"},
			{"name": "_value", "type": "uint256"}
		],
		"name": "approve",
		"outputs": [{"name": "", "type": "bool"}],
		"type": "function"
	},
	{
		"constant": true,
		"inputs": [
			{"name": "_owner", "type": "address"},
			{"name": "_spender", "type": "address"}
		],
		"name": "allowance",
		"outputs": [{"name": "", "type": "uint256"}],
		"type": "function"
	},
	{
		"constant": true,
		"inputs": [],
		"name": "symbol",
		"outputs": [{"name": "", "type": "string"}],
		"type": "function"
	},
	{
		"constant": true,
		"inputs": [],
		"name": "decimals",
		"outputs": [{"name": "", "type": "uint8"}],
		"type": "function"
	}
]`
