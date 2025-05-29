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
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/mark3labs/mcp-go/mcp"
)

// Blockchain interaction handlers

func (s *Server) getNativeTokenBalanceHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Get required parameters
	rpcUrl := getStringArg(request, "rpcUrl")
	address := getStringArg(request, "address")

	if rpcUrl == "" || address == "" {
		return mcp.NewToolResultError("both rpcUrl and address parameters are required"), nil
	}

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
		return mcp.NewToolResultError(fmt.Sprintf("error serializing result: %v", err)), nil
	}

	return mcp.NewToolResultText(string(jsonResult)), nil
}

func (s *Server) getTokenBalanceHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Get required parameters
	rpcUrl := getStringArg(request, "rpcUrl")
	tokenAddress := getStringArg(request, "tokenAddress")
	walletAddress := getStringArg(request, "walletAddress")

	if rpcUrl == "" || tokenAddress == "" || walletAddress == "" {
		return mcp.NewToolResultError("rpcUrl, tokenAddress, and walletAddress parameters are required"), nil
	}

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
		return mcp.NewToolResultError(fmt.Sprintf("error serializing result: %v", err)), nil
	}

	return mcp.NewToolResultText(string(jsonResponse)), nil
}

func (s *Server) getAllowanceHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Get required parameters
	rpcUrl := getStringArg(request, "rpcUrl")
	tokenAddress := getStringArg(request, "tokenAddress")
	ownerAddress := getStringArg(request, "ownerAddress")
	spenderAddress := getStringArg(request, "spenderAddress")

	// Validate required parameters individually for better error messages
	if rpcUrl == "" {
		return mcp.NewToolResultError("RPC URL is required"), nil
	}

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

func (s *Server) executeQuoteHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Check if private key is loaded
	if s.privateKey == nil {
		return mcp.NewToolResultError("no private key loaded. Please start the server with a keystore"), nil
	}

	// Get RPC URL (required)
	rpcUrl := getStringArg(request, "rpcUrl")
	if rpcUrl == "" {
		return mcp.NewToolResultError("RPC URL is required"), nil
	}

	// Get transactionRequest object (required)
	txRequest := getObjectArg(request, "transactionRequest")
	if txRequest == nil {
		return mcp.NewToolResultError("transaction request object is required"), nil
	}

	// Execute the transaction
	return s.executeTransactionRequest(ctx, txRequest, rpcUrl)
}

func (s *Server) approveTokenHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Check if private key is loaded
	if s.privateKey == nil {
		return mcp.NewToolResultError("no private key loaded. Please start the server with a keystore"), nil
	}

	// Get required parameters
	rpcUrl := getStringArg(request, "rpcUrl")
	tokenAddress := getStringArg(request, "tokenAddress")
	spenderAddress := getStringArg(request, "spenderAddress")
	amountStr := getStringArg(request, "amount")

	// Validate required parameters individually for better error messages
	if rpcUrl == "" {
		return mcp.NewToolResultError("RPC URL is required"), nil
	}

	if tokenAddress == "" {
		return mcp.NewToolResultError("token address is required"), nil
	}

	if spenderAddress == "" {
		return mcp.NewToolResultError("spender address is required"), nil
	}

	if amountStr == "" {
		return mcp.NewToolResultError("amount is required"), nil
	}

	// Validate addresses
	if !common.IsHexAddress(tokenAddress) {
		return mcp.NewToolResultError(fmt.Sprintf("invalid token address format: %s", tokenAddress)), nil
	}
	if !common.IsHexAddress(spenderAddress) {
		return mcp.NewToolResultError(fmt.Sprintf("invalid spender address format: %s", spenderAddress)), nil
	}

	// Parse amount
	amount := new(big.Int)
	amount, ok := amount.SetString(amountStr, 10)
	if !ok {
		return mcp.NewToolResultError(fmt.Sprintf("invalid amount format: %s", amountStr)), nil
	}

	// Connect to the Ethereum client
	client, err := ethclient.Dial(rpcUrl)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to connect to the Ethereum client: %v", err)), nil
	}
	defer client.Close()

	// Get chain ID
	chainID, err := client.ChainID(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get chain ID: %v", err)), nil
	}

	// Parse the ERC20 ABI
	parsedABI, err := abi.JSON(strings.NewReader(ERC20ABI))
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to parse ERC20 ABI: %v", err)), nil
	}

	// Get the wallet address from the private key
	walletAddress := crypto.PubkeyToAddress(s.privateKey.PublicKey)

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
		return mcp.NewToolResultError(fmt.Sprintf("failed to pack approve data: %v", err)), nil
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

		return mcp.NewToolResultError(fmt.Sprintf("approval would fail: %v. Revert reason: %s", err, revertReason)), nil
	}

	// Estimate gas for the transaction
	gasLimit, err := client.EstimateGas(ctx, ethereum.CallMsg{
		From: walletAddress,
		To:   &tokenAddr,
		Data: data,
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to estimate gas: %v", err)), nil
	}

	// Add a buffer to the gas limit for safety
	gasLimit = uint64(float64(gasLimit) * 1.2)

	// Get nonce
	nonce, err := client.PendingNonceAt(ctx, walletAddress)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get nonce: %v", err)), nil
	}

	// Get EIP-1559 fee suggestions
	head, err := client.HeaderByNumber(ctx, nil)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get latest block header: %v", err)), nil
	}

	// Check if the network supports EIP-1559
	var tx *types.Transaction
	if head.BaseFee != nil {
		// EIP-1559 transaction
		// Get fee suggestions
		gasTipCap, err := client.SuggestGasTipCap(ctx)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to suggest gas tip cap: %v", err)), nil
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
			return mcp.NewToolResultError(fmt.Sprintf("failed to suggest gas price: %v", err)), nil
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
	signedTx, err := types.SignTx(tx, types.LatestSignerForChainID(chainID), s.privateKey)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to sign transaction: %v", err)), nil
	}

	// Send the transaction
	err = client.SendTransaction(ctx, signedTx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to send transaction: %v", err)), nil
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
		return mcp.NewToolResultError(fmt.Sprintf("error serializing result: %v", err)), nil
	}

	return mcp.NewToolResultText(string(jsonResponse)), nil
}

func (s *Server) transferTokenHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Check if private key is loaded
	if s.privateKey == nil {
		return mcp.NewToolResultError("no private key loaded. Please start the server with a keystore"), nil
	}

	// Get required parameters
	rpcUrl := getStringArg(request, "rpcUrl")
	tokenAddress := getStringArg(request, "tokenAddress")
	recipientAddress := getStringArg(request, "to")
	amountStr := getStringArg(request, "amount")

	// Validate required parameters individually for better error messages
	if rpcUrl == "" {
		return mcp.NewToolResultError("RPC URL is required"), nil
	}

	if tokenAddress == "" {
		return mcp.NewToolResultError("token address is required"), nil
	}

	if recipientAddress == "" {
		return mcp.NewToolResultError("recipient address (to) is required"), nil
	}

	if amountStr == "" {
		return mcp.NewToolResultError("amount is required"), nil
	}

	// Validate addresses
	if !common.IsHexAddress(tokenAddress) {
		return mcp.NewToolResultError(fmt.Sprintf("invalid token address format: %s", tokenAddress)), nil
	}
	if !common.IsHexAddress(recipientAddress) {
		return mcp.NewToolResultError(fmt.Sprintf("invalid recipient address format: %s", recipientAddress)), nil
	}

	// Parse amount
	amount := new(big.Int)
	amount, ok := amount.SetString(amountStr, 10)
	if !ok {
		return mcp.NewToolResultError(fmt.Sprintf("invalid amount format: %s", amountStr)), nil
	}

	// Connect to the Ethereum client
	client, err := ethclient.Dial(rpcUrl)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to connect to the Ethereum client: %v", err)), nil
	}
	defer client.Close()

	// Get chain ID
	chainID, err := client.ChainID(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get chain ID: %v", err)), nil
	}

	// Parse the ERC20 ABI
	parsedABI, err := abi.JSON(strings.NewReader(ERC20ABI))
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to parse ERC20 ABI: %v", err)), nil
	}

	// Get the wallet address from the private key
	walletAddress := crypto.PubkeyToAddress(s.privateKey.PublicKey)

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
		return mcp.NewToolResultError(fmt.Sprintf("failed to pack balanceOf data: %v", err)), nil
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

		return mcp.NewToolResultError(fmt.Sprintf("failed to call balanceOf: %v. Revert reason: %s", err, revertReason)), nil
	}

	// Unpack the balance
	var balance *big.Int
	err = parsedABI.UnpackIntoInterface(&balance, "balanceOf", balanceResult)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to unpack balance: %v", err)), nil
	}

	// Check if the balance is sufficient
	if balance.Cmp(amount) < 0 {
		return mcp.NewToolResultError(fmt.Sprintf(
			"insufficient token balance: have %s, need %s", balance.String(), amount.String())), nil
	}

	// Pack the transfer function data
	data, err := parsedABI.Pack("transfer", common.HexToAddress(recipientAddress), amount)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to pack transfer data: %v", err)), nil
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

		return mcp.NewToolResultError(fmt.Sprintf("transfer would fail: %v. Revert reason: %s", err, revertReason)), nil
	}

	// Estimate gas for the transaction
	gasLimit, err := client.EstimateGas(ctx, ethereum.CallMsg{
		From: walletAddress,
		To:   &tokenAddr,
		Data: data,
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to estimate gas: %v", err)), nil
	}

	// Add a buffer to the gas limit for safety
	gasLimit = uint64(float64(gasLimit) * 1.2)

	// Get nonce
	nonce, err := client.PendingNonceAt(ctx, walletAddress)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get nonce: %v", err)), nil
	}

	// Get latest block header to check for EIP-1559 support
	head, err := client.HeaderByNumber(ctx, nil)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get latest block header: %v", err)), nil
	}

	// Create and sign the transaction based on EIP-1559 support
	var tx *types.Transaction
	if head.BaseFee != nil {
		// EIP-1559 transaction
		// Get fee suggestions
		gasTipCap, err := client.SuggestGasTipCap(ctx)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to suggest gas tip cap: %v", err)), nil
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
			return mcp.NewToolResultError(fmt.Sprintf("failed to suggest gas price: %v", err)), nil
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
	signedTx, err := types.SignTx(tx, types.LatestSignerForChainID(chainID), s.privateKey)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to sign transaction: %v", err)), nil
	}

	// Send the transaction
	err = client.SendTransaction(ctx, signedTx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to send transaction: %v", err)), nil
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
		return mcp.NewToolResultError(fmt.Sprintf("error serializing result: %v", err)), nil
	}

	return mcp.NewToolResultText(string(jsonResponse)), nil
}

func (s *Server) transferNativeHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Check if private key is loaded
	if s.privateKey == nil {
		return mcp.NewToolResultError("no private key loaded. Please start the server with a keystore"), nil
	}

	// Get required parameters
	rpcUrl := getStringArg(request, "rpcUrl")
	recipientAddress := getStringArg(request, "to")
	amountStr := getStringArg(request, "amount")

	// Validate required parameters individually for better error messages
	if rpcUrl == "" {
		return mcp.NewToolResultError("RPC URL is required"), nil
	}

	if recipientAddress == "" {
		return mcp.NewToolResultError("recipient address (to) is required"), nil
	}

	if amountStr == "" {
		return mcp.NewToolResultError("amount is required"), nil
	}

	// Validate recipient address
	if !common.IsHexAddress(recipientAddress) {
		return mcp.NewToolResultError(fmt.Sprintf("invalid recipient address format: %s", recipientAddress)), nil
	}

	// Parse amount
	amount := new(big.Int)
	amount, ok := amount.SetString(amountStr, 10)
	if !ok {
		return mcp.NewToolResultError(fmt.Sprintf("invalid amount format: %s", amountStr)), nil
	}

	// Connect to the Ethereum client
	client, err := ethclient.Dial(rpcUrl)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to connect to the Ethereum client: %v", err)), nil
	}
	defer client.Close()

	// Get chain ID
	chainID, err := client.ChainID(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get chain ID: %v", err)), nil
	}

	// Get the wallet address from the private key
	walletAddress := crypto.PubkeyToAddress(s.privateKey.PublicKey)

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
		return mcp.NewToolResultError(fmt.Sprintf("failed to get wallet balance: %v", err)), nil
	}

	// Standard gas for ETH transfer is 21000
	gasLimit := uint64(21000)

	// Get latest block header to check for EIP-1559 support
	head, err := client.HeaderByNumber(ctx, nil)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get latest block header: %v", err)), nil
	}

	// Calculate gas cost based on network type (EIP-1559 or legacy)
	var gasCost *big.Int
	var tx *types.Transaction

	if head.BaseFee != nil {
		// EIP-1559 network
		gasTipCap, err := client.SuggestGasTipCap(ctx)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to suggest gas tip cap: %v", err)), nil
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
			return mcp.NewToolResultError(fmt.Sprintf(
				"insufficient balance: have %s, need %s (including max gas cost)",
				balance.String(), totalNeeded.String())), nil
		}

		// Get nonce
		nonce, err := client.PendingNonceAt(ctx, walletAddress)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get nonce: %v", err)), nil
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
			return mcp.NewToolResultError(fmt.Sprintf("failed to suggest gas price: %v", err)), nil
		}

		// Calculate gas cost
		gasCost = new(big.Int).Mul(gasPrice, big.NewInt(int64(gasLimit)))

		// Check if we have enough funds
		totalNeeded := new(big.Int).Add(amount, gasCost)
		if balance.Cmp(totalNeeded) < 0 {
			return mcp.NewToolResultError(fmt.Sprintf(
				"insufficient balance: have %s, need %s (including gas cost)",
				balance.String(), totalNeeded.String())), nil
		}

		// Get nonce
		nonce, err := client.PendingNonceAt(ctx, walletAddress)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get nonce: %v", err)), nil
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
	signedTx, err := types.SignTx(tx, types.LatestSignerForChainID(chainID), s.privateKey)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to sign transaction: %v", err)), nil
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

		return mcp.NewToolResultError(fmt.Sprintf("transfer would fail: %v. Revert reason: %s", err, revertReason)), nil
	}

	// Send the transaction
	err = client.SendTransaction(ctx, signedTx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to send transaction: %v", err)), nil
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
		return mcp.NewToolResultError(fmt.Sprintf("error serializing result: %v", err)), nil
	}

	return mcp.NewToolResultText(string(jsonResponse)), nil
}
