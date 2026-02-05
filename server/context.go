package server

import (
	"context"
	"net/http"
	"strings"
)

// contextKey is a custom type for context keys to avoid collisions
type contextKey string

const (
	// ctxKeyAPIKey is the context key for storing the LI.FI API key
	ctxKeyAPIKey contextKey = "lifi-api-key"
)

// ExtractAPIKeyFromRequest is the HTTPContextFunc for mcp-go's Streamable HTTP server.
// It extracts the LI.FI API key from the HTTP request headers and stores it in context.
// Supports both Authorization Bearer token and custom X-LiFi-Api-Key header.
func ExtractAPIKeyFromRequest(ctx context.Context, r *http.Request) context.Context {
	// Try Authorization header first (Bearer token)
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		apiKey := strings.TrimPrefix(auth, "Bearer ")
		if apiKey != "" {
			return context.WithValue(ctx, ctxKeyAPIKey, apiKey)
		}
	}

	// Fall back to custom header
	apiKey := r.Header.Get("X-LiFi-Api-Key")
	if apiKey != "" {
		return context.WithValue(ctx, ctxKeyAPIKey, apiKey)
	}

	// No API key provided - will use default rate limits
	return ctx
}

// APIKeyFromContext retrieves the LI.FI API key from the request context.
// Returns empty string if no API key was provided in the request.
func APIKeyFromContext(ctx context.Context) string {
	if v := ctx.Value(ctxKeyAPIKey); v != nil {
		if apiKey, ok := v.(string); ok {
			return apiKey
		}
	}
	return ""
}
