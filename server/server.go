package server

import (
	"crypto/ecdsa"

	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

const (
	BaseURL = "https://li.quest"
)

// Server represents the LiFi MCP server
type Server struct {
	mcpServer  *mcpserver.MCPServer
	privateKey *ecdsa.PrivateKey
	version    string
}

// NewServer creates a new LiFi MCP server instance
func NewServer(version string) *Server {
	s := &Server{
		version: version,
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

// ServeStdio starts the server in stdio mode
func (s *Server) ServeStdio() error {
	return mcpserver.ServeStdio(s.mcpServer)
}

// LoadKeystore loads a keystore file for transaction signing
func (s *Server) LoadKeystore(keystoreName, password string) error {
	privateKey, err := loadKeystore(keystoreName, password)
	if err != nil {
		return err
	}
	s.privateKey = privateKey
	return nil
}

// registerTools registers all available tools with the MCP server
func (s *Server) registerTools() {
	// LiFi API tools
	s.mcpServer.AddTool(mcp.NewTool("get-tokens",
		mcp.WithDescription("Get all known tokens from LiFi API"),
		mcp.WithString("chains", mcp.Description("Comma-separated list of chain IDs to filter tokens")),
		mcp.WithString("chainTypes", mcp.Description("Comma-separated list of chain types to filter tokens")),
		mcp.WithString("minPriceUSD", mcp.Description("Minimum price in USD to filter tokens")),
	), s.getTokensHandler)

	s.mcpServer.AddTool(mcp.NewTool("get-token",
		mcp.WithDescription("Get information about a specific token"),
		mcp.WithString("chain", mcp.Description("Chain ID or name"), mcp.Required()),
		mcp.WithString("token", mcp.Description("Token address or symbol"), mcp.Required()),
	), s.getTokenHandler)

	s.mcpServer.AddTool(mcp.NewTool("get-quote",
		mcp.WithDescription("Get a quote for token transfers/swaps"),
		mcp.WithString("fromChain", mcp.Description("Source chain ID"), mcp.Required()),
		mcp.WithString("toChain", mcp.Description("Destination chain ID"), mcp.Required()),
		mcp.WithString("fromToken", mcp.Description("Source token address"), mcp.Required()),
		mcp.WithString("toToken", mcp.Description("Destination token address"), mcp.Required()),
		mcp.WithString("fromAddress", mcp.Description("Source wallet address"), mcp.Required()),
		mcp.WithString("fromAmount", mcp.Description("Amount to transfer (in token units)"), mcp.Required()),
		mcp.WithString("toAddress", mcp.Description("Destination wallet address")),
		mcp.WithString("slippage", mcp.Description("Slippage tolerance (e.g., '0.03' for 3%)")),
		mcp.WithString("integrator", mcp.Description("Integrator identifier")),
		mcp.WithString("order", mcp.Description("Order preference (RECOMMENDED, FASTEST, CHEAPEST, SAFEST)")),
		mcp.WithArray("allowBridges", mcp.Description("Array of allowed bridge names")),
		mcp.WithArray("allowExchanges", mcp.Description("Array of allowed exchange names")),
	), s.getQuoteHandler)

	s.mcpServer.AddTool(mcp.NewTool("get-status",
		mcp.WithDescription("Check the status of a cross-chain transfer"),
		mcp.WithString("txHash", mcp.Description("Transaction hash to check"), mcp.Required()),
		mcp.WithString("bridge", mcp.Description("Bridge name used for the transfer")),
		mcp.WithString("fromChain", mcp.Description("Source chain ID")),
		mcp.WithString("toChain", mcp.Description("Destination chain ID")),
	), s.getStatusHandler)

	s.mcpServer.AddTool(mcp.NewTool("get-chains",
		mcp.WithDescription("Get information about supported chains"),
		mcp.WithString("chainTypes", mcp.Description("Comma-separated list of chain types to filter")),
	), s.getChainsHandler)

	s.mcpServer.AddTool(mcp.NewTool("get-connections",
		mcp.WithDescription("Get information about possible connections between chains"),
		mcp.WithString("fromChain", mcp.Description("Source chain ID")),
		mcp.WithString("toChain", mcp.Description("Destination chain ID")),
		mcp.WithString("fromToken", mcp.Description("Source token address")),
		mcp.WithString("toToken", mcp.Description("Destination token address")),
		mcp.WithString("chainTypes", mcp.Description("Comma-separated list of chain types")),
		mcp.WithArray("allowBridges", mcp.Description("Array of allowed bridge names")),
	), s.getConnectionsHandler)

	s.mcpServer.AddTool(mcp.NewTool("get-tools",
		mcp.WithDescription("Get available bridges and exchanges"),
		mcp.WithArray("chains", mcp.Description("Array of chain IDs to filter tools")),
	), s.getToolsHandler)

	s.mcpServer.AddTool(mcp.NewTool("get-chain-by-id",
		mcp.WithDescription("Get chain information by ID"),
		mcp.WithString("id", mcp.Description("Chain ID"), mcp.Required()),
	), s.getChainByIdHandler)

	s.mcpServer.AddTool(mcp.NewTool("get-chain-by-name",
		mcp.WithDescription("Get chain information by name"),
		mcp.WithString("name", mcp.Description("Chain name or key"), mcp.Required()),
	), s.getChainByNameHandler)

	// Blockchain interaction tools
	s.mcpServer.AddTool(mcp.NewTool("get-native-token-balance",
		mcp.WithDescription("Get native token balance for an address"),
		mcp.WithString("rpcUrl", mcp.Description("RPC URL for the blockchain"), mcp.Required()),
		mcp.WithString("address", mcp.Description("Wallet address to check"), mcp.Required()),
	), s.getNativeTokenBalanceHandler)

	s.mcpServer.AddTool(mcp.NewTool("get-token-balance",
		mcp.WithDescription("Get ERC20 token balance for an address"),
		mcp.WithString("rpcUrl", mcp.Description("RPC URL for the blockchain"), mcp.Required()),
		mcp.WithString("tokenAddress", mcp.Description("Token contract address"), mcp.Required()),
		mcp.WithString("walletAddress", mcp.Description("Wallet address to check"), mcp.Required()),
	), s.getTokenBalanceHandler)

	s.mcpServer.AddTool(mcp.NewTool("get-allowance",
		mcp.WithDescription("Check ERC20 token allowance for a spender"),
		mcp.WithString("rpcUrl", mcp.Description("RPC URL for the blockchain"), mcp.Required()),
		mcp.WithString("tokenAddress", mcp.Description("Token contract address"), mcp.Required()),
		mcp.WithString("ownerAddress", mcp.Description("Token owner address"), mcp.Required()),
		mcp.WithString("spenderAddress", mcp.Description("Spender address to check allowance for"), mcp.Required()),
	), s.getAllowanceHandler)

	// Wallet tools (require keystore)
	s.mcpServer.AddTool(mcp.NewTool("get-wallet-address",
		mcp.WithDescription("Get the wallet address from loaded keystore"),
	), s.getWalletAddressHandler)

	s.mcpServer.AddTool(mcp.NewTool("execute-quote",
		mcp.WithDescription("Execute a quote transaction using loaded keystore"),
		mcp.WithString("rpcUrl", mcp.Description("RPC URL for the blockchain"), mcp.Required()),
		mcp.WithObject("transactionRequest", mcp.Description("Transaction request object from get-quote response"), mcp.Required()),
	), s.executeQuoteHandler)

	s.mcpServer.AddTool(mcp.NewTool("approve-token",
		mcp.WithDescription("Approve ERC20 token spending"),
		mcp.WithString("rpcUrl", mcp.Description("RPC URL for the blockchain"), mcp.Required()),
		mcp.WithString("tokenAddress", mcp.Description("Token contract address"), mcp.Required()),
		mcp.WithString("spenderAddress", mcp.Description("Address to approve for spending"), mcp.Required()),
		mcp.WithString("amount", mcp.Description("Amount to approve (in token units)"), mcp.Required()),
	), s.approveTokenHandler)

	s.mcpServer.AddTool(mcp.NewTool("transfer-token",
		mcp.WithDescription("Transfer ERC20 tokens"),
		mcp.WithString("rpcUrl", mcp.Description("RPC URL for the blockchain"), mcp.Required()),
		mcp.WithString("tokenAddress", mcp.Description("Token contract address"), mcp.Required()),
		mcp.WithString("to", mcp.Description("Recipient address"), mcp.Required()),
		mcp.WithString("amount", mcp.Description("Amount to transfer (in token units)"), mcp.Required()),
	), s.transferTokenHandler)

	s.mcpServer.AddTool(mcp.NewTool("transfer-native",
		mcp.WithDescription("Transfer native cryptocurrency"),
		mcp.WithString("rpcUrl", mcp.Description("RPC URL for the blockchain"), mcp.Required()),
		mcp.WithString("to", mcp.Description("Recipient address"), mcp.Required()),
		mcp.WithString("amount", mcp.Description("Amount to transfer (in wei)"), mcp.Required()),
	), s.transferNativeHandler)
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

// Cache for chain data
var chainsCache ChainData
var chainsCacheInitialized bool = false

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