package main

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const (
	BaseURL = "https://li.quest"
)

// Global variable to store the private key
var privateKey *ecdsa.PrivateKey

// GetWalletAddress returns the Ethereum address corresponding to the loaded private key
func GetWalletAddress() (string, error) {
	if privateKey == nil {
		return "", errors.New("no private key loaded")
	}
	
	publicKey := crypto.PubkeyToAddress(privateKey.PublicKey)
	return publicKey.Hex(), nil
}

// getKeystoreDir returns the default keystore directory based on the operating system
func getKeystoreDir() (string, error) {
	var keystorePath string
	
	// Get home directory
	usr, err := user.Current()
	if err != nil {
		return "", fmt.Errorf("failed to get current user: %v", err)
	}
	
	// Determine OS-specific keystore path
	switch runtime.GOOS {
	case "linux":
		keystorePath = filepath.Join(usr.HomeDir, ".ethereum", "keystore")
	case "darwin": // macOS
		keystorePath = filepath.Join(usr.HomeDir, "Library", "Ethereum", "keystore")
	case "windows":
		keystorePath = filepath.Join(usr.HomeDir, "AppData", "Roaming", "Ethereum", "keystore")
	default:
		return "", fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}
	
	return keystorePath, nil
}

// loadKeystore loads a keystore file by name from the standard Ethereum keystore location
func loadKeystore(keystoreName, password string) (*ecdsa.PrivateKey, error) {
	// Get the keystore directory
	keystoreDir, err := getKeystoreDir()
	if err != nil {
		return nil, err
	}
	
	// List files in the keystore directory to find the matching keystore
	files, err := os.ReadDir(keystoreDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read keystore directory: %v", err)
	}
	
	// Look for a keystore file matching the provided name
	var keystorePath string
	for _, file := range files {
		if strings.Contains(file.Name(), keystoreName) {
			keystorePath = filepath.Join(keystoreDir, file.Name())
			break
		}
	}
	
	if keystorePath == "" {
		return nil, fmt.Errorf("keystore not found with name: %s", keystoreName)
	}
	
	// Read the keystore file
	keystoreJSON, err := os.ReadFile(keystorePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read keystore file: %v", err)
	}
	
	// Decrypt the key with the provided password
	key, err := keystore.DecryptKey(keystoreJSON, password)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt keystore: %v", err)
	}
	
	return key.PrivateKey, nil
}

// GetTokensHandler handles requests to get all known tokens
func GetTokensHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	chains, _ := request.Params.Arguments["chains"].(string)
	chainTypes, _ := request.Params.Arguments["chainTypes"].(string)
	minPriceUSD, _ := request.Params.Arguments["minPriceUSD"].(string)

	// Build the query parameters
	params := url.Values{}
	if chains != "" {
		params.Add("chains", chains)
	}
	if chainTypes != "" {
		params.Add("chainTypes", chainTypes)
	}
	if minPriceUSD != "" {
		params.Add("minPriceUSD", minPriceUSD)
	}

	// Build the request URL
	requestURL := fmt.Sprintf("%s/v1/tokens", BaseURL)
	if len(params) > 0 {
		requestURL += "?" + params.Encode()
	}

	// Make the request
	resp, err := http.Get(requestURL)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Error making request: %v", err))
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Error reading response: %v", err))
	}

	return mcp.NewToolResultText(string(body)), nil
}

// GetTokenHandler handles requests to get info about a specific token
func GetTokenHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	chain, _ := request.Params.Arguments["chain"].(string)
	token, _ := request.Params.Arguments["token"].(string)

	if chain == "" || token == "" {
		return nil, errors.New("Both chain and token parameters are required")
	}

	// Build the query parameters
	params := url.Values{}
	params.Add("chain", chain)
	params.Add("token", token)

	// Build the request URL
	requestURL := fmt.Sprintf("%s/v1/token?%s", BaseURL, params.Encode())

	// Make the request
	resp, err := http.Get(requestURL)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Error making request: %v", err))
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Error reading response: %v", err))
	}

	return mcp.NewToolResultText(string(body)), nil
}

// GetQuoteHandler handles requests to get a quote for token transfers
func GetQuoteHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Get all required parameters
	fromChain, _ := request.Params.Arguments["fromChain"].(string)
	toChain, _ := request.Params.Arguments["toChain"].(string)
	fromToken, _ := request.Params.Arguments["fromToken"].(string)
	toToken, _ := request.Params.Arguments["toToken"].(string)
	fromAddress, _ := request.Params.Arguments["fromAddress"].(string)
	fromAmount, _ := request.Params.Arguments["fromAmount"].(string)

	// Required parameters check
	if fromChain == "" || toChain == "" || fromToken == "" || toToken == "" || fromAddress == "" || fromAmount == "" {
		return nil, errors.New("Required parameters: fromChain, toChain, fromToken, toToken, fromAddress, fromAmount")
	}

	// Get optional parameters
	toAddress, _ := request.Params.Arguments["toAddress"].(string)
	slippage, _ := request.Params.Arguments["slippage"].(string)
	integrator, _ := request.Params.Arguments["integrator"].(string)
	order, _ := request.Params.Arguments["order"].(string)

	// Build the query parameters
	params := url.Values{}
	params.Add("fromChain", fromChain)
	params.Add("toChain", toChain)
	params.Add("fromToken", fromToken)
	params.Add("toToken", toToken)
	params.Add("fromAddress", fromAddress)
	params.Add("fromAmount", fromAmount)

	if toAddress != "" {
		params.Add("toAddress", toAddress)
	}
	if slippage != "" {
		params.Add("slippage", slippage)
	}
	if integrator != "" {
		params.Add("integrator", integrator)
	}
	if order != "" {
		params.Add("order", order)
	}

	// Add any array parameters
	if allowBridges, ok := request.Params.Arguments["allowBridges"]; ok {
		if bridgesSlice, isArray := allowBridges.([]interface{}); isArray {
			bridgesList := make([]string, len(bridgesSlice))
			for i, bridge := range bridgesSlice {
				bridgesList[i] = fmt.Sprintf("%v", bridge)
			}
			params.Add("allowBridges", strings.Join(bridgesList, ","))
		}
	}

	if allowExchanges, ok := request.Params.Arguments["allowExchanges"]; ok {
		if exchangesSlice, isArray := allowExchanges.([]interface{}); isArray {
			exchangesList := make([]string, len(exchangesSlice))
			for i, exchange := range exchangesSlice {
				exchangesList[i] = fmt.Sprintf("%v", exchange)
			}
			params.Add("allowExchanges", strings.Join(exchangesList, ","))
		}
	}

	// Build the request URL
	requestURL := fmt.Sprintf("%s/v1/quote?%s", BaseURL, params.Encode())

	// Make the request
	resp, err := http.Get(requestURL)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Error making request: %v", err))
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Error reading response: %v", err))
	}

	return mcp.NewToolResultText(string(body)), nil
}

// GetStatusHandler handles requests to check the status of a cross-chain transfer
func GetStatusHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	txHash, _ := request.Params.Arguments["txHash"].(string)

	if txHash == "" {
		return nil, errors.New("txHash parameter is required")
	}

	// Get optional parameters
	bridge, _ := request.Params.Arguments["bridge"].(string)
	fromChain, _ := request.Params.Arguments["fromChain"].(string)
	toChain, _ := request.Params.Arguments["toChain"].(string)

	// Build the query parameters
	params := url.Values{}
	params.Add("txHash", txHash)
	
	if bridge != "" {
		params.Add("bridge", bridge)
	}
	if fromChain != "" {
		params.Add("fromChain", fromChain)
	}
	if toChain != "" {
		params.Add("toChain", toChain)
	}

	// Build the request URL
	requestURL := fmt.Sprintf("%s/v1/status?%s", BaseURL, params.Encode())

	// Make the request
	resp, err := http.Get(requestURL)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Error making request: %v", err))
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Error reading response: %v", err))
	}

	return mcp.NewToolResultText(string(body)), nil
}

// GetChainsHandler handles requests to get information about supported chains
func GetChainsHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	chainTypes, _ := request.Params.Arguments["chainTypes"].(string)

	// Ensure the chains are loaded
	if !chainsCacheInitialized {
		err := refreshChainsCache()
		if err != nil {
			return nil, errors.New(fmt.Sprintf("Failed to fetch chain data: %v", err))
		}
	}

	// If no chain types filter is specified, return all chains
	if chainTypes == "" {
		jsonData, err := json.Marshal(chainsCache)
		if err != nil {
			return nil, errors.New(fmt.Sprintf("Error serializing chain data: %v", err))
		}
		return mcp.NewToolResultText(string(jsonData)), nil
	}

	// Filter chains by chainTypes
	chainTypesSlice := strings.Split(chainTypes, ",")
	filteredChains := ChainData{
		Chains: []Chain{},
	}

	for _, chain := range chainsCache.Chains {
		// Check if the chain matches any of the requested chain types
		// Note: The actual implementation of this filter would depend on how
		// chain types are represented in the Chain struct. This is a placeholder.
		// If chain types are not directly in the Chain struct, we might need to make
		// an API call specifically for filtering.
		for _, ct := range chainTypesSlice {
			// This is a simplified check - adjust based on actual data structure
			if strings.Contains(strings.ToLower(chain.Key), strings.ToLower(strings.TrimSpace(ct))) {
				filteredChains.Chains = append(filteredChains.Chains, chain)
				break
			}
		}
	}

	// If no chains matched the filter, make a direct API call to ensure accurate results
	if len(filteredChains.Chains) == 0 {
		// Build the query parameters
		params := url.Values{}
		params.Add("chainTypes", chainTypes)

		// Build the request URL
		requestURL := fmt.Sprintf("%s/v1/chains?%s", BaseURL, params.Encode())

		// Make the request
		resp, err := http.Get(requestURL)
		if err != nil {
			return nil, errors.New(fmt.Sprintf("Error making request: %v", err))
		}
		defer resp.Body.Close()

		// Read the response body
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, errors.New(fmt.Sprintf("Error reading response: %v", err))
		}

		return mcp.NewToolResultText(string(body)), nil
	}

	// Return the filtered chains
	jsonData, err := json.Marshal(filteredChains)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Error serializing filtered chain data: %v", err))
	}
	return mcp.NewToolResultText(string(jsonData)), nil
}

// GetConnectionsHandler handles requests to get info about possible connections between chains
func GetConnectionsHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Get parameters
	fromChain, _ := request.Params.Arguments["fromChain"].(string)
	toChain, _ := request.Params.Arguments["toChain"].(string)
	fromToken, _ := request.Params.Arguments["fromToken"].(string)
	toToken, _ := request.Params.Arguments["toToken"].(string)
	chainTypes, _ := request.Params.Arguments["chainTypes"].(string)

	// Build the query parameters
	params := url.Values{}
	if fromChain != "" {
		params.Add("fromChain", fromChain)
	}
	if toChain != "" {
		params.Add("toChain", toChain)
	}
	if fromToken != "" {
		params.Add("fromToken", fromToken)
	}
	if toToken != "" {
		params.Add("toToken", toToken)
	}
	if chainTypes != "" {
		params.Add("chainTypes", chainTypes)
	}

	// Add array parameters if present
	if allowBridges, ok := request.Params.Arguments["allowBridges"]; ok {
		if bridgesSlice, isArray := allowBridges.([]interface{}); isArray {
			bridgesList := make([]string, len(bridgesSlice))
			for i, bridge := range bridgesSlice {
				bridgesList[i] = fmt.Sprintf("%v", bridge)
			}
			encodedBridges, _ := json.Marshal(bridgesList)
			params.Add("allowBridges", string(encodedBridges))
		}
	}

	// Build the request URL
	requestURL := fmt.Sprintf("%s/v1/connections?%s", BaseURL, params.Encode())

	// Make the request
	resp, err := http.Get(requestURL)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Error making request: %v", err))
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Error reading response: %v", err))
	}

	return mcp.NewToolResultText(string(body)), nil
}

// GetToolsHandler handles requests to get available bridges and exchanges
func GetToolsHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Get parameters (chains is an array)
	var chains []string
	
	if chainsParam, ok := request.Params.Arguments["chains"]; ok {
		if chainsSlice, isArray := chainsParam.([]interface{}); isArray {
			chains = make([]string, len(chainsSlice))
			for i, chain := range chainsSlice {
				chains[i] = fmt.Sprintf("%v", chain)
			}
		}
	}

	// Build the query parameters
	params := url.Values{}
	if len(chains) > 0 {
		encodedChains, _ := json.Marshal(chains)
		params.Add("chains", string(encodedChains))
	}

	// Build the request URL
	requestURL := fmt.Sprintf("%s/v1/tools?%s", BaseURL, params.Encode())

	// Make the request
	resp, err := http.Get(requestURL)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Error making request: %v", err))
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Error reading response: %v", err))
	}

	return mcp.NewToolResultText(string(body)), nil
}

// GetWalletAddressHandler returns the Ethereum address for the loaded private key
func GetWalletAddressHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	address, err := GetWalletAddress()
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Error getting wallet address: %v", err))
	}
	
	result := map[string]string{
		"address": address,
	}
	
	jsonResult, err := json.Marshal(result)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Error serializing result: %v", err))
	}
	
	return mcp.NewToolResultText(string(jsonResult)), nil
}

// Chain data structure to parse Li.Fi API responses
type ChainData struct {
	Chains []Chain `json:"chains"`
}

type Chain struct {
	ID               int           `json:"id"`
	Key              string        `json:"key"`
	Name             string        `json:"name"`
	NativeToken      Token         `json:"nativeToken"`
	NativeCurrency   Token         `json:"nativeCurrency"`
	Metamask         MetamaskInfo  `json:"metamask"`
}

type MetamaskInfo struct {
	ChainId            string   `json:"chainId"`
	BlockExplorerUrls  []string `json:"blockExplorerUrls"`
	ChainName          string   `json:"chainName"`
	RpcUrls            []string `json:"rpcUrls"`
}

type Token struct {
	Address  string `json:"address"`
	Symbol   string `json:"symbol"`
	Decimals int    `json:"decimals"`
	Name     string `json:"name"`
}

// Cache for chain data to avoid repeated API calls
var chainsCache ChainData
var chainsCacheInitialized bool = false

// GetChainByIdHandler returns a chain that matches the provided id
func GetChainByIdHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Get required parameter
	idStr, _ := request.Params.Arguments["id"].(string)
	
	if idStr == "" {
		return nil, errors.New("ID parameter is required")
	}
	
	// Attempt to parse the ID as an integer
	var id int
	_, err := fmt.Sscanf(idStr, "%d", &id)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Invalid ID format. Expected integer, got: %s", idStr))
	}
	
	// Ensure the chains are loaded
	if !chainsCacheInitialized {
		err := refreshChainsCache()
		if err != nil {
			return nil, errors.New(fmt.Sprintf("Failed to fetch chain data: %v", err))
		}
	}
	
	// Look for the chain by ID
	for _, chain := range chainsCache.Chains {
		if chain.ID == id {
			// Found a match, return the chain data
			chainData, err := json.Marshal(chain)
			if err != nil {
				return nil, errors.New(fmt.Sprintf("Error serializing chain data: %v", err))
			}
			
			return mcp.NewToolResultText(string(chainData)), nil
		}
	}
	
	// No matching chain found
	return nil, errors.New(fmt.Sprintf("No chain found with ID: %d", id))
}

// GetChainByNameHandler returns a chain that matches the provided name
func GetChainByNameHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Get required parameter
	name, _ := request.Params.Arguments["name"].(string)
	
	if name == "" {
		return nil, errors.New("Name parameter is required")
	}
	
	// Ensure the chains are loaded
	if !chainsCacheInitialized {
		err := refreshChainsCache()
		if err != nil {
			return nil, errors.New(fmt.Sprintf("Failed to fetch chain data: %v", err))
		}
	}
	
	// Convert name to lowercase for case-insensitive matching
	nameLower := strings.ToLower(name)
	
	// Look for the chain by name
	for _, chain := range chainsCache.Chains {
		// Try matching against name, key, or chain ID as string
		if strings.ToLower(chain.Name) == nameLower ||
		   strings.ToLower(chain.Key) == nameLower || 
		   strings.ToLower(fmt.Sprintf("%d", chain.ID)) == nameLower {
			// Found a match, return the chain data
			chainData, err := json.Marshal(chain)
			if err != nil {
				return nil, errors.New(fmt.Sprintf("Error serializing chain data: %v", err))
			}
			
			return mcp.NewToolResultText(string(chainData)), nil
		}
	}
	
	// No matching chain found
	return nil, errors.New(fmt.Sprintf("No chain found matching name: %s", name))
}

// GetNativeTokenBalanceHandler returns the native token balance of a given address on the specified chain
func GetNativeTokenBalanceHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Get required parameters
	rpcUrl, _ := request.Params.Arguments["rpcUrl"].(string)
	address, _ := request.Params.Arguments["address"].(string)
	
	if rpcUrl == "" || address == "" {
		return nil, errors.New("Both rpcUrl and address parameters are required")
	}
	
	// Validate address format
	if !common.IsHexAddress(address) {
		return nil, errors.New(fmt.Sprintf("Invalid Ethereum address format: %s", address))
	}
	
	// Connect to the Ethereum client
	client, err := ethclient.Dial(rpcUrl)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Failed to connect to the Ethereum client: %v", err))
	}
	defer client.Close()
	
	// Convert address string to common.Address
	accountAddress := common.HexToAddress(address)
	
	// Get the balance
	balance, err := client.BalanceAt(ctx, accountAddress, nil) // nil means latest block
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Failed to get balance: %v", err))
	}
	
	// Get chain ID to determine which token symbol to display
	chainID, err := client.ChainID(ctx)
	if err != nil {
		// If we can't get chain ID, we'll just use "Native Token" as the symbol
		chainID = big.NewInt(0)
	}
	
	// Get token symbol from chain data
	symbol, decimals, err := getNativeTokenInfo(chainID)
	if err != nil {
		// Fall back to a generic symbol if we can't get chain data
		symbol = "Native Token"
		decimals = 18 // Most chains use 18 decimals
	}
	
	// Format the result
	result := map[string]interface{}{
		"address":     address,
		"balance":     balance.String(),
		"tokenSymbol": symbol,
		"chainId":     chainID.String(),
		"decimals":    decimals,
	}
	
	jsonResult, err := json.Marshal(result)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Error serializing result: %v", err))
	}
	
	return mcp.NewToolResultText(string(jsonResult)), nil
}

// ExecuteQuoteHandler executes a quote transaction using the stored private key
func ExecuteQuoteHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Check if private key is loaded
	if privateKey == nil {
		return nil, errors.New("No private key loaded. Please start the server with a keystore.")
	}
	
	// Get RPC URL (required)
	rpcUrl, _ := request.Params.Arguments["rpcUrl"].(string)
	if rpcUrl == "" {
		return nil, errors.New("RPC URL is required")
	}
	
	// Get transactionRequest object (required)
	txRequest, ok := request.Params.Arguments["transactionRequest"].(map[string]interface{})
	if !ok || txRequest == nil {
		return nil, errors.New("Transaction request object is required")
	}
	
	// Execute the transaction
	return executeTransactionRequest(ctx, txRequest, rpcUrl)
}

// executeTransactionRequest handles execution of a transaction request object 
// that comes directly from the GetQuote response
func executeTransactionRequest(ctx context.Context, txRequest map[string]interface{}, rpcUrl string) (*mcp.CallToolResult, error) {
	// Validate the RPC URL
	if rpcUrl == "" {
		return nil, errors.New("RPC URL is required")
	}
	
	// Connect to the Ethereum client
	client, err := ethclient.Dial(rpcUrl)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Failed to connect to the Ethereum client: %v", err))
	}
	defer client.Close()
	
	// Get chain ID from the client
	networkChainID, err := client.ChainID(ctx)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Failed to get chain ID: %v", err))
	}
	
	// Get and validate transaction parameters
	valuehex, _ := txRequest["value"].(string)
	tohex, _ := txRequest["to"].(string)
	datahex, _ := txRequest["data"].(string)
	fromhex, _ := txRequest["from"].(string)
	
	// Validate required transaction parameters
	if tohex == "" {
		return nil, errors.New("Transaction 'to' address is required in transactionRequest")
	}
	
	if datahex == "" {
		return nil, errors.New("Transaction 'data' is required in transactionRequest")
	}
	
	// Get the wallet address
	walletAddress := crypto.PubkeyToAddress(privateKey.PublicKey)
	
	// If from address is specified, verify it matches our wallet address
	if fromhex != "" && !strings.EqualFold(fromhex, walletAddress.Hex()) {
		return nil, errors.New(fmt.Sprintf(
			"Transaction 'from' address (%s) doesn't match wallet address (%s)", 
			fromhex, walletAddress.Hex()))
	}
	
	// Convert chain ID from request
	var requestChainID *big.Int
	if chainIDValue, ok := txRequest["chainId"]; ok {
		switch v := chainIDValue.(type) {
		case float64:
			requestChainID = big.NewInt(int64(v))
		case string:
			requestChainID = new(big.Int)
			if strings.HasPrefix(v, "0x") {
				requestChainID.SetString(v[2:], 16)
			} else {
				requestChainID.SetString(v, 10)
			}
		}
		
		// Validate chain ID matches the network
		if requestChainID != nil && requestChainID.Cmp(networkChainID) != 0 {
			return nil, errors.New(fmt.Sprintf(
				"Chain ID in transaction (%s) doesn't match network chain ID (%s)",
				requestChainID.String(), networkChainID.String()))
		}
	}
	
	// If no chain ID was in the request or it was invalid, use the network chain ID
	if requestChainID == nil {
		requestChainID = networkChainID
	}
	
	// Convert hex value to big.Int
	valueInt := new(big.Int)
	if valuehex == "" || valuehex == "0x" || valuehex == "0x0" {
		valueInt.SetInt64(0)
	} else {
		if strings.HasPrefix(valuehex, "0x") {
			valueInt.SetString(valuehex[2:], 16)
		} else {
			valueInt.SetString(valuehex, 10)
		}
	}
	
	// Parse gas price from request or get suggested gas price
	var gasPriceInt *big.Int
	if gasPriceHex, ok := txRequest["gasPrice"].(string); ok && gasPriceHex != "" {
		gasPriceInt = new(big.Int)
		if strings.HasPrefix(gasPriceHex, "0x") {
			gasPriceInt.SetString(gasPriceHex[2:], 16)
		} else {
			gasPriceInt.SetString(gasPriceHex, 10)
		}
	} else {
		gasPriceInt, err = client.SuggestGasPrice(ctx)
		if err != nil {
			return nil, errors.New(fmt.Sprintf("Failed to suggest gas price: %v", err))
		}
	}
	
	// Decode data hex string
	var dataBytes []byte
	if strings.HasPrefix(datahex, "0x") {
		dataBytes, err = hex.DecodeString(datahex[2:])
	} else {
		dataBytes, err = hex.DecodeString(datahex)
	}
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Invalid transaction data: %v", err))
	}
	
	// Parse gas limit or estimate it
	var gasLimitInt uint64
	if gasLimitHex, ok := txRequest["gasLimit"].(string); ok && gasLimitHex != "" {
		if strings.HasPrefix(gasLimitHex, "0x") {
			gasLimitInt64, err := strconv.ParseInt(gasLimitHex[2:], 16, 64)
			if err != nil {
				return nil, errors.New(fmt.Sprintf("Invalid gas limit: %s", gasLimitHex))
			}
			gasLimitInt = uint64(gasLimitInt64)
		} else {
			gasLimitInt64, err := strconv.ParseInt(gasLimitHex, 10, 64)
			if err != nil {
				return nil, errors.New(fmt.Sprintf("Invalid gas limit: %s", gasLimitHex))
			}
			gasLimitInt = uint64(gasLimitInt64)
		}
	} else {
		// Estimate gas using the transaction data
		toAddress := common.HexToAddress(tohex)
		msg := ethereum.CallMsg{
			From:     walletAddress,
			To:       &toAddress,
			Gas:      0,
			GasPrice: gasPriceInt,
			Value:    valueInt,
			Data:     dataBytes,
		}
		
		gasLimitInt, err = client.EstimateGas(ctx, msg)
		if err != nil {
			return nil, errors.New(fmt.Sprintf("Failed to estimate gas: %v", err))
		}
		
		// Add a buffer to the gas limit to avoid out-of-gas errors
		gasLimitInt = uint64(float64(gasLimitInt) * 1.2) // Add 20% buffer
	}
	
	// Get current nonce
	nonceInt, err := client.PendingNonceAt(ctx, walletAddress)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Failed to get nonce: %v", err))
	}
	
	// Create and send the transaction
	tx := types.NewTransaction(
		nonceInt,
		common.HexToAddress(tohex),
		valueInt,
		gasLimitInt,
		gasPriceInt,
		dataBytes,
	)
	
	// Sign the transaction with the private key
	signedTx, err := types.SignTx(tx, types.NewEIP155Signer(requestChainID), privateKey)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Failed to sign transaction: %v", err))
	}
	
	// Try simulating the transaction first to check for reverts
	toAddress := common.HexToAddress(tohex)
	msg := ethereum.CallMsg{
		From:     walletAddress,
		To:       &toAddress,
		Gas:      gasLimitInt,
		GasPrice: gasPriceInt,
		Value:    valueInt,
		Data:     dataBytes,
	}
	
	// Simulate the transaction
	_, err = client.CallContract(ctx, msg, nil)
	if err != nil {
		// Extract detailed revert reason if possible
		revertReason := "Unknown reason"
		errorText := err.Error()
		
		// Try to extract a revert reason from the error message
		if strings.Contains(errorText, "execution reverted") {
			// Extract any reason provided after "execution reverted:"
			if parts := strings.SplitN(errorText, "execution reverted:", 2); len(parts) > 1 {
				revertReason = strings.TrimSpace(parts[1])
			}
		}
		
		return nil, errors.New(fmt.Sprintf("Transaction would fail: %v. Revert reason: %s", err, revertReason))
	}
	
	// Send the transaction
	err = client.SendTransaction(ctx, signedTx)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Failed to send transaction: %v", err))
	}
	
	// Return the transaction hash and other details
	result := map[string]interface{}{
		"transactionHash": signedTx.Hash().Hex(),
		"from":            walletAddress.Hex(),
		"to":              tohex,
		"value":           valueInt.String(),
		"gasLimit":        gasLimitInt,
		"gasPrice":        gasPriceInt.String(),
		"nonce":           nonceInt,
		"chainId":         requestChainID.String(),
	}
	
	jsonResult, err := json.Marshal(result)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Error serializing result: %v", err))
	}
	
	return mcp.NewToolResultText(string(jsonResult)), nil
}

// GetTokenBalanceHandler gets the balance of a specific ERC20 token for a wallet address
func GetTokenBalanceHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Get required parameters
	rpcUrl, _ := request.Params.Arguments["rpcUrl"].(string)
	tokenAddress, _ := request.Params.Arguments["tokenAddress"].(string)
	walletAddress, _ := request.Params.Arguments["walletAddress"].(string)
	
	if rpcUrl == "" || tokenAddress == "" || walletAddress == "" {
		return nil, errors.New("rpcUrl, tokenAddress, and walletAddress parameters are required")
	}
	
	// Validate addresses
	if !common.IsHexAddress(tokenAddress) {
		return nil, errors.New(fmt.Sprintf("Invalid token address format: %s", tokenAddress))
	}
	if !common.IsHexAddress(walletAddress) {
		return nil, errors.New(fmt.Sprintf("Invalid wallet address format: %s", walletAddress))
	}
	
	// Connect to the Ethereum client
	client, err := ethclient.Dial(rpcUrl)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Failed to connect to the Ethereum client: %v", err))
	}
	defer client.Close()
	
	// Parse the ERC20 ABI
	parsedABI, err := abi.JSON(strings.NewReader(ERC20ABI))
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Failed to parse ERC20 ABI: %v", err))
	}
	
	// Create common.Address objects
	tokenAddr := common.HexToAddress(tokenAddress)
	walletAddr := common.HexToAddress(walletAddress)
	
	// Pack the input data for the balanceOf function
	data, err := parsedABI.Pack("balanceOf", walletAddr)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Failed to pack input data: %v", err))
	}
	
	// Create the call message
	msg := ethereum.CallMsg{
		To:   &tokenAddr,
		Data: data,
	}
	
	// Call the contract
	result, err := client.CallContract(ctx, msg, nil) // nil means latest block
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Failed to call contract: %v", err))
	}
	
	// Unpack the result
	var balance *big.Int
	err = parsedABI.UnpackIntoInterface(&balance, "balanceOf", result)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Failed to unpack result: %v", err))
	}
	
	// Get token information
	tokenSymbol, tokenDecimals, err := getTokenInfo(ctx, client, tokenAddress)
	if err != nil {
		// If we can't get token info, just use defaults
		tokenSymbol = "Unknown"
		tokenDecimals = 18
	}
	
	// Get chain ID to include in the response
	chainID, err := client.ChainID(ctx)
	if err != nil {
		chainID = big.NewInt(0)
	}
	
	// Format the result
	responseData := map[string]interface{}{
		"walletAddress": walletAddress,
		"tokenAddress":  tokenAddress,
		"balance":       balance.String(),
		"tokenSymbol":   tokenSymbol,
		"decimals":      tokenDecimals,
		"chainId":       chainID.String(),
	}
	
	jsonResponse, err := json.Marshal(responseData)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Error serializing result: %v", err))
	}
	
	return mcp.NewToolResultText(string(jsonResponse)), nil
}

// ApproveTokenHandler approves a specific amount of ERC20 tokens to be spent by another address
func ApproveTokenHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Check if private key is loaded
	if privateKey == nil {
		return nil, errors.New("No private key loaded. Please start the server with a keystore.")
	}
	
	// Get required parameters
	rpcUrl, _ := request.Params.Arguments["rpcUrl"].(string)
	tokenAddress, _ := request.Params.Arguments["tokenAddress"].(string)
	spenderAddress, _ := request.Params.Arguments["spenderAddress"].(string)
	amountStr, _ := request.Params.Arguments["amount"].(string)
	
	// Validate required parameters individually for better error messages
	if rpcUrl == "" {
		return nil, errors.New("RPC URL is required")
	}
	
	if tokenAddress == "" {
		return nil, errors.New("Token address is required")
	}
	
	if spenderAddress == "" {
		return nil, errors.New("Spender address is required")
	}
	
	if amountStr == "" {
		return nil, errors.New("Amount is required")
	}
	
	// Validate addresses
	if !common.IsHexAddress(tokenAddress) {
		return nil, errors.New(fmt.Sprintf("Invalid token address format: %s", tokenAddress))
	}
	if !common.IsHexAddress(spenderAddress) {
		return nil, errors.New(fmt.Sprintf("Invalid spender address format: %s", spenderAddress))
	}
	
	// Parse amount
	amount := new(big.Int)
	amount, ok := amount.SetString(amountStr, 10)
	if !ok {
		return nil, errors.New(fmt.Sprintf("Invalid amount format: %s", amountStr))
	}
	
	// Connect to the Ethereum client
	client, err := ethclient.Dial(rpcUrl)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Failed to connect to the Ethereum client: %v", err))
	}
	defer client.Close()
	
	// Get chain ID
	chainID, err := client.ChainID(ctx)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Failed to get chain ID: %v", err))
	}
	
	// Parse the ERC20 ABI
	parsedABI, err := abi.JSON(strings.NewReader(ERC20ABI))
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Failed to parse ERC20 ABI: %v", err))
	}
	
	// Get the wallet address from the private key
	walletAddress := crypto.PubkeyToAddress(privateKey.PublicKey)
	
	// Get token information for better UX in response
	tokenSymbol, tokenDecimals, err := getTokenInfo(ctx, client, tokenAddress)
	if err != nil {
		// If we can't get token info, just use defaults
		tokenSymbol = "Unknown"
		tokenDecimals = 18
	}
	
	// Convert token address to common.Address once and reuse
	tokenAddr := common.HexToAddress(tokenAddress)
	
	// Pack the approve function data
	data, err := parsedABI.Pack("approve", common.HexToAddress(spenderAddress), amount)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Failed to pack approve data: %v", err))
	}
	
	// Try simulating the transaction first to check for reverts
	_, err = client.CallContract(ctx, ethereum.CallMsg{
		From: walletAddress,
		To:   &tokenAddr,
		Data: data,
	}, nil)
	if err != nil {
		// Extract detailed revert reason if possible
		revertReason := "Unknown reason"
		errorText := err.Error()
		
		// Try to extract a revert reason from the error message
		if strings.Contains(errorText, "execution reverted") {
			// Extract any reason provided after "execution reverted:"
			if parts := strings.SplitN(errorText, "execution reverted:", 2); len(parts) > 1 {
				revertReason = strings.TrimSpace(parts[1])
			}
		}
		
		return nil, errors.New(fmt.Sprintf("Approval would fail: %v. Revert reason: %s", err, revertReason))
	}
	
	// Estimate gas for the transaction
	gasLimit, err := client.EstimateGas(ctx, ethereum.CallMsg{
		From: walletAddress,
		To:   &tokenAddr,
		Data: data,
	})
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Failed to estimate gas: %v", err))
	}
	
	// Add a buffer to the gas limit for safety
	gasLimit = uint64(float64(gasLimit) * 1.2)
	
	// Get nonce
	nonce, err := client.PendingNonceAt(ctx, walletAddress)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Failed to get nonce: %v", err))
	}
	
	// Get EIP-1559 fee suggestions
	head, err := client.HeaderByNumber(ctx, nil)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Failed to get latest block header: %v", err))
	}
	
	// Check if the network supports EIP-1559
	var tx *types.Transaction
	if head.BaseFee != nil {
		// EIP-1559 transaction
		// Get fee suggestions
		gasTipCap, err := client.SuggestGasTipCap(ctx)
		if err != nil {
			return nil, errors.New(fmt.Sprintf("Failed to suggest gas tip cap: %v", err))
		}
		
		// Calculate max fee per gas (base fee * 2 + tip)
		baseFee := head.BaseFee
		maxFeePerGas := new(big.Int).Add(
			new(big.Int).Mul(baseFee, big.NewInt(2)),
			gasTipCap,
		)
		
		// Create the EIP-1559 transaction
		tx = types.NewTx(&types.DynamicFeeTx{
			ChainID:   chainID,
			Nonce:     nonce,
			GasTipCap: gasTipCap,
			GasFeeCap: maxFeePerGas,
			Gas:       gasLimit,
			To:        &tokenAddr,
			Value:     big.NewInt(0),
			Data:      data,
		})
	} else {
		// Legacy transaction for chains that don't support EIP-1559
		gasPrice, err := client.SuggestGasPrice(ctx)
		if err != nil {
			return nil, errors.New(fmt.Sprintf("Failed to suggest gas price: %v", err))
		}
		
		// Create the legacy transaction
		tx = types.NewTx(&types.LegacyTx{
			Nonce:    nonce,
			GasPrice: gasPrice,
			Gas:      gasLimit,
			To:       &tokenAddr,
			Value:    big.NewInt(0),
			Data:     data,
		})
	}
	
	// Sign the transaction
	signedTx, err := types.SignTx(tx, types.LatestSignerForChainID(chainID), privateKey)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Failed to sign transaction: %v", err))
	}
	
	// Send the transaction
	err = client.SendTransaction(ctx, signedTx)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Failed to send transaction: %v", err))
	}
	
	// Format the response
	responseData := map[string]interface{}{
		"transactionHash": signedTx.Hash().Hex(),
		"from":            walletAddress.Hex(),
		"tokenAddress":    tokenAddress,
		"tokenSymbol":     tokenSymbol,
		"spender":         spenderAddress,
		"amount":          amount.String(),
		"decimals":        tokenDecimals,
		"chainId":         chainID.String(),
		"gasLimit":        gasLimit,
		"nonce":           nonce,
	}
	
	// Add fee information based on transaction type
	if head.BaseFee != nil {
		// For EIP-1559 transactions
		if signedTx.Type() == types.DynamicFeeTxType {
			responseData["maxFeePerGas"] = signedTx.GasFeeCap().String()
			responseData["maxPriorityFeePerGas"] = signedTx.GasTipCap().String()
			responseData["transactionType"] = "EIP-1559"
		}
	} else {
		// For legacy transactions
		responseData["gasPrice"] = signedTx.GasPrice().String()
		responseData["transactionType"] = "Legacy"
	}
	
	jsonResponse, err := json.Marshal(responseData)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Error serializing result: %v", err))
	}
	
	return mcp.NewToolResultText(string(jsonResponse)), nil
}

// TransferTokenHandler transfers ERC20 tokens to another address
func TransferTokenHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Check if private key is loaded
	if privateKey == nil {
		return nil, errors.New("No private key loaded. Please start the server with a keystore.")
	}
	
	// Get required parameters
	rpcUrl, _ := request.Params.Arguments["rpcUrl"].(string)
	tokenAddress, _ := request.Params.Arguments["tokenAddress"].(string)
	recipientAddress, _ := request.Params.Arguments["to"].(string)
	amountStr, _ := request.Params.Arguments["amount"].(string)
	
	// Validate required parameters individually for better error messages
	if rpcUrl == "" {
		return nil, errors.New("RPC URL is required")
	}
	
	if tokenAddress == "" {
		return nil, errors.New("Token address is required")
	}
	
	if recipientAddress == "" {
		return nil, errors.New("Recipient address (to) is required")
	}
	
	if amountStr == "" {
		return nil, errors.New("Amount is required")
	}
	
	// Validate addresses
	if !common.IsHexAddress(tokenAddress) {
		return nil, errors.New(fmt.Sprintf("Invalid token address format: %s", tokenAddress))
	}
	if !common.IsHexAddress(recipientAddress) {
		return nil, errors.New(fmt.Sprintf("Invalid recipient address format: %s", recipientAddress))
	}
	
	// Parse amount
	amount := new(big.Int)
	amount, ok := amount.SetString(amountStr, 10)
	if !ok {
		return nil, errors.New(fmt.Sprintf("Invalid amount format: %s", amountStr))
	}
	
	// Connect to the Ethereum client
	client, err := ethclient.Dial(rpcUrl)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Failed to connect to the Ethereum client: %v", err))
	}
	defer client.Close()
	
	// Get chain ID
	chainID, err := client.ChainID(ctx)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Failed to get chain ID: %v", err))
	}
	
	// Parse the ERC20 ABI
	parsedABI, err := abi.JSON(strings.NewReader(ERC20ABI))
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Failed to parse ERC20 ABI: %v", err))
	}
	
	// Get the wallet address from the private key
	walletAddress := crypto.PubkeyToAddress(privateKey.PublicKey)
	
	// Get token information for better UX in response
	tokenSymbol, tokenDecimals, err := getTokenInfo(ctx, client, tokenAddress)
	if err != nil {
		// If we can't get token info, just use defaults
		tokenSymbol = "Unknown"
		tokenDecimals = 18
	}
	
	// Convert token address to common.Address once and reuse
	tokenAddr := common.HexToAddress(tokenAddress)
	
	// Check token balance before transfer
	balanceData, err := parsedABI.Pack("balanceOf", walletAddress)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Failed to pack balanceOf data: %v", err))
	}
	
	// Call the balanceOf function
	balanceResult, err := client.CallContract(ctx, ethereum.CallMsg{
		To:   &tokenAddr,
		Data: balanceData,
	}, nil)
	if err != nil {
		// Extract detailed revert reason if possible
		revertReason := "Unknown reason"
		errorText := err.Error()
		
		// Try to extract a revert reason from the error message
		if strings.Contains(errorText, "execution reverted") {
			if parts := strings.SplitN(errorText, "execution reverted:", 2); len(parts) > 1 {
				revertReason = strings.TrimSpace(parts[1])
			}
		}
		
		return nil, errors.New(fmt.Sprintf("Failed to call balanceOf: %v. Revert reason: %s", err, revertReason))
	}
	
	// Unpack the balance
	var balance *big.Int
	err = parsedABI.UnpackIntoInterface(&balance, "balanceOf", balanceResult)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Failed to unpack balance: %v", err))
	}
	
	// Check if the balance is sufficient
	if balance.Cmp(amount) < 0 {
		return nil, errors.New(fmt.Sprintf(
			"Insufficient token balance: have %s, need %s", balance.String(), amount.String()))
	}
	
	// Pack the transfer function data
	data, err := parsedABI.Pack("transfer", common.HexToAddress(recipientAddress), amount)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Failed to pack transfer data: %v", err))
	}
	
	// Try simulating the transaction first to check for reverts
	_, err = client.CallContract(ctx, ethereum.CallMsg{
		From: walletAddress,
		To:   &tokenAddr,
		Data: data,
	}, nil)
	if err != nil {
		// Extract detailed revert reason if possible
		revertReason := "Unknown reason"
		errorText := err.Error()
		
		// Try to extract a revert reason from the error message
		if strings.Contains(errorText, "execution reverted") {
			if parts := strings.SplitN(errorText, "execution reverted:", 2); len(parts) > 1 {
				revertReason = strings.TrimSpace(parts[1])
			}
		}
		
		return nil, errors.New(fmt.Sprintf("Transfer would fail: %v. Revert reason: %s", err, revertReason))
	}
	
	// Estimate gas for the transaction
	gasLimit, err := client.EstimateGas(ctx, ethereum.CallMsg{
		From: walletAddress,
		To:   &tokenAddr,
		Data: data,
	})
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Failed to estimate gas: %v", err))
	}
	
	// Add a buffer to the gas limit for safety
	gasLimit = uint64(float64(gasLimit) * 1.2)
	
	// Get nonce
	nonce, err := client.PendingNonceAt(ctx, walletAddress)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Failed to get nonce: %v", err))
	}
	
	// Get latest block header to check for EIP-1559 support
	head, err := client.HeaderByNumber(ctx, nil)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Failed to get latest block header: %v", err))
	}
	
	// Create and sign the transaction based on EIP-1559 support
	var tx *types.Transaction
	if head.BaseFee != nil {
		// EIP-1559 transaction
		// Get fee suggestions
		gasTipCap, err := client.SuggestGasTipCap(ctx)
		if err != nil {
			return nil, errors.New(fmt.Sprintf("Failed to suggest gas tip cap: %v", err))
		}
		
		// Calculate max fee per gas (base fee * 2 + tip)
		baseFee := head.BaseFee
		maxFeePerGas := new(big.Int).Add(
			new(big.Int).Mul(baseFee, big.NewInt(2)),
			gasTipCap,
		)
		
		// Create the EIP-1559 transaction
		tx = types.NewTx(&types.DynamicFeeTx{
			ChainID:   chainID,
			Nonce:     nonce,
			GasTipCap: gasTipCap,
			GasFeeCap: maxFeePerGas,
			Gas:       gasLimit,
			To:        &tokenAddr,
			Value:     big.NewInt(0),
			Data:      data,
		})
	} else {
		// Legacy transaction for chains that don't support EIP-1559
		gasPrice, err := client.SuggestGasPrice(ctx)
		if err != nil {
			return nil, errors.New(fmt.Sprintf("Failed to suggest gas price: %v", err))
		}
		
		// Create the legacy transaction
		tx = types.NewTx(&types.LegacyTx{
			Nonce:    nonce,
			GasPrice: gasPrice,
			Gas:      gasLimit,
			To:       &tokenAddr,
			Value:    big.NewInt(0),
			Data:     data,
		})
	}
	
	// Sign the transaction
	signedTx, err := types.SignTx(tx, types.LatestSignerForChainID(chainID), privateKey)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Failed to sign transaction: %v", err))
	}
	
	// Send the transaction
	err = client.SendTransaction(ctx, signedTx)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Failed to send transaction: %v", err))
	}
	
	// Format the response
	responseData := map[string]interface{}{
		"transactionHash": signedTx.Hash().Hex(),
		"from":            walletAddress.Hex(),
		"to":              recipientAddress,
		"tokenAddress":    tokenAddress,
		"tokenSymbol":     tokenSymbol,
		"amount":          amount.String(),
		"decimals":        tokenDecimals,
		"chainId":         chainID.String(),
		"gasLimit":        gasLimit,
		"nonce":           nonce,
	}
	
	// Add fee information based on transaction type
	if head.BaseFee != nil {
		// For EIP-1559 transactions
		if signedTx.Type() == types.DynamicFeeTxType {
			responseData["maxFeePerGas"] = signedTx.GasFeeCap().String()
			responseData["maxPriorityFeePerGas"] = signedTx.GasTipCap().String()
			responseData["transactionType"] = "EIP-1559"
		}
	} else {
		// For legacy transactions
		responseData["gasPrice"] = signedTx.GasPrice().String()
		responseData["transactionType"] = "Legacy"
	}
	
	jsonResponse, err := json.Marshal(responseData)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Error serializing result: %v", err))
	}
	
	return mcp.NewToolResultText(string(jsonResponse)), nil
}

// TransferNativeHandler transfers native cryptocurrency to another address
func TransferNativeHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Check if private key is loaded
	if privateKey == nil {
		return nil, errors.New("No private key loaded. Please start the server with a keystore.")
	}
	
	// Get required parameters
	rpcUrl, _ := request.Params.Arguments["rpcUrl"].(string)
	recipientAddress, _ := request.Params.Arguments["to"].(string)
	amountStr, _ := request.Params.Arguments["amount"].(string)
	
	// Validate required parameters individually for better error messages
	if rpcUrl == "" {
		return nil, errors.New("RPC URL is required")
	}
	
	if recipientAddress == "" {
		return nil, errors.New("Recipient address (to) is required")
	}
	
	if amountStr == "" {
		return nil, errors.New("Amount is required")
	}
	
	// Validate recipient address
	if !common.IsHexAddress(recipientAddress) {
		return nil, errors.New(fmt.Sprintf("Invalid recipient address format: %s", recipientAddress))
	}
	
	// Parse amount
	amount := new(big.Int)
	amount, ok := amount.SetString(amountStr, 10)
	if !ok {
		return nil, errors.New(fmt.Sprintf("Invalid amount format: %s", amountStr))
	}
	
	// Connect to the Ethereum client
	client, err := ethclient.Dial(rpcUrl)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Failed to connect to the Ethereum client: %v", err))
	}
	defer client.Close()
	
	// Get chain ID
	chainID, err := client.ChainID(ctx)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Failed to get chain ID: %v", err))
	}
	
	// Get the wallet address from the private key
	walletAddress := crypto.PubkeyToAddress(privateKey.PublicKey)
	
	// Get native token info for the response
	tokenSymbol, tokenDecimals, err := getNativeTokenInfo(chainID)
	if err != nil {
		// Default values if we can't get chain info
		tokenSymbol = "Native Token"
		tokenDecimals = 18
	}
	
	// Check balance before transfer
	balance, err := client.BalanceAt(ctx, walletAddress, nil)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Failed to get wallet balance: %v", err))
	}
	
	// Standard gas for ETH transfer is 21000
	gasLimit := uint64(21000)
	
	// Get latest block header to check for EIP-1559 support
	head, err := client.HeaderByNumber(ctx, nil)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Failed to get latest block header: %v", err))
	}
	
	// Calculate gas cost based on network type (EIP-1559 or legacy)
	var gasCost *big.Int
	var tx *types.Transaction
	
	if head.BaseFee != nil {
		// EIP-1559 network
		gasTipCap, err := client.SuggestGasTipCap(ctx)
		if err != nil {
			return nil, errors.New(fmt.Sprintf("Failed to suggest gas tip cap: %v", err))
		}
		
		// Calculate max fee per gas (base fee * 2 + tip)
		baseFee := head.BaseFee
		maxFeePerGas := new(big.Int).Add(
			new(big.Int).Mul(baseFee, big.NewInt(2)),
			gasTipCap,
		)
		
		// Calculate gas cost using max fee
		gasCost = new(big.Int).Mul(maxFeePerGas, big.NewInt(int64(gasLimit)))
		
		// Check if we have enough funds
		totalNeeded := new(big.Int).Add(amount, gasCost)
		if balance.Cmp(totalNeeded) < 0 {
			return nil, errors.New(fmt.Sprintf(
				"Insufficient balance: have %s, need %s (including max gas cost)", 
				balance.String(), totalNeeded.String()))
		}
		
		// Get nonce
		nonce, err := client.PendingNonceAt(ctx, walletAddress)
		if err != nil {
			return nil, errors.New(fmt.Sprintf("Failed to get nonce: %v", err))
		}
		
		// Create the EIP-1559 transaction
		recipientAddr := common.HexToAddress(recipientAddress)
		tx = types.NewTx(&types.DynamicFeeTx{
			ChainID:   chainID,
			Nonce:     nonce,
			GasTipCap: gasTipCap,
			GasFeeCap: maxFeePerGas,
			Gas:       gasLimit,
			To:        &recipientAddr,
			Value:     amount,
			Data:      nil,
		})
	} else {
		// Legacy network
		gasPrice, err := client.SuggestGasPrice(ctx)
		if err != nil {
			return nil, errors.New(fmt.Sprintf("Failed to suggest gas price: %v", err))
		}
		
		// Calculate gas cost
		gasCost = new(big.Int).Mul(gasPrice, big.NewInt(int64(gasLimit)))
		
		// Check if we have enough funds
		totalNeeded := new(big.Int).Add(amount, gasCost)
		if balance.Cmp(totalNeeded) < 0 {
			return nil, errors.New(fmt.Sprintf(
				"Insufficient balance: have %s, need %s (including gas cost)", 
				balance.String(), totalNeeded.String()))
		}
		
		// Get nonce
		nonce, err := client.PendingNonceAt(ctx, walletAddress)
		if err != nil {
			return nil, errors.New(fmt.Sprintf("Failed to get nonce: %v", err))
		}
		
		// Create the legacy transaction
		recipientAddr := common.HexToAddress(recipientAddress)
		tx = types.NewTx(&types.LegacyTx{
			Nonce:    nonce,
			GasPrice: gasPrice,
			Gas:      gasLimit,
			To:       &recipientAddr,
			Value:    amount,
			Data:     nil,
		})
	}
	
	// Sign the transaction
	signedTx, err := types.SignTx(tx, types.LatestSignerForChainID(chainID), privateKey)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Failed to sign transaction: %v", err))
	}
	
	// Try simulating the transaction to check for reverts
	toAddress := common.HexToAddress(recipientAddress)
	msg := ethereum.CallMsg{
		From:  walletAddress,
		To:    &toAddress,
		Value: amount,
		Data:  nil, // No data for native transfers
	}
	
	// Add gas parameters based on network type
	if head.BaseFee != nil {
		// For EIP-1559
		msg.GasFeeCap = signedTx.GasFeeCap()
		msg.GasTipCap = signedTx.GasTipCap()
		msg.Gas = gasLimit
	} else {
		// For legacy
		msg.GasPrice = signedTx.GasPrice()
		msg.Gas = gasLimit
	}
	
	// Simulate the transaction
	_, err = client.CallContract(ctx, msg, nil)
	if err != nil {
		// Extract detailed revert reason if possible
		revertReason := "Unknown reason"
		errorText := err.Error()
		
		// Try to extract a revert reason from the error message
		if strings.Contains(errorText, "execution reverted") {
			if parts := strings.SplitN(errorText, "execution reverted:", 2); len(parts) > 1 {
				revertReason = strings.TrimSpace(parts[1])
			}
		}
		
		return nil, errors.New(fmt.Sprintf("Transfer would fail: %v. Revert reason: %s", err, revertReason))
	}
	
	// Send the transaction
	err = client.SendTransaction(ctx, signedTx)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Failed to send transaction: %v", err))
	}
	
	// Format the response
	responseData := map[string]interface{}{
		"transactionHash": signedTx.Hash().Hex(),
		"from":            walletAddress.Hex(),
		"to":              recipientAddress,
		"amount":          amount.String(),
		"tokenSymbol":     tokenSymbol,
		"decimals":        tokenDecimals,
		"chainId":         chainID.String(),
		"gasLimit":        gasLimit,
	}
	
	// Add fee information based on transaction type
	if head.BaseFee != nil {
		// For EIP-1559 transactions
		if signedTx.Type() == types.DynamicFeeTxType {
			responseData["maxFeePerGas"] = signedTx.GasFeeCap().String()
			responseData["maxPriorityFeePerGas"] = signedTx.GasTipCap().String()
			responseData["transactionType"] = "EIP-1559"
			responseData["nonce"] = signedTx.Nonce()
		}
	} else {
		// For legacy transactions
		responseData["gasPrice"] = signedTx.GasPrice().String()
		responseData["transactionType"] = "Legacy"
		responseData["nonce"] = signedTx.Nonce()
	}
	
	jsonResponse, err := json.Marshal(responseData)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Error serializing result: %v", err))
	}
	
	return mcp.NewToolResultText(string(jsonResponse)), nil
}

// GetAllowanceHandler checks the allowance of an ERC20 token for a specific spender
func GetAllowanceHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Get required parameters
	rpcUrl, _ := request.Params.Arguments["rpcUrl"].(string)
	tokenAddress, _ := request.Params.Arguments["tokenAddress"].(string)
	ownerAddress, _ := request.Params.Arguments["ownerAddress"].(string)
	spenderAddress, _ := request.Params.Arguments["spenderAddress"].(string)
	
	// Validate required parameters individually for better error messages
	if rpcUrl == "" {
		return nil, errors.New("RPC URL is required")
	}
	
	if tokenAddress == "" {
		return nil, errors.New("Token address is required")
	}
	
	if ownerAddress == "" {
		return nil, errors.New("Owner address is required")
	}
	
	if spenderAddress == "" {
		return nil, errors.New("Spender address is required")
	}
	
	// Validate addresses
	if !common.IsHexAddress(tokenAddress) {
		return nil, errors.New(fmt.Sprintf("Invalid token address format: %s", tokenAddress))
	}
	if !common.IsHexAddress(ownerAddress) {
		return nil, errors.New(fmt.Sprintf("Invalid owner address format: %s", ownerAddress))
	}
	if !common.IsHexAddress(spenderAddress) {
		return nil, errors.New(fmt.Sprintf("Invalid spender address format: %s", spenderAddress))
	}
	
	// Connect to the Ethereum client
	client, err := ethclient.Dial(rpcUrl)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Failed to connect to the Ethereum client: %v", err))
	}
	defer client.Close()
	
	// Parse the ERC20 ABI
	parsedABI, err := abi.JSON(strings.NewReader(ERC20ABI))
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Failed to parse ERC20 ABI: %v", err))
	}
	
	// Convert addresses to common.Address
	tokenAddr := common.HexToAddress(tokenAddress)
	ownerAddr := common.HexToAddress(ownerAddress)
	spenderAddr := common.HexToAddress(spenderAddress)
	
	// Pack the allowance function data
	data, err := parsedABI.Pack("allowance", ownerAddr, spenderAddr)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Failed to pack allowance data: %v", err))
	}
	
	// Call the allowance function
	result, err := client.CallContract(ctx, ethereum.CallMsg{
		To:   &tokenAddr,
		Data: data,
	}, nil) // nil means latest block
	if err != nil {
		// Extract detailed revert reason if possible
		revertReason := "Unknown reason"
		errorText := err.Error()
		
		// Try to extract a revert reason from the error message
		if strings.Contains(errorText, "execution reverted") {
			if parts := strings.SplitN(errorText, "execution reverted:", 2); len(parts) > 1 {
				revertReason = strings.TrimSpace(parts[1])
			}
		}
		
		return nil, errors.New(fmt.Sprintf("Failed to call allowance: %v. Revert reason: %s", err, revertReason))
	}
	
	// Unpack the allowance
	var allowance *big.Int
	err = parsedABI.UnpackIntoInterface(&allowance, "allowance", result)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Failed to unpack allowance: %v", err))
	}
	
	// Get token information for better UX in response
	tokenSymbol, tokenDecimals, err := getTokenInfo(ctx, client, tokenAddress)
	if err != nil {
		// If we can't get token info, just use defaults
		tokenSymbol = "Unknown"
		tokenDecimals = 18
	}
	
	// Get chain ID to include in the response
	chainID, err := client.ChainID(ctx)
	if err != nil {
		chainID = big.NewInt(0)
	}
	
	// Format the response
	responseData := map[string]interface{}{
		"tokenAddress":    tokenAddress,
		"tokenSymbol":     tokenSymbol,
		"ownerAddress":    ownerAddress,
		"spenderAddress":  spenderAddress,
		"allowance":       allowance.String(),
		"decimals":        tokenDecimals,
		"chainId":         chainID.String(),
	}
	
	jsonResponse, err := json.Marshal(responseData)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Error serializing result: %v", err))
	}
	
	return mcp.NewToolResultText(string(jsonResponse)), nil
}

// getTokenInfo retrieves token symbol and decimals for a given token contract
func getTokenInfo(ctx context.Context, client *ethclient.Client, tokenAddress string) (string, int, error) {
	tokenContract := common.HexToAddress(tokenAddress)
	
	// Parse the ERC20 ABI
	parsedABI, err := abi.JSON(strings.NewReader(ERC20ABI))
	if err != nil {
		return "", 0, fmt.Errorf("failed to parse ERC20 ABI: %v", err)
	}
	
	// Get symbol
	symbolData, err := parsedABI.Pack("symbol")
	if err != nil {
		return "", 0, fmt.Errorf("failed to pack symbol data: %v", err)
	}
	
	symbolResult, err := client.CallContract(ctx, ethereum.CallMsg{
		To:   &tokenContract,
		Data: symbolData,
	}, nil)
	if err != nil {
		return "", 0, fmt.Errorf("failed to call symbol: %v", err)
	}
	
	var symbol string
	err = parsedABI.UnpackIntoInterface(&symbol, "symbol", symbolResult)
	if err != nil {
		return "", 0, fmt.Errorf("failed to unpack symbol: %v", err)
	}
	
	// Get decimals
	decimalsData, err := parsedABI.Pack("decimals")
	if err != nil {
		return symbol, 18, fmt.Errorf("failed to pack decimals data: %v", err)
	}
	
	decimalsResult, err := client.CallContract(ctx, ethereum.CallMsg{
		To:   &tokenContract,
		Data: decimalsData,
	}, nil)
	if err != nil {
		return symbol, 18, fmt.Errorf("failed to call decimals: %v", err)
	}
	
	var decimals uint8
	err = parsedABI.UnpackIntoInterface(&decimals, "decimals", decimalsResult)
	if err != nil {
		return symbol, 18, fmt.Errorf("failed to unpack decimals: %v", err)
	}
	
	return symbol, int(decimals), nil
}

// getNativeTokenInfo returns the native token symbol and decimals for a given chain ID
func getNativeTokenInfo(chainID *big.Int) (string, int, error) {
	// Initialize chains cache if not already done
	if !chainsCacheInitialized {
		err := refreshChainsCache()
		if err != nil {
			return "", 18, err
		}
	}
	
	// Look for the chain in the cache
	chainIDInt := int(chainID.Int64())
	for _, chain := range chainsCache.Chains {
		if chain.ID == chainIDInt {
			// Some chains use nativeToken, others use nativeCurrency
			if chain.NativeToken.Symbol != "" {
				return chain.NativeToken.Symbol, chain.NativeToken.Decimals, nil
			}
			if chain.NativeCurrency.Symbol != "" {
				return chain.NativeCurrency.Symbol, chain.NativeCurrency.Decimals, nil
			}
			// If neither is available, try getting from metamask
			if chain.Metamask.ChainName != "" {
				symbolParts := strings.Split(chain.Metamask.ChainName, " ")
				if len(symbolParts) > 0 {
					return symbolParts[0], 18, nil
				}
			}
		}
	}
	
	// If chain not found in cache, try refreshing the cache once
	err := refreshChainsCache()
	if err != nil {
		return "", 18, err
	}
	
	// Look again after refreshing
	for _, chain := range chainsCache.Chains {
		if chain.ID == chainIDInt {
			if chain.NativeToken.Symbol != "" {
				return chain.NativeToken.Symbol, chain.NativeToken.Decimals, nil
			}
			if chain.NativeCurrency.Symbol != "" {
				return chain.NativeCurrency.Symbol, chain.NativeCurrency.Decimals, nil
			}
			// If neither is available, try getting from metamask
			if chain.Metamask.ChainName != "" {
				symbolParts := strings.Split(chain.Metamask.ChainName, " ")
				if len(symbolParts) > 0 {
					return symbolParts[0], 18, nil
				}
			}
		}
	}
	
	return "", 18, fmt.Errorf("chain ID %s not found in Li.Fi API", chainID.String())
}


// refreshChainsCache fetches the latest chain data from Li.Fi API
func refreshChainsCache() error {
	// Build the request URL
	requestURL := fmt.Sprintf("%s/v1/chains", BaseURL)
	
	// Make the request
	resp, err := http.Get(requestURL)
	if err != nil {
		return fmt.Errorf("error making request to Li.Fi API: %v", err)
	}
	defer resp.Body.Close()
	
	// Check for non-200 response
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Li.Fi API returned non-OK status: %s", resp.Status)
	}
	
	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("error reading response: %v", err)
	}
	
	// Log response for debugging (only in development)
	// fmt.Printf("Raw API response: %s\n", string(body))
	
	// Parse the JSON response
	err = json.Unmarshal(body, &chainsCache)
	if err != nil {
		return fmt.Errorf("error parsing chain data: %v (response: %.100s...)", err, string(body))
	}
	
	// Validate the response contains some chains
	if len(chainsCache.Chains) == 0 {
		return fmt.Errorf("Li.Fi API returned empty chains array")
	}
	
	chainsCacheInitialized = true
	return nil
}

// createNewWallet generates a new random Ethereum wallet and saves it to the keystore
func createNewWallet(name, password string) error {
	// Create a new random private key
	key, err := crypto.GenerateKey()
	if err != nil {
		return fmt.Errorf("failed to generate private key: %v", err)
	}
	
	// Get keystore directory
	keystoreDir, err := getKeystoreDir()
	if err != nil {
		return err
	}
	
	// Create directory if it doesn't exist
	if _, err := os.Stat(keystoreDir); os.IsNotExist(err) {
		err = os.MkdirAll(keystoreDir, 0700)
		if err != nil {
			return fmt.Errorf("failed to create keystore directory: %v", err)
		}
	}
	
	// Create the keystore object
	ks := keystore.NewKeyStore(keystoreDir, keystore.StandardScryptN, keystore.StandardScryptP)
	
	// Get the address from the private key
	account, err := ks.ImportECDSA(key, password)
	if err != nil {
		return fmt.Errorf("failed to import key to keystore: %v", err)
	}
	
	// Get the filename of the created keystore
	var keystorePath string
	files, err := os.ReadDir(keystoreDir)
	if err != nil {
		return fmt.Errorf("failed to read keystore directory: %v", err)
	}
	
	// Find the latest file (should be the one just created)
	var latestTime time.Time
	for _, file := range files {
		fileInfo, err := os.Stat(filepath.Join(keystoreDir, file.Name()))
		if err != nil {
			continue
		}
		
		if fileInfo.ModTime().After(latestTime) {
			latestTime = fileInfo.ModTime()
			keystorePath = filepath.Join(keystoreDir, file.Name())
		}
	}
	
	// Rename the keystore file to include the user's name
	if keystorePath != "" {
		// Get the directory and original filename
		dir := filepath.Dir(keystorePath)
		origFilename := filepath.Base(keystorePath)
		
		// Create new filename with the provided name
		// Format: UTC--<timestamp>-<name>-<address>.json
		newFilename := fmt.Sprintf("%s-%s%s", 
			strings.TrimSuffix(origFilename, filepath.Ext(origFilename)), 
			name, 
			filepath.Ext(origFilename))
		
		newPath := filepath.Join(dir, newFilename)
		
		// Rename the file
		err = os.Rename(keystorePath, newPath)
		if err != nil {
			return fmt.Errorf("failed to rename keystore file: %v", err)
		}
		
		keystorePath = newPath
	}
	
	// Print the wallet address
	fmt.Printf("New wallet created successfully!\n")
	fmt.Printf("Address: %s\n", account.Address.Hex())
	fmt.Printf("Keystore location: %s\n", keystorePath)
	
	return nil
}

func main() {
	// Create a command for generating a new wallet
	newWalletCmd := flag.NewFlagSet("new-wallet", flag.ExitOnError)
	newWalletName := newWalletCmd.String("name", "wallet", "A name for the new wallet keystore")
	newWalletPassword := newWalletCmd.String("password", "", "Password to encrypt the new keystore")
	
	// MCP server flags
	serverCmd := flag.NewFlagSet("server", flag.ExitOnError)
	keystoreName := serverCmd.String("keystore", "", "Name of the keystore file to load")
	password := serverCmd.String("password", "", "Password to decrypt the keystore")
	
	// Check which command is being run
	if len(os.Args) > 1 && os.Args[1] == "new-wallet" {
		newWalletCmd.Parse(os.Args[2:])
		
		if *newWalletPassword == "" {
			log.Fatalf("Password is required for creating a new wallet")
		}
		
		err := createNewWallet(*newWalletName, *newWalletPassword)
		if err != nil {
			log.Fatalf("Failed to create new wallet: %v", err)
		}
		
		return
	}
	
	// Default to server mode
	// If no command is specified or if "server" is specified
	serverCmd.Parse(os.Args[1:])
	
	// Load keystore if provided
	if *keystoreName != "" && *password != "" {
		var err error
		privateKey, err = loadKeystore(*keystoreName, *password)
		if err != nil {
			log.Fatalf("Failed to load keystore: %v", err)
		}
		log.Printf("Successfully loaded keystore for: %s", *keystoreName)
	}

	// Create a new MCP server
	mcpServer := server.NewMCPServer(
		"lifi-mcp", // Server name
		"0.1.0",    // Server version
		server.WithToolCapabilities(true),
		server.WithLogging(),
	)

	// LI.FI API tools
	
	// 1. GetTokens - Fetch all known tokens
	getTokensTool := mcp.NewTool(
		"GetTokens",
		mcp.WithDescription("Fetch all tokens known to the LI.FI services"),
		mcp.WithString("chains", 
			mcp.Description("Restrict the resulting tokens to the given chains (comma-separated)")),
		mcp.WithString("chainTypes", 
			mcp.Description("Restrict the resulting tokens to the given chainTypes (comma-separated)")),
		mcp.WithString("minPriceUSD", 
			mcp.Description("Filters results by minimum token price in USD")),
	)
	mcpServer.AddTool(getTokensTool, GetTokensHandler)

	// 2. GetToken - Fetch information about a specific token
	getTokenTool := mcp.NewTool(
		"GetToken",
		mcp.WithDescription("Get more information about a token by its address or symbol and its chain"),
		mcp.WithString("chain", 
			mcp.Description("ID or key of the chain that contains the token"),
			mcp.Required()),
		mcp.WithString("token", 
			mcp.Description("Address or symbol of the token on the requested chain"),
			mcp.Required()),
	)
	mcpServer.AddTool(getTokenTool, GetTokenHandler)

	// 3. GetQuote - Get a quote for a token transfer
	getQuoteTool := mcp.NewTool(
		"GetQuote",
		mcp.WithDescription("Get a quote for a token transfer cross-chain or not"),
		mcp.WithString("fromChain", 
			mcp.Description("The sending chain. Can be the chain id or chain key"),
			mcp.Required()),
		mcp.WithString("toChain", 
			mcp.Description("The receiving chain. Can be the chain id or chain key"),
			mcp.Required()),
		mcp.WithString("fromToken", 
			mcp.Description("The token that should be transferred. Can be the address or the symbol"),
			mcp.Required()),
		mcp.WithString("toToken", 
			mcp.Description("The token that should be transferred to. Can be the address or the symbol"),
			mcp.Required()),
		mcp.WithString("fromAddress", 
			mcp.Description("The sending wallet address"),
			mcp.Required()),
		mcp.WithString("toAddress", 
			mcp.Description("The receiving wallet address. If none is provided, the fromAddress will be used")),
		mcp.WithString("fromAmount", 
			mcp.Description("The amount that should be sent including all decimals (e.g. 1000000 for 1 USDC with 6 decimals)"),
			mcp.Required()),
		mcp.WithString("order", 
			mcp.Description("Which kind of route should be preferred (FASTEST or CHEAPEST)")),
		mcp.WithString("slippage", 
			mcp.Description("The maximum allowed slippage for the transaction as a decimal value (e.g. 0.005 for 0.5%)")),
		mcp.WithString("integrator", 
			mcp.Description("A string containing tracking information about the integrator of the API")),
	)
	mcpServer.AddTool(getQuoteTool, GetQuoteHandler)

	// 4. GetStatus - Check the status of a cross-chain transfer
	getStatusTool := mcp.NewTool(
		"GetStatus",
		mcp.WithDescription("Check the status of a cross-chain transfer"),
		mcp.WithString("txHash", 
			mcp.Description("The transaction hash on the sending chain, destination chain or lifi step id"),
			mcp.Required()),
		mcp.WithString("bridge", 
			mcp.Description("The bridging tool used for the transfer")),
		mcp.WithString("fromChain", 
			mcp.Description("The sending chain. Can be the chain id or chain key")),
		mcp.WithString("toChain", 
			mcp.Description("The receiving chain. Can be the chain id or chain key")),
	)
	mcpServer.AddTool(getStatusTool, GetStatusHandler)

	// 5. GetChains - Get information about supported chains
	getChainsTool := mcp.NewTool(
		"GetChains",
		mcp.WithDescription("Get information about all currently supported chains"),
		mcp.WithString("chainTypes", 
			mcp.Description("Restrict the resulting chains to the given chainTypes")),
	)
	mcpServer.AddTool(getChainsTool, GetChainsHandler)

	// 6. GetConnections - Get possible connections between chains
	getConnectionsTool := mcp.NewTool(
		"GetConnections",
		mcp.WithDescription("Returns all possible connections based on from- or toChain"),
		mcp.WithString("fromChain", 
			mcp.Description("The chain that should be the start of the possible connections")),
		mcp.WithString("toChain", 
			mcp.Description("The chain that should be the end of the possible connections")),
		mcp.WithString("fromToken", 
			mcp.Description("Only return connections starting with this token")),
		mcp.WithString("toToken", 
			mcp.Description("Only return connections ending with this token")),
		mcp.WithString("chainTypes", 
			mcp.Description("Restrict the resulting tokens to the given chainTypes")),
	)
	mcpServer.AddTool(getConnectionsTool, GetConnectionsHandler)

	// 7. GetTools - Get available bridges and exchanges
	getToolsTool := mcp.NewTool(
		"GetTools",
		mcp.WithDescription("Get available bridges and exchanges"),
	)
	mcpServer.AddTool(getToolsTool, GetToolsHandler)
	
	// 8. GetWalletAddress - Get the Ethereum address for the loaded private key
	getWalletAddressTool := mcp.NewTool(
		"GetWalletAddress",
		mcp.WithDescription("Get the Ethereum address for the loaded private key. Use this tool whenever a user refers to their wallet or needs their wallet address"),
	)
	mcpServer.AddTool(getWalletAddressTool, GetWalletAddressHandler)
	
	// 9. GetNativeTokenBalance - Get the native token balance of a wallet
	getNativeTokenBalanceTool := mcp.NewTool(
		"GetNativeTokenBalance",
		mcp.WithDescription("Get the native token (e.g ETH) balance of a wallet address"),
		mcp.WithString("rpcUrl", 
			mcp.Description("The RPC URL of the blockchain node"),
			mcp.Required()),
		mcp.WithString("address", 
			mcp.Description("The wallet address to check the balance of"),
			mcp.Required()),
	)
	mcpServer.AddTool(getNativeTokenBalanceTool, GetNativeTokenBalanceHandler)
	
	// Register the GetChainByName tool
	getChainByNameTool := mcp.NewTool(
		"GetChainByName",
		mcp.WithDescription("Find a chain by name, key, or ID (case insensitive)"),
		mcp.WithString("name", 
			mcp.Description("The name, key, or ID of the chain to find"),
			mcp.Required()),
	)
	mcpServer.AddTool(getChainByNameTool, GetChainByNameHandler)
	
	// Register the GetChainById tool
	getChainByIdTool := mcp.NewTool(
		"GetChainById",
		mcp.WithDescription("Find a chain by its numeric ID"),
		mcp.WithString("id", 
			mcp.Description("The numeric ID of the chain to find"),
			mcp.Required()),
	)
	mcpServer.AddTool(getChainByIdTool, GetChainByIdHandler)
	
	// Register the ExecuteQuote tool
	executeQuoteTool := mcp.NewTool(
		"ExecuteQuote",
		mcp.WithDescription("Execute a quote transaction using the stored private key"),
		mcp.WithString("rpcUrl", 
			mcp.Description("The RPC URL of the blockchain node to connect to"),
			mcp.Required()),
		mcp.WithObject("transactionRequest",
			mcp.Description("Transaction request object from GetQuote response"),
			mcp.Required(),
			mcp.Properties(map[string]interface{}{
				"value": map[string]interface{}{
					"type": "string",
					"description": "Amount of native token to send with the transaction (in hex)",
				},
				"to": map[string]interface{}{
					"type": "string",
					"description": "Contract address to send the transaction to",
				},
				"data": map[string]interface{}{
					"type": "string",
					"description": "Transaction calldata in hex format",
				},
				"from": map[string]interface{}{
					"type": "string",
					"description": "Sender address (must match wallet address)",
				},
				"chainId": map[string]interface{}{
					"type": "number",
					"description": "Chain ID for the transaction",
				},
				"gasPrice": map[string]interface{}{
					"type": "string",
					"description": "Gas price in hex format (optional, will be auto-determined if not provided)",
				},
				"gasLimit": map[string]interface{}{
					"type": "string",
					"description": "Gas limit in hex format (optional, will be estimated if not provided)",
				},
			})),
	)
	mcpServer.AddTool(executeQuoteTool, ExecuteQuoteHandler)

	// 10. GetTokenBalance - Get the balance of a specific token for a wallet
	getTokenBalanceTool := mcp.NewTool(
		"GetTokenBalance",
		mcp.WithDescription("Get the balance of a specific ERC20 token for a wallet address"),
		mcp.WithString("rpcUrl", 
			mcp.Description("The RPC URL of the blockchain node"),
			mcp.Required()),
		mcp.WithString("tokenAddress", 
			mcp.Description("The contract address of the token"),
			mcp.Required()),
		mcp.WithString("walletAddress", 
			mcp.Description("The wallet address to check the balance of"),
			mcp.Required()),
	)
	mcpServer.AddTool(getTokenBalanceTool, GetTokenBalanceHandler)
	
	// 11. ApproveToken - Approve a token for spending by another address
	approveTokenTool := mcp.NewTool(
		"ApproveToken",
		mcp.WithDescription("Approve a specific amount of ERC20 tokens to be spent by another address"),
		mcp.WithString("rpcUrl", 
			mcp.Description("The RPC URL of the blockchain node"),
			mcp.Required()),
		mcp.WithString("tokenAddress", 
			mcp.Description("The contract address of the token"),
			mcp.Required()),
		mcp.WithString("spenderAddress", 
			mcp.Description("The address to approve for spending"),
			mcp.Required()),
		mcp.WithString("amount", 
			mcp.Description("The amount to approve (in token's smallest units)"),
			mcp.Required()),
	)
	mcpServer.AddTool(approveTokenTool, ApproveTokenHandler)
	
	// 12. TransferToken - Transfer ERC20 tokens to another address
	transferTokenTool := mcp.NewTool(
		"TransferToken",
		mcp.WithDescription("Transfer ERC20 tokens to another address"),
		mcp.WithString("rpcUrl", 
			mcp.Description("The RPC URL of the blockchain node"),
			mcp.Required()),
		mcp.WithString("tokenAddress", 
			mcp.Description("The contract address of the token"),
			mcp.Required()),
		mcp.WithString("to", 
			mcp.Description("The address to transfer tokens to"),
			mcp.Required()),
		mcp.WithString("amount", 
			mcp.Description("The amount to transfer (in token's smallest units)"),
			mcp.Required()),
	)
	mcpServer.AddTool(transferTokenTool, TransferTokenHandler)
	
	// 13. TransferNative - Transfer native cryptocurrency to another address
	transferNativeTool := mcp.NewTool(
		"TransferNative",
		mcp.WithDescription("Transfer native cryptocurrency (ETH, BNB, etc.) to another address"),
		mcp.WithString("rpcUrl", 
			mcp.Description("The RPC URL of the blockchain node"),
			mcp.Required()),
		mcp.WithString("to", 
			mcp.Description("The address to transfer to"),
			mcp.Required()),
		mcp.WithString("amount", 
			mcp.Description("The amount to transfer (in wei)"),
			mcp.Required()),
	)
	mcpServer.AddTool(transferNativeTool, TransferNativeHandler)
	
	// 14. GetAllowance - Check the allowance of an ERC20 token for a specific spender
	getAllowanceTool := mcp.NewTool(
		"GetAllowance",
		mcp.WithDescription("Check the allowance of an ERC20 token for a specific spender"),
		mcp.WithString("rpcUrl", 
			mcp.Description("The RPC URL of the blockchain node"),
			mcp.Required()),
		mcp.WithString("tokenAddress", 
			mcp.Description("The contract address of the token"),
			mcp.Required()),
		mcp.WithString("ownerAddress", 
			mcp.Description("The address of the token owner"),
			mcp.Required()),
		mcp.WithString("spenderAddress", 
			mcp.Description("The address of the spender to check allowance for"),
			mcp.Required()),
	)
	mcpServer.AddTool(getAllowanceTool, GetAllowanceHandler)

	// Start the server on stdin/stdout
	fmt.Println("Starting lifi-mcp server...")
	
	// Preload chain data at startup
	fmt.Println("Preloading chain data...")
	err := refreshChainsCache()
	if err != nil {
		log.Printf("Warning: Failed to preload chain data: %v", err)
	} else {
		log.Printf("Successfully loaded %d chains", len(chainsCache.Chains))
	}

	// Handle signals for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\nShutting down...")
		os.Exit(0)
	}()

	// Serve on stdin/stdout
	if err := server.ServeStdio(mcpServer); err != nil {
		log.Fatalf("Error serving MCP: %v", err)
	}
}
