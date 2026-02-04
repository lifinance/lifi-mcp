package server

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

const apiKeyHeader = "x-lifi-api-key"

// HTTPClient wraps http.Client with automatic API key header injection
type HTTPClient struct {
	client *http.Client
	apiKey string
}

// NewHTTPClient creates a new HTTP client with optional API key
func NewHTTPClient(apiKey string) *HTTPClient {
	return &HTTPClient{
		client: &http.Client{Timeout: 30 * time.Second},
		apiKey: apiKey,
	}
}

// Get performs a GET request with context and automatic API key header
func (c *HTTPClient) Get(ctx context.Context, requestURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	if c.apiKey != "" {
		req.Header.Set(apiKeyHeader, c.apiKey)
	}
	return c.do(req)
}

// Post performs a POST request with context and automatic API key header
func (c *HTTPClient) Post(ctx context.Context, requestURL string, body []byte) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set(apiKeyHeader, c.apiKey)
	}
	return c.do(req)
}

func (c *HTTPClient) do(req *http.Request) ([]byte, error) {
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}
