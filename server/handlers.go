package server

import (
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/mark3labs/mcp-go/mcp"
)

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

// GetWalletAddress returns the Ethereum address corresponding to the loaded private key
func (s *Server) GetWalletAddress() (string, error) {
	if s.privateKey == nil {
		return "", errors.New("no private key loaded")
	}

	publicKey := crypto.PubkeyToAddress(s.privateKey.PublicKey)
	return publicKey.Hex(), nil
}

// refreshChainsCache fetches the latest chain data from Li.Fi API
func refreshChainsCache() error {
	resp, err := http.Get(fmt.Sprintf("%s/v1/chains", BaseURL))
	if err != nil {
		return fmt.Errorf("failed to fetch chains: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %v", err)
	}

	var chainData ChainData
	err = json.Unmarshal(body, &chainData)
	if err != nil {
		return fmt.Errorf("failed to parse chain data: %v", err)
	}

	chainsCache = chainData
	chainsCacheInitialized = true
	return nil
}

// Helper function to get arguments from request
func getStringArg(request mcp.CallToolRequest, key string) string {
	if args, ok := request.Params.Arguments.(map[string]interface{}); ok {
		if val, exists := args[key]; exists {
			if str, ok := val.(string); ok {
				return str
			}
		}
	}
	return ""
}

func getArrayArg(request mcp.CallToolRequest, key string) []interface{} {
	if args, ok := request.Params.Arguments.(map[string]interface{}); ok {
		if val, exists := args[key]; exists {
			if arr, ok := val.([]interface{}); ok {
				return arr
			}
		}
	}
	return nil
}

func getObjectArg(request mcp.CallToolRequest, key string) map[string]interface{} {
	if args, ok := request.Params.Arguments.(map[string]interface{}); ok {
		if val, exists := args[key]; exists {
			if obj, ok := val.(map[string]interface{}); ok {
				return obj
			}
		}
	}
	return nil
}

// LiFi API handlers
func (s *Server) getTokensHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	chains := getStringArg(request, "chains")
	chainTypes := getStringArg(request, "chainTypes")
	minPriceUSD := getStringArg(request, "minPriceUSD")

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
		return nil, fmt.Errorf("error making request: %v", err)
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %v", err)
	}

	return mcp.NewToolResultText(string(body)), nil
}

func (s *Server) getTokenHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	chain := getStringArg(request, "chain")
	token := getStringArg(request, "token")

	if chain == "" || token == "" {
		return nil, errors.New("both chain and token parameters are required")
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
		return nil, fmt.Errorf("error making request: %v", err)
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %v", err)
	}

	return mcp.NewToolResultText(string(body)), nil
}

func (s *Server) getQuoteHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Get all required parameters
	fromChain := getStringArg(request, "fromChain")
	toChain := getStringArg(request, "toChain")
	fromToken := getStringArg(request, "fromToken")
	toToken := getStringArg(request, "toToken")
	fromAddress := getStringArg(request, "fromAddress")
	fromAmount := getStringArg(request, "fromAmount")

	// Required parameters check
	if fromChain == "" || toChain == "" || fromToken == "" || toToken == "" || fromAddress == "" || fromAmount == "" {
		return nil, errors.New("required parameters: fromChain, toChain, fromToken, toToken, fromAddress, fromAmount")
	}

	// Get optional parameters
	toAddress := getStringArg(request, "toAddress")
	slippage := getStringArg(request, "slippage")
	integrator := getStringArg(request, "integrator")
	order := getStringArg(request, "order")

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
	if allowBridges := getArrayArg(request, "allowBridges"); allowBridges != nil {
		bridgesList := make([]string, len(allowBridges))
		for i, bridge := range allowBridges {
			bridgesList[i] = fmt.Sprintf("%v", bridge)
		}
		params.Add("allowBridges", strings.Join(bridgesList, ","))
	}

	if allowExchanges := getArrayArg(request, "allowExchanges"); allowExchanges != nil {
		exchangesList := make([]string, len(allowExchanges))
		for i, exchange := range allowExchanges {
			exchangesList[i] = fmt.Sprintf("%v", exchange)
		}
		params.Add("allowExchanges", strings.Join(exchangesList, ","))
	}

	// Build the request URL
	requestURL := fmt.Sprintf("%s/v1/quote?%s", BaseURL, params.Encode())

	// Make the request
	resp, err := http.Get(requestURL)
	if err != nil {
		return nil, fmt.Errorf("error making request: %v", err)
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %v", err)
	}

	return mcp.NewToolResultText(string(body)), nil
}

func (s *Server) getStatusHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	txHash := getStringArg(request, "txHash")

	if txHash == "" {
		return nil, errors.New("txHash parameter is required")
	}

	// Get optional parameters
	bridge := getStringArg(request, "bridge")
	fromChain := getStringArg(request, "fromChain")
	toChain := getStringArg(request, "toChain")

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
		return nil, fmt.Errorf("error making request: %v", err)
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %v", err)
	}

	return mcp.NewToolResultText(string(body)), nil
}

func (s *Server) getChainsHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	chainTypes := getStringArg(request, "chainTypes")

	// Ensure the chains are loaded
	if !chainsCacheInitialized {
		err := refreshChainsCache()
		if err != nil {
			return nil, fmt.Errorf("failed to fetch chain data: %v", err)
		}
	}

	// If no chain types filter is specified, return all chains
	if chainTypes == "" {
		jsonData, err := json.Marshal(chainsCache)
		if err != nil {
			return nil, fmt.Errorf("error serializing chain data: %v", err)
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
			return nil, fmt.Errorf("error making request: %v", err)
		}
		defer resp.Body.Close()

		// Read the response body
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("error reading response: %v", err)
		}

		return mcp.NewToolResultText(string(body)), nil
	}

	// Return the filtered chains
	jsonData, err := json.Marshal(filteredChains)
	if err != nil {
		return nil, fmt.Errorf("error serializing filtered chain data: %v", err)
	}
	return mcp.NewToolResultText(string(jsonData)), nil
}

func (s *Server) getConnectionsHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Get parameters
	fromChain := getStringArg(request, "fromChain")
	toChain := getStringArg(request, "toChain")
	fromToken := getStringArg(request, "fromToken")
	toToken := getStringArg(request, "toToken")
	chainTypes := getStringArg(request, "chainTypes")

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
	if allowBridges := getArrayArg(request, "allowBridges"); allowBridges != nil {
		bridgesList := make([]string, len(allowBridges))
		for i, bridge := range allowBridges {
			bridgesList[i] = fmt.Sprintf("%v", bridge)
		}
		encodedBridges, _ := json.Marshal(bridgesList)
		params.Add("allowBridges", string(encodedBridges))
	}

	// Build the request URL
	requestURL := fmt.Sprintf("%s/v1/connections?%s", BaseURL, params.Encode())

	// Make the request
	resp, err := http.Get(requestURL)
	if err != nil {
		return nil, fmt.Errorf("error making request: %v", err)
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %v", err)
	}

	return mcp.NewToolResultText(string(body)), nil
}

func (s *Server) getToolsHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Get parameters (chains is an array)
	var chains []string

	if chainsParam := getArrayArg(request, "chains"); chainsParam != nil {
		chains = make([]string, len(chainsParam))
		for i, chain := range chainsParam {
			chains[i] = fmt.Sprintf("%v", chain)
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
		return nil, fmt.Errorf("error making request: %v", err)
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %v", err)
	}

	return mcp.NewToolResultText(string(body)), nil
}

func (s *Server) getChainByIdHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Get required parameter
	idStr := getStringArg(request, "id")

	if idStr == "" {
		return nil, errors.New("ID parameter is required")
	}

	// Attempt to parse the ID as an integer
	var id int
	_, err := fmt.Sscanf(idStr, "%d", &id)
	if err != nil {
		return nil, fmt.Errorf("invalid ID format. Expected integer, got: %s", idStr)
	}

	// Ensure the chains are loaded
	if !chainsCacheInitialized {
		err := refreshChainsCache()
		if err != nil {
			return nil, fmt.Errorf("failed to fetch chain data: %v", err)
		}
	}

	// Look for the chain by ID
	for _, chain := range chainsCache.Chains {
		if chain.ID == id {
			// Found a match, return the chain data
			chainData, err := json.Marshal(chain)
			if err != nil {
				return nil, fmt.Errorf("error serializing chain data: %v", err)
			}

			return mcp.NewToolResultText(string(chainData)), nil
		}
	}

	// No matching chain found
	return nil, fmt.Errorf("no chain found with ID: %d", id)
}

func (s *Server) getChainByNameHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Get required parameter
	name := getStringArg(request, "name")

	if name == "" {
		return nil, errors.New("name parameter is required")
	}

	// Ensure the chains are loaded
	if !chainsCacheInitialized {
		err := refreshChainsCache()
		if err != nil {
			return nil, fmt.Errorf("failed to fetch chain data: %v", err)
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
				return nil, fmt.Errorf("error serializing chain data: %v", err)
			}

			return mcp.NewToolResultText(string(chainData)), nil
		}
	}

	// No matching chain found
	return nil, fmt.Errorf("no chain found matching name: %s", name)
}

func (s *Server) getWalletAddressHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	address, err := s.GetWalletAddress()
	if err != nil {
		return nil, fmt.Errorf("error getting wallet address: %v", err)
	}

	result := map[string]string{
		"address": address,
	}

	jsonResult, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("error serializing result: %v", err)
	}

	return mcp.NewToolResultText(string(jsonResult)), nil
}
