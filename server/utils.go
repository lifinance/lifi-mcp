package server

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/mark3labs/mcp-go/mcp"
)

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
func (s *Server) getNativeTokenInfo(ctx context.Context, chainID *big.Int) (string, int, error) {
	// Initialize chains cache if not already done
	chainsCacheMu.RLock()
	initialized := chainsCacheInitialized
	chainsCacheMu.RUnlock()

	if !initialized {
		err := s.refreshChainsCache(ctx)
		if err != nil {
			return "", 0, err
		}
	}

	chainsCacheMu.RLock()
	// Look for the chain in the cache
	chainIDInt := int(chainID.Int64())
	for _, chain := range chainsCache.Chains {
		if chain.ID == chainIDInt {
			// Some chains use nativeToken, others use nativeCurrency
			if chain.NativeToken.Symbol != "" {
				chainsCacheMu.RUnlock()
				return chain.NativeToken.Symbol, chain.NativeToken.Decimals, nil
			}
			if chain.NativeCurrency.Symbol != "" {
				chainsCacheMu.RUnlock()
				return chain.NativeCurrency.Symbol, chain.NativeCurrency.Decimals, nil
			}
			// If neither is available, try getting from metamask
			if chain.Metamask.ChainName != "" {
				symbolParts := strings.Split(chain.Metamask.ChainName, " ")
				if len(symbolParts) > 0 {
					chainsCacheMu.RUnlock()
					return symbolParts[0], 18, nil
				}
			}
		}
	}
	chainsCacheMu.RUnlock()

	// If chain not found in cache, try refreshing the cache once
	err := s.refreshChainsCache(ctx)
	if err != nil {
		return "", 0, err
	}

	chainsCacheMu.RLock()
	defer chainsCacheMu.RUnlock()

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

	return "", 0, fmt.Errorf("chain ID %s not found in Li.Fi API", chainID.String())
}

// resolveRpcUrl resolves an RPC URL from a chain identifier.
// If rpcUrl is provided, it's returned directly.
// If only chain is provided, looks up the RPC URL from chain data.
// The chain parameter can be a numeric ID (e.g., "1") or a name (e.g., "ethereum").
func (s *Server) resolveRpcUrl(ctx context.Context, chain, rpcUrl string) (string, error) {
	// If explicit RPC URL provided, use it
	if rpcUrl != "" {
		return rpcUrl, nil
	}

	// Chain is required if rpcUrl is not provided
	if chain == "" {
		return "", fmt.Errorf("either 'chain' or 'rpcUrl' parameter is required")
	}

	// Initialize chains cache if not already done
	chainsCacheMu.RLock()
	initialized := chainsCacheInitialized
	chainsCacheMu.RUnlock()

	if !initialized {
		if err := s.refreshChainsCache(ctx); err != nil {
			return "", fmt.Errorf("failed to load chain data: %v", err)
		}
	}

	chainsCacheMu.RLock()
	defer chainsCacheMu.RUnlock()

	// Try to parse as numeric chain ID first
	chainID, err := strconv.Atoi(chain)
	if err == nil {
		// It's a numeric ID
		for _, c := range chainsCache.Chains {
			if c.ID == chainID {
				if len(c.Metamask.RpcUrls) > 0 {
					return c.Metamask.RpcUrls[0], nil
				}
				return "", fmt.Errorf("chain %d has no RPC URLs configured", chainID)
			}
		}
		return "", fmt.Errorf("chain ID %d not found", chainID)
	}

	// Try to match by name or key (case-insensitive)
	chainLower := strings.ToLower(chain)
	for _, c := range chainsCache.Chains {
		if strings.ToLower(c.Name) == chainLower ||
			strings.ToLower(c.Key) == chainLower ||
			strings.ToLower(c.Metamask.ChainName) == chainLower {
			if len(c.Metamask.RpcUrls) > 0 {
				return c.Metamask.RpcUrls[0], nil
			}
			return "", fmt.Errorf("chain '%s' has no RPC URLs configured", chain)
		}
	}

	return "", fmt.Errorf("chain '%s' not found", chain)
}

// executeTransactionRequest handles execution of a transaction request object
// that comes directly from the GetQuote response
func (s *Server) executeTransactionRequest(ctx context.Context, txRequest map[string]interface{}, rpcUrl string) (*mcp.CallToolResult, error) {
	// Validate the RPC URL
	if rpcUrl == "" {
		return mcp.NewToolResultError("RPC URL is required"), nil
	}

	// Connect to the Ethereum client
	client, err := ethclient.Dial(rpcUrl)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to connect to the Ethereum client: %v", err)), nil
	}
	defer client.Close()

	// Get chain ID from the client
	networkChainID, err := client.ChainID(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get chain ID: %v", err)), nil
	}

	// Get and validate transaction parameters with proper type checking
	var valuehex, tohex, datahex, fromhex string
	var ok bool

	if v, exists := txRequest["to"]; exists {
		if tohex, ok = v.(string); !ok {
			return mcp.NewToolResultError("transaction 'to' must be a string"), nil
		}
	}
	if tohex == "" {
		return mcp.NewToolResultError("transaction 'to' address is required in transactionRequest"), nil
	}

	if v, exists := txRequest["data"]; exists {
		if datahex, ok = v.(string); !ok {
			return mcp.NewToolResultError("transaction 'data' must be a string"), nil
		}
	}
	if datahex == "" {
		return mcp.NewToolResultError("transaction 'data' is required in transactionRequest"), nil
	}

	if v, exists := txRequest["value"]; exists {
		if valuehex, ok = v.(string); !ok {
			return mcp.NewToolResultError("transaction 'value' must be a string"), nil
		}
	}

	if v, exists := txRequest["from"]; exists {
		if fromhex, ok = v.(string); !ok {
			return mcp.NewToolResultError("transaction 'from' must be a string"), nil
		}
	}

	// Get the wallet address
	walletAddress := crypto.PubkeyToAddress(s.privateKey.PublicKey)

	// If from address is specified, verify it matches our wallet address
	if fromhex != "" && !strings.EqualFold(fromhex, walletAddress.Hex()) {
		return mcp.NewToolResultError(fmt.Sprintf(
			"transaction 'from' address (%s) doesn't match wallet address (%s)",
			fromhex, walletAddress.Hex())), nil
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
			return mcp.NewToolResultError(fmt.Sprintf(
				"chain ID in transaction (%s) doesn't match network chain ID (%s)",
				requestChainID.String(), networkChainID.String())), nil
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
			return mcp.NewToolResultError(fmt.Sprintf("failed to suggest gas price: %v", err)), nil
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
		return mcp.NewToolResultError(fmt.Sprintf("invalid transaction data: %v", err)), nil
	}

	// Parse gas limit or estimate it
	var gasLimitInt uint64
	if gasLimitHex, ok := txRequest["gasLimit"].(string); ok && gasLimitHex != "" {
		if strings.HasPrefix(gasLimitHex, "0x") {
			gasLimitInt64, err := strconv.ParseInt(gasLimitHex[2:], 16, 64)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("invalid gas limit: %s", gasLimitHex)), nil
			}
			gasLimitInt = uint64(gasLimitInt64)
		} else {
			gasLimitInt64, err := strconv.ParseInt(gasLimitHex, 10, 64)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("invalid gas limit: %s", gasLimitHex)), nil
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
			return mcp.NewToolResultError(fmt.Sprintf("failed to estimate gas: %v", err)), nil
		}

		// Add a buffer to the gas limit to avoid out-of-gas errors
		gasLimitInt = uint64(float64(gasLimitInt) * 1.2) // Add 20% buffer
	}

	// Get current nonce
	nonceInt, err := client.PendingNonceAt(ctx, walletAddress)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get nonce: %v", err)), nil
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
	signedTx, err := types.SignTx(tx, types.NewEIP155Signer(requestChainID), s.privateKey)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to sign transaction: %v", err)), nil
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

		return mcp.NewToolResultError(fmt.Sprintf("transaction would fail: %v. Revert reason: %s", err, revertReason)), nil
	}

	// Send the transaction
	err = client.SendTransaction(ctx, signedTx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to send transaction: %v", err)), nil
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
		return mcp.NewToolResultError(fmt.Sprintf("error serializing result: %v", err)), nil
	}

	return mcp.NewToolResultText(string(jsonResult)), nil
}
