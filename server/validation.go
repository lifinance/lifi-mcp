package server

import (
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/common"
)

const (
	// ZeroAddress is the Ethereum zero/burn address
	ZeroAddress = "0x0000000000000000000000000000000000000000"

	// MaxAmountDigits is the maximum number of digits allowed in an amount
	// This prevents overflow attacks with extremely large numbers
	MaxAmountDigits = 78 // uint256 max is ~78 digits
)

// ValidationError represents a validation failure
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// ValidateAddress validates an Ethereum address format
func ValidateAddress(field, address string) error {
	if address == "" {
		return &ValidationError{Field: field, Message: "address is required"}
	}

	if !common.IsHexAddress(address) {
		return &ValidationError{Field: field, Message: fmt.Sprintf("invalid address format: %s", address)}
	}

	return nil
}

// ValidateRecipientAddress validates a recipient address - must be valid AND not the zero address
func ValidateRecipientAddress(field, address string) error {
	if err := ValidateAddress(field, address); err != nil {
		return err
	}

	// Prevent sending to zero/burn address
	if strings.EqualFold(address, ZeroAddress) {
		return &ValidationError{
			Field:   field,
			Message: "cannot send to zero address (burn address) - this would permanently destroy funds",
		}
	}

	return nil
}

// ValidateChainID validates a chain ID string is a valid positive integer
func ValidateChainID(field, chainID string) error {
	if chainID == "" {
		return &ValidationError{Field: field, Message: "chain ID is required"}
	}

	// Try to parse as integer
	id, err := strconv.ParseInt(chainID, 10, 64)
	if err != nil {
		return &ValidationError{Field: field, Message: fmt.Sprintf("invalid chain ID format (must be numeric): %s", chainID)}
	}

	if id <= 0 {
		return &ValidationError{Field: field, Message: "chain ID must be a positive integer"}
	}

	return nil
}

// ValidateAmount validates a token amount string
func ValidateAmount(field, amount string) error {
	if amount == "" {
		return &ValidationError{Field: field, Message: "amount is required"}
	}

	// Check for excessive length (overflow protection)
	if len(amount) > MaxAmountDigits {
		return &ValidationError{Field: field, Message: "amount exceeds maximum allowed digits"}
	}

	// Parse the amount
	amountInt := new(big.Int)
	_, ok := amountInt.SetString(amount, 10)
	if !ok {
		return &ValidationError{Field: field, Message: fmt.Sprintf("invalid amount format: %s", amount)}
	}

	// Check for negative amounts
	if amountInt.Sign() < 0 {
		return &ValidationError{Field: field, Message: "amount cannot be negative"}
	}

	// Check for zero amount (optional, may be valid in some contexts)
	if amountInt.Sign() == 0 {
		return &ValidationError{Field: field, Message: "amount cannot be zero"}
	}

	return nil
}

// ValidateAmountAllowZero validates a token amount string, allowing zero
func ValidateAmountAllowZero(field, amount string) error {
	if amount == "" {
		return &ValidationError{Field: field, Message: "amount is required"}
	}

	// Check for excessive length (overflow protection)
	if len(amount) > MaxAmountDigits {
		return &ValidationError{Field: field, Message: "amount exceeds maximum allowed digits"}
	}

	// Parse the amount
	amountInt := new(big.Int)
	_, ok := amountInt.SetString(amount, 10)
	if !ok {
		return &ValidationError{Field: field, Message: fmt.Sprintf("invalid amount format: %s", amount)}
	}

	// Check for negative amounts
	if amountInt.Sign() < 0 {
		return &ValidationError{Field: field, Message: "amount cannot be negative"}
	}

	return nil
}

// ValidateSlippage validates a slippage value (optional field)
func ValidateSlippage(slippage string) error {
	if slippage == "" {
		return nil // Optional field
	}

	val, err := strconv.ParseFloat(slippage, 64)
	if err != nil {
		return &ValidationError{Field: "slippage", Message: fmt.Sprintf("invalid slippage format: %s", slippage)}
	}

	if val < 0 {
		return &ValidationError{Field: "slippage", Message: "slippage cannot be negative"}
	}

	if val > 1 {
		return &ValidationError{Field: "slippage", Message: "slippage cannot exceed 1 (100%)"}
	}

	return nil
}

// ValidateTokenAddress validates a token address, allowing zero address for native tokens
func ValidateTokenAddress(field, address string) error {
	if address == "" {
		return &ValidationError{Field: field, Message: "token address is required"}
	}

	if !common.IsHexAddress(address) {
		return &ValidationError{Field: field, Message: fmt.Sprintf("invalid token address format: %s", address)}
	}

	// Zero address is valid for native tokens
	return nil
}
