package server

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/mark3labs/mcp-go/mcp"
)

// Read-only blockchain interaction handlers (no private key required)

func (s *Server) getNativeTokenBalanceHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	apiKey := APIKeyFromContext(ctx)

	// Get parameters
	chain := getStringArg(request, "chain")
	rpcUrl := getStringArg(request, "rpcUrl")
	address := getStringArg(request, "address")

	if address == "" {
		return mcp.NewToolResultError("address parameter is required"), nil
	}

	// Resolve RPC URL from chain or use provided rpcUrl
	resolvedRpcUrl, err := s.resolveRpcUrl(ctx, chain, rpcUrl, apiKey)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	rpcUrl = resolvedRpcUrl

	// Validate address format
	if !common.IsHexAddress(address) {
		return mcp.NewToolResultError(fmt.Sprintf("invalid Ethereum address format: %s", address)), nil
	}

	// Connect to the Ethereum client
	client, err := ethclient.Dial(rpcUrl)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to connect to the Ethereum client: %v", err)), nil
	}
	defer client.Close()

	// Convert address string to common.Address
	accountAddress := common.HexToAddress(address)

	// Get the balance
	balance, err := client.BalanceAt(ctx, accountAddress, nil) // nil means latest block
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get balance: %v", err)), nil
	}

	// Get chain ID to determine which token symbol to display
	chainID, err := client.ChainID(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get chain ID: %v", err)), nil
	}

	// Get token symbol from chain data
	symbol, decimals, err := s.getNativeTokenInfo(ctx, chainID, apiKey)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get native token info: %v", err)), nil
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
		return mcp.NewToolResultError(fmt.Sprintf("error serializing result: %v", err)), nil
	}

	return mcp.NewToolResultText(string(jsonResult)), nil
}

func (s *Server) getTokenBalanceHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	apiKey := APIKeyFromContext(ctx)

	// Get parameters
	chain := getStringArg(request, "chain")
	rpcUrl := getStringArg(request, "rpcUrl")
	tokenAddress := getStringArg(request, "tokenAddress")
	walletAddress := getStringArg(request, "walletAddress")

	if tokenAddress == "" || walletAddress == "" {
		return mcp.NewToolResultError("tokenAddress and walletAddress parameters are required"), nil
	}

	// Resolve RPC URL from chain or use provided rpcUrl
	resolvedRpcUrl, err := s.resolveRpcUrl(ctx, chain, rpcUrl, apiKey)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	rpcUrl = resolvedRpcUrl

	// Validate addresses
	if !common.IsHexAddress(tokenAddress) {
		return mcp.NewToolResultError(fmt.Sprintf("invalid token address format: %s", tokenAddress)), nil
	}
	if !common.IsHexAddress(walletAddress) {
		return mcp.NewToolResultError(fmt.Sprintf("invalid wallet address format: %s", walletAddress)), nil
	}

	// Connect to the Ethereum client
	client, err := ethclient.Dial(rpcUrl)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to connect to the Ethereum client: %v", err)), nil
	}
	defer client.Close()

	// Parse the ERC20 ABI
	parsedABI, err := abi.JSON(strings.NewReader(ERC20ABI))
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to parse ERC20 ABI: %v", err)), nil
	}

	// Create common.Address objects
	tokenAddr := common.HexToAddress(tokenAddress)
	walletAddr := common.HexToAddress(walletAddress)

	// Pack the input data for the balanceOf function
	data, err := parsedABI.Pack("balanceOf", walletAddr)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to pack input data: %v", err)), nil
	}

	// Create the call message
	msg := ethereum.CallMsg{
		To:   &tokenAddr,
		Data: data,
	}

	// Call the contract
	result, err := client.CallContract(ctx, msg, nil) // nil means latest block
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to call contract: %v", err)), nil
	}

	// Unpack the result
	var balance *big.Int
	err = parsedABI.UnpackIntoInterface(&balance, "balanceOf", result)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to unpack result: %v", err)), nil
	}

	// Get token information
	tokenSymbol, tokenDecimals, err := getTokenInfo(ctx, client, tokenAddress)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get token info for %s: %v", tokenAddress, err)), nil
	}

	// Get chain ID to include in the response
	chainID, err := client.ChainID(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get chain ID: %v", err)), nil
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
		return mcp.NewToolResultError(fmt.Sprintf("error serializing result: %v", err)), nil
	}

	return mcp.NewToolResultText(string(jsonResponse)), nil
}

func (s *Server) getAllowanceHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	apiKey := APIKeyFromContext(ctx)

	// Get parameters
	chain := getStringArg(request, "chain")
	rpcUrl := getStringArg(request, "rpcUrl")
	tokenAddress := getStringArg(request, "tokenAddress")
	ownerAddress := getStringArg(request, "ownerAddress")
	spenderAddress := getStringArg(request, "spenderAddress")

	// Resolve RPC URL from chain or use provided rpcUrl
	resolvedRpcUrl, err := s.resolveRpcUrl(ctx, chain, rpcUrl, apiKey)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	rpcUrl = resolvedRpcUrl

	if tokenAddress == "" {
		return mcp.NewToolResultError("token address is required"), nil
	}

	if ownerAddress == "" {
		return mcp.NewToolResultError("owner address is required"), nil
	}

	if spenderAddress == "" {
		return mcp.NewToolResultError("spender address is required"), nil
	}

	// Validate addresses
	if !common.IsHexAddress(tokenAddress) {
		return mcp.NewToolResultError(fmt.Sprintf("invalid token address format: %s", tokenAddress)), nil
	}
	if !common.IsHexAddress(ownerAddress) {
		return mcp.NewToolResultError(fmt.Sprintf("invalid owner address format: %s", ownerAddress)), nil
	}
	if !common.IsHexAddress(spenderAddress) {
		return mcp.NewToolResultError(fmt.Sprintf("invalid spender address format: %s", spenderAddress)), nil
	}

	// Connect to the Ethereum client
	client, err := ethclient.Dial(rpcUrl)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to connect to the Ethereum client: %v", err)), nil
	}
	defer client.Close()

	// Parse the ERC20 ABI
	parsedABI, err := abi.JSON(strings.NewReader(ERC20ABI))
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to parse ERC20 ABI: %v", err)), nil
	}

	// Convert addresses to common.Address
	tokenAddr := common.HexToAddress(tokenAddress)
	ownerAddr := common.HexToAddress(ownerAddress)
	spenderAddr := common.HexToAddress(spenderAddress)

	// Pack the allowance function data
	data, err := parsedABI.Pack("allowance", ownerAddr, spenderAddr)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to pack allowance data: %v", err)), nil
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

		return mcp.NewToolResultError(fmt.Sprintf("failed to call allowance: %v. Revert reason: %s", err, revertReason)), nil
	}

	// Unpack the allowance
	var allowance *big.Int
	err = parsedABI.UnpackIntoInterface(&allowance, "allowance", result)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to unpack allowance: %v", err)), nil
	}

	// Get token information for better UX in response
	tokenSymbol, tokenDecimals, err := getTokenInfo(ctx, client, tokenAddress)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get token info for %s: %v", tokenAddress, err)), nil
	}

	// Get chain ID to include in the response
	chainID, err := client.ChainID(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get chain ID: %v", err)), nil
	}

	// Format the response
	responseData := map[string]interface{}{
		"tokenAddress":   tokenAddress,
		"tokenSymbol":    tokenSymbol,
		"ownerAddress":   ownerAddress,
		"spenderAddress": spenderAddress,
		"allowance":      allowance.String(),
		"decimals":       tokenDecimals,
		"chainId":        chainID.String(),
	}

	jsonResponse, err := json.Marshal(responseData)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("error serializing result: %v", err)), nil
	}

	return mcp.NewToolResultText(string(jsonResponse)), nil
}
