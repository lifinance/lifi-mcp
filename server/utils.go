package server

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
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
func (s *Server) getNativeTokenInfo(ctx context.Context, chainID *big.Int, apiKey string) (string, int, error) {
	// Initialize chains cache if not already done
	chainsCacheMu.RLock()
	initialized := chainsCacheInitialized
	chainsCacheMu.RUnlock()

	if !initialized {
		err := s.refreshChainsCache(ctx, apiKey)
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
	err := s.refreshChainsCache(ctx, apiKey)
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
func (s *Server) resolveRpcUrl(ctx context.Context, chain, rpcUrl, apiKey string) (string, error) {
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
		if err := s.refreshChainsCache(ctx, apiKey); err != nil {
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

// refreshChainsCache fetches the latest chain data from Li.Fi API
func (s *Server) refreshChainsCache(ctx context.Context, apiKey string) error {
	body, err := s.httpClient.Get(ctx, fmt.Sprintf("%s/v1/chains?chainTypes=SVM,EVM", BaseURL), apiKey)
	if err != nil {
		return fmt.Errorf("failed to fetch chains: %v", err)
	}

	var chainData ChainData
	err = json.Unmarshal(body, &chainData)
	if err != nil {
		return fmt.Errorf("failed to parse chain data: %v", err)
	}

	chainsCacheMu.Lock()
	chainsCache = chainData
	chainsCacheInitialized = true
	chainsCacheMu.Unlock()
	return nil
}
