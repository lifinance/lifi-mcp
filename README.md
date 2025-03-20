# LI.FI MCP Server

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

This MCP server integrates with the [LI.FI API](https://li.quest) to provide cross-chain swap functionality across multiple liquidity pools and bridges.

## Components

### Tools

#### Token Information
- **GetTokens**
  - Fetch all tokens known to the LI.FI services
  - Parameters: `chains`, `chainTypes`, `minPriceUSD`
  
- **GetToken**
  - Get detailed information about a specific token
  - Parameters: `chain` (required), `token` (required)

#### Chain Information
- **GetChains**
  - Get information about all supported chains
  - Parameters: `chainTypes`
  
- **GetChainById**
  - Find a chain by its numeric ID
  - Parameters: `id` (required)
  
- **GetChainByName**
  - Find a chain by name, key, or ID (case insensitive)
  - Parameters: `name` (required)

#### Cross-Chain Operations
- **GetQuote**
  - Get a quote for a token transfer (cross-chain or same-chain)
  - Parameters: `fromChain`, `toChain`, `fromToken`, `toToken`, `fromAddress`, `fromAmount`, etc.
  
- **GetStatus**
  - Check the status of a cross-chain transfer
  - Parameters: `txHash` (required), `bridge`, `fromChain`, `toChain`
  
- **GetConnections**
  - Returns all possible connections between chains
  - Parameters: `fromChain`, `toChain`, `fromToken`, `toToken`, `chainTypes`
  
- **GetTools**
  - Get available bridges and exchanges
  - Parameters: `chains`

#### Wallet Operations
- **GetWalletAddress**
  - Get the Ethereum address for the loaded private key
  
- **GetNativeTokenBalance**
  - Get the native token balance of a wallet
  - Parameters: `rpcUrl` (required), `address` (required)
  
- **GetTokenBalance**
  - Get the balance of a specific ERC20 token for a wallet
  - Parameters: `rpcUrl`, `tokenAddress`, `walletAddress`
  
- **GetAllowance**
  - Check the allowance of an ERC20 token for a specific spender
  - Parameters: `rpcUrl`, `tokenAddress`, `ownerAddress`, `spenderAddress`

#### Transaction Operations
- **ExecuteQuote**
  - Execute a quote transaction using the stored private key
  - Parameters: `rpcUrl`, `transactionRequest`
  
- **ApproveToken**
  - Approve a specific amount of ERC20 tokens to be spent by another address
  - Parameters: `rpcUrl`, `tokenAddress`, `spenderAddress`, `amount`
  
- **TransferToken**
  - Transfer ERC20 tokens to another address
  - Parameters: `rpcUrl`, `tokenAddress`, `to`, `amount`
  
- **TransferNative**
  - Transfer native cryptocurrency to another address
  - Parameters: `rpcUrl`, `to`, `amount`

## Getting Started

### Installation

#### Using the Install Script

You can install the LI.FI MCP server using the following command:

```bash
curl https://raw.githubusercontent.com/lifinance/lifi-mcp/refs/heads/main/install.sh | bash
```

#### Using Go Install

Alternatively, you can install using Go:

```bash
go install github.com/lifinance/lifi-mcp@latest
```


### Wallet Management

#### Using an Existing Keystore

Run the server with an Ethereum keystore file:

```bash
./lifi-mcp server --keystore <keystore-name> --password <keystore-password>
```

The server will search for a file containing this name in the standard Ethereum keystore directory:
- Linux: ~/.ethereum/keystore
- macOS: ~/Library/Ethereum/keystore
- Windows: %APPDATA%\Ethereum\keystore

#### Creating a New Wallet

Create a new wallet keystore:

```bash
./lifi-mcp new-wallet --name <wallet-name> --password <wallet-password>
```

This generates a new Ethereum wallet, saves it to the standard keystore location, and displays the wallet address.

### Usage with Desktop App

To integrate this server with the desktop app, add the following to your app's server configuration:

```json
{
  "mcpServers": {
    "lifi": {
      "command": "./lifi-mcp",
      "args": ["server"]
    }
  }
}
```

For wallet functionality, include keystore parameters:

```json
{
  "mcpServers": {
    "lifi": {
      "command": "./lifi-mcp",
      "args": ["server", "--keystore", "your-keystore", "--password", "your-password"]
    }
  }
}
```


## License

MIT
