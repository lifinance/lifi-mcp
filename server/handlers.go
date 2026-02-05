package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

// Helper function to get arguments from request - using new mcp.ParseString
func getStringArg(request mcp.CallToolRequest, key string) string {
	return mcp.ParseString(request, key, "")
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
	return mcp.ParseStringMap(request, key, nil)
}

// healthCheckHandler returns the health status of the server
func (s *Server) healthCheckHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	apiKey := APIKeyFromContext(ctx)

	health := map[string]interface{}{
		"status":    "healthy",
		"version":   s.version,
		"hasAPIKey": apiKey != "",
	}

	// Check API connectivity with a simple chains request
	apiStatus := "connected"
	_, err := s.httpClient.Get(ctx, fmt.Sprintf("%s/v1/chains?limit=1", BaseURL), apiKey)
	if err != nil {
		apiStatus = fmt.Sprintf("error: %v", err)
		health["status"] = "degraded"
	}
	health["apiStatus"] = apiStatus

	jsonResult, err := json.Marshal(health)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("error serializing health status: %v", err)), nil
	}

	return mcp.NewToolResultText(string(jsonResult)), nil
}

// LiFi API handlers
func (s *Server) getTokensHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	apiKey := APIKeyFromContext(ctx)

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
	body, err := s.httpClient.Get(ctx, requestURL, apiKey)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("error making request: %v", err)), nil
	}

	return mcp.NewToolResultText(string(body)), nil
}

func (s *Server) getTokenHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	apiKey := APIKeyFromContext(ctx)

	chain := getStringArg(request, "chain")
	token := getStringArg(request, "token")

	if chain == "" || token == "" {
		return mcp.NewToolResultError("both chain and token parameters are required"), nil
	}

	// Build the query parameters
	params := url.Values{}
	params.Add("chain", chain)
	params.Add("token", token)

	// Build the request URL
	requestURL := fmt.Sprintf("%s/v1/token?%s", BaseURL, params.Encode())

	// Make the request
	body, err := s.httpClient.Get(ctx, requestURL, apiKey)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("error making request: %v", err)), nil
	}

	return mcp.NewToolResultText(string(body)), nil
}

func (s *Server) getQuoteHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	apiKey := APIKeyFromContext(ctx)

	// Get all required parameters
	fromChain := getStringArg(request, "fromChain")
	toChain := getStringArg(request, "toChain")
	fromToken := getStringArg(request, "fromToken")
	toToken := getStringArg(request, "toToken")
	fromAddress := getStringArg(request, "fromAddress")
	fromAmount := getStringArg(request, "fromAmount")

	// Validate required parameters
	if err := ValidateChainID("fromChain", fromChain); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	if err := ValidateChainID("toChain", toChain); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	if err := ValidateTokenAddress("fromToken", fromToken); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	if err := ValidateTokenAddress("toToken", toToken); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	if err := ValidateAddress("fromAddress", fromAddress); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	if err := ValidateAmount("fromAmount", fromAmount); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Get optional parameters
	toAddress := getStringArg(request, "toAddress")
	slippage := getStringArg(request, "slippage")
	integrator := getStringArg(request, "integrator")
	order := getStringArg(request, "order")

	// Validate optional parameters
	if toAddress != "" {
		if err := ValidateRecipientAddress("toAddress", toAddress); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
	}
	if err := ValidateSlippage(slippage); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

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
	body, err := s.httpClient.Get(ctx, requestURL, apiKey)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("error making request: %v", err)), nil
	}

	return mcp.NewToolResultText(string(body)), nil
}

func (s *Server) getStatusHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	apiKey := APIKeyFromContext(ctx)

	txHash := getStringArg(request, "txHash")

	if txHash == "" {
		return mcp.NewToolResultError("txHash parameter is required"), nil
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
	body, err := s.httpClient.Get(ctx, requestURL, apiKey)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("error making request: %v", err)), nil
	}

	return mcp.NewToolResultText(string(body)), nil
}

func (s *Server) getChainsHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	apiKey := APIKeyFromContext(ctx)

	chainTypes := getStringArg(request, "chainTypes")

	// Ensure the chains are loaded
	chainsCacheMu.RLock()
	initialized := chainsCacheInitialized
	chainsCacheMu.RUnlock()

	if !initialized {
		err := s.refreshChainsCache(ctx, apiKey)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to fetch chain data: %v", err)), nil
		}
	}

	chainsCacheMu.RLock()
	defer chainsCacheMu.RUnlock()

	// If no chain types filter is specified, return all chains
	if chainTypes == "" {
		jsonData, err := json.Marshal(chainsCache)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("error serializing chain data: %v", err)), nil
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
		body, err := s.httpClient.Get(ctx, requestURL, apiKey)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("error making request: %v", err)), nil
		}

		return mcp.NewToolResultText(string(body)), nil
	}

	// Return the filtered chains
	jsonData, err := json.Marshal(filteredChains)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("error serializing filtered chain data: %v", err)), nil
	}
	return mcp.NewToolResultText(string(jsonData)), nil
}

func (s *Server) getConnectionsHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	apiKey := APIKeyFromContext(ctx)

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
	body, err := s.httpClient.Get(ctx, requestURL, apiKey)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("error making request: %v", err)), nil
	}

	return mcp.NewToolResultText(string(body)), nil
}

func (s *Server) getToolsHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	apiKey := APIKeyFromContext(ctx)

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
	body, err := s.httpClient.Get(ctx, requestURL, apiKey)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("error making request: %v", err)), nil
	}

	// Parse the response to filter out unnecessary fields
	var toolsResponse map[string]interface{}
	if err := json.Unmarshal(body, &toolsResponse); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to parse tools response: %v", err)), nil
	}

	// Create filtered response with only key and name for bridges and exchanges
	filteredResponse := make(map[string]interface{})

	// Process bridges if present
	if bridges, ok := toolsResponse["bridges"].([]interface{}); ok {
		filteredBridges := make([]map[string]interface{}, 0, len(bridges))
		for _, bridge := range bridges {
			if bridgeMap, ok := bridge.(map[string]interface{}); ok {
				filtered := make(map[string]interface{})
				if key, exists := bridgeMap["key"]; exists {
					filtered["key"] = key
				}
				if name, exists := bridgeMap["name"]; exists {
					filtered["name"] = name
				}
				filteredBridges = append(filteredBridges, filtered)
			}
		}
		filteredResponse["bridges"] = filteredBridges
	}

	// Process exchanges if present
	if exchanges, ok := toolsResponse["exchanges"].([]interface{}); ok {
		filteredExchanges := make([]map[string]interface{}, 0, len(exchanges))
		for _, exchange := range exchanges {
			if exchangeMap, ok := exchange.(map[string]interface{}); ok {
				filtered := make(map[string]interface{})
				if key, exists := exchangeMap["key"]; exists {
					filtered["key"] = key
				}
				if name, exists := exchangeMap["name"]; exists {
					filtered["name"] = name
				}
				filteredExchanges = append(filteredExchanges, filtered)
			}
		}
		filteredResponse["exchanges"] = filteredExchanges
	}

	// Marshal the filtered response
	filteredBody, err := json.Marshal(filteredResponse)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to serialize filtered tools response: %v", err)), nil
	}

	return mcp.NewToolResultText(string(filteredBody)), nil
}

func (s *Server) getChainByIdHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	apiKey := APIKeyFromContext(ctx)

	// Get required parameter
	idStr := getStringArg(request, "id")

	if idStr == "" {
		return mcp.NewToolResultError("ID parameter is required"), nil
	}

	// Attempt to parse the ID as an integer
	var id int
	_, err := fmt.Sscanf(idStr, "%d", &id)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("invalid ID format. Expected integer, got: %s", idStr)), nil
	}

	// Ensure the chains are loaded
	chainsCacheMu.RLock()
	initialized := chainsCacheInitialized
	chainsCacheMu.RUnlock()

	if !initialized {
		err := s.refreshChainsCache(ctx, apiKey)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to fetch chain data: %v", err)), nil
		}
	}

	chainsCacheMu.RLock()
	defer chainsCacheMu.RUnlock()

	// Look for the chain by ID
	for _, chain := range chainsCache.Chains {
		if chain.ID == id {
			// Found a match, return the chain data
			chainData, err := json.Marshal(chain)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("error serializing chain data: %v", err)), nil
			}

			return mcp.NewToolResultText(string(chainData)), nil
		}
	}

	// No matching chain found
	return mcp.NewToolResultError(fmt.Sprintf("no chain found with ID: %d", id)), nil
}

func (s *Server) getChainByNameHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	apiKey := APIKeyFromContext(ctx)

	// Get required parameter
	name := getStringArg(request, "name")

	if name == "" {
		return mcp.NewToolResultError("name parameter is required"), nil
	}

	// Ensure the chains are loaded
	chainsCacheMu.RLock()
	initialized := chainsCacheInitialized
	chainsCacheMu.RUnlock()

	if !initialized {
		err := s.refreshChainsCache(ctx, apiKey)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to fetch chain data: %v", err)), nil
		}
	}

	chainsCacheMu.RLock()
	defer chainsCacheMu.RUnlock()

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
				return mcp.NewToolResultError(fmt.Sprintf("error serializing chain data: %v", err)), nil
			}

			return mcp.NewToolResultText(string(chainData)), nil
		}
	}

	// No matching chain found
	return mcp.NewToolResultError(fmt.Sprintf("no chain found matching name: %s", name)), nil
}

func (s *Server) getRoutesHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	apiKey := APIKeyFromContext(ctx)

	// Get all required parameters
	fromChainId := getStringArg(request, "fromChainId")
	toChainId := getStringArg(request, "toChainId")
	fromTokenAddress := getStringArg(request, "fromTokenAddress")
	toTokenAddress := getStringArg(request, "toTokenAddress")
	fromAddress := getStringArg(request, "fromAddress")
	fromAmount := getStringArg(request, "fromAmount")

	// Validate required parameters
	if err := ValidateChainID("fromChainId", fromChainId); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	if err := ValidateChainID("toChainId", toChainId); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	if err := ValidateTokenAddress("fromTokenAddress", fromTokenAddress); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	if err := ValidateTokenAddress("toTokenAddress", toTokenAddress); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	if err := ValidateAddress("fromAddress", fromAddress); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	if err := ValidateAmount("fromAmount", fromAmount); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Get optional parameters
	toAddress := getStringArg(request, "toAddress")
	slippage := getStringArg(request, "slippage")
	order := getStringArg(request, "order")

	// Validate optional parameters
	if toAddress != "" {
		if err := ValidateRecipientAddress("toAddress", toAddress); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
	}
	if err := ValidateSlippage(slippage); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Build the request body
	requestBody := map[string]interface{}{
		"fromChainId":      fromChainId,
		"toChainId":        toChainId,
		"fromTokenAddress": fromTokenAddress,
		"toTokenAddress":   toTokenAddress,
		"fromAddress":      fromAddress,
		"fromAmount":       fromAmount,
	}

	if toAddress != "" {
		requestBody["toAddress"] = toAddress
	}
	if slippage != "" {
		requestBody["slippage"] = slippage
	}
	if order != "" {
		requestBody["options"] = map[string]interface{}{
			"order": order,
		}
	}

	// Marshal the request body
	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal request body: %v", err)), nil
	}

	// Build the request URL
	requestURL := fmt.Sprintf("%s/v1/advanced/routes", BaseURL)

	// Make the POST request
	body, err := s.httpClient.Post(ctx, requestURL, jsonBody, apiKey)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("error making request: %v", err)), nil
	}

	return mcp.NewToolResultText(string(body)), nil
}

func (s *Server) getQuoteWithCallsHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	apiKey := APIKeyFromContext(ctx)

	// Get all required parameters
	fromChain := getStringArg(request, "fromChain")
	toChain := getStringArg(request, "toChain")
	fromToken := getStringArg(request, "fromToken")
	toToken := getStringArg(request, "toToken")
	fromAddress := getStringArg(request, "fromAddress")
	fromAmount := getStringArg(request, "fromAmount")
	contractCalls := getArrayArg(request, "contractCalls")

	// Required parameters check
	if fromChain == "" || toChain == "" || fromToken == "" || toToken == "" || fromAddress == "" || fromAmount == "" {
		return mcp.NewToolResultError("required parameters: fromChain, toChain, fromToken, toToken, fromAddress, fromAmount"), nil
	}

	if contractCalls == nil || len(contractCalls) == 0 {
		return mcp.NewToolResultError("contractCalls array is required and must not be empty"), nil
	}

	// Get optional parameters
	slippage := getStringArg(request, "slippage")

	// Build the request body
	requestBody := map[string]interface{}{
		"fromChain":     fromChain,
		"toChain":       toChain,
		"fromToken":     fromToken,
		"toToken":       toToken,
		"fromAddress":   fromAddress,
		"fromAmount":    fromAmount,
		"contractCalls": contractCalls,
	}

	if slippage != "" {
		requestBody["slippage"] = slippage
	}

	// Marshal the request body
	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal request body: %v", err)), nil
	}

	// Build the request URL
	requestURL := fmt.Sprintf("%s/v1/quote/contractCalls", BaseURL)

	// Make the POST request
	body, err := s.httpClient.Post(ctx, requestURL, jsonBody, apiKey)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("error making request: %v", err)), nil
	}

	return mcp.NewToolResultText(string(body)), nil
}

func (s *Server) getStepTransactionHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	apiKey := APIKeyFromContext(ctx)

	// Get the step object
	step := getObjectArg(request, "step")

	if step == nil {
		return mcp.NewToolResultError("step object is required"), nil
	}

	// Marshal the step object for the request body
	jsonBody, err := json.Marshal(step)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal step object: %v", err)), nil
	}

	// Build the request URL
	requestURL := fmt.Sprintf("%s/v1/advanced/stepTransaction", BaseURL)

	// Make the POST request
	body, err := s.httpClient.Post(ctx, requestURL, jsonBody, apiKey)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("error making request: %v", err)), nil
	}

	return mcp.NewToolResultText(string(body)), nil
}

func (s *Server) getGasPricesHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	apiKey := APIKeyFromContext(ctx)

	// Build the request URL
	requestURL := fmt.Sprintf("%s/v1/gas/prices", BaseURL)

	// Make the request
	body, err := s.httpClient.Get(ctx, requestURL, apiKey)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("error making request: %v", err)), nil
	}

	return mcp.NewToolResultText(string(body)), nil
}

func (s *Server) getGasSuggestionHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	apiKey := APIKeyFromContext(ctx)

	// Get required parameter
	chainId := getStringArg(request, "chainId")

	if chainId == "" {
		return mcp.NewToolResultError("chainId parameter is required"), nil
	}

	// Validate chainId is numeric to prevent path injection
	if _, err := strconv.Atoi(chainId); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("chainId must be numeric, got: %s", chainId)), nil
	}

	// Build the request URL with path escaping for safety
	requestURL := fmt.Sprintf("%s/v1/gas/suggestion/%s", BaseURL, url.PathEscape(chainId))

	// Make the request
	body, err := s.httpClient.Get(ctx, requestURL, apiKey)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("error making request: %v", err)), nil
	}

	return mcp.NewToolResultText(string(body)), nil
}

func (s *Server) testApiKeyHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	apiKey := APIKeyFromContext(ctx)

	if apiKey == "" {
		return mcp.NewToolResultError("No API key provided. Pass your LI.FI API key via Authorization header (Bearer token) or X-LiFi-Api-Key header."), nil
	}

	requestURL := fmt.Sprintf("%s/v1/keys/test", BaseURL)
	body, err := s.httpClient.Get(ctx, requestURL, apiKey)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("API key test failed: %v", err)), nil
	}

	return mcp.NewToolResultText(string(body)), nil
}
