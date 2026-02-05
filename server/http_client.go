package server

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"strconv"
	"sync"
	"time"
)

const (
	apiKeyHeader = "x-lifi-api-key"

	// Default rate limit: 200 requests per 2 hours (without API key)
	defaultRateLimit  = 200
	defaultRatePeriod = 2 * time.Hour

	// Retry configuration
	maxRetries       = 3
	baseRetryDelay   = 500 * time.Millisecond
	maxRetryDelay    = 30 * time.Second
	retryJitterRatio = 0.3
)

// HTTPClient wraps http.Client with rate limiting and retry logic.
// API key is passed per-request rather than stored in the client.
type HTTPClient struct {
	client  *http.Client
	logger  *slog.Logger
	limiter *rateLimiter
}

// rateLimiter implements a simple token bucket rate limiter
type rateLimiter struct {
	mu         sync.Mutex
	tokens     int
	maxTokens  int
	refillRate time.Duration
	lastRefill time.Time
}

func newRateLimiter(maxTokens int, period time.Duration) *rateLimiter {
	return &rateLimiter{
		tokens:     maxTokens,
		maxTokens:  maxTokens,
		refillRate: period / time.Duration(maxTokens),
		lastRefill: time.Now(),
	}
}

func (r *rateLimiter) acquire(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Refill tokens based on time elapsed
	now := time.Now()
	elapsed := now.Sub(r.lastRefill)
	tokensToAdd := int(elapsed / r.refillRate)
	if tokensToAdd > 0 {
		r.tokens = min(r.tokens+tokensToAdd, r.maxTokens)
		r.lastRefill = now
	}

	if r.tokens > 0 {
		r.tokens--
		return nil
	}

	// Calculate wait time until next token
	waitTime := r.refillRate - (now.Sub(r.lastRefill) % r.refillRate)

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(waitTime):
		r.tokens = 0 // We just used the token we waited for
		r.lastRefill = time.Now()
		return nil
	}
}

// NewHTTPClient creates a new HTTP client with logging and global rate limiting.
// The rate limiter uses default limits; per-request API keys don't change the global limit
// but are passed through to the LI.FI API which has its own per-key limits.
func NewHTTPClient(logger *slog.Logger) *HTTPClient {
	if logger == nil {
		logger = slog.Default()
	}

	return &HTTPClient{
		client:  &http.Client{Timeout: 30 * time.Second},
		logger:  logger,
		limiter: newRateLimiter(defaultRateLimit, defaultRatePeriod),
	}
}

// Get performs a GET request with context, rate limiting, retries, and per-request API key.
// Pass empty string for apiKey if no API key should be sent.
func (c *HTTPClient) Get(ctx context.Context, requestURL string, apiKey string) ([]byte, error) {
	return c.doWithRetry(ctx, http.MethodGet, requestURL, nil, apiKey)
}

// Post performs a POST request with context, rate limiting, retries, and per-request API key.
// Pass empty string for apiKey if no API key should be sent.
func (c *HTTPClient) Post(ctx context.Context, requestURL string, body []byte, apiKey string) ([]byte, error) {
	return c.doWithRetry(ctx, http.MethodPost, requestURL, body, apiKey)
}

func (c *HTTPClient) doWithRetry(ctx context.Context, method, requestURL string, body []byte, apiKey string) ([]byte, error) {
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		// Apply rate limiting
		if err := c.limiter.acquire(ctx); err != nil {
			return nil, fmt.Errorf("rate limiter: %w", err)
		}

		result, err, shouldRetry := c.doRequest(ctx, method, requestURL, body, apiKey)
		if err == nil {
			return result, nil
		}

		lastErr = err

		if !shouldRetry || attempt == maxRetries {
			break
		}

		// Calculate backoff with jitter
		delay := c.calculateBackoff(attempt)
		c.logger.Debug("Retrying request",
			"attempt", attempt+1,
			"max_retries", maxRetries,
			"delay", delay,
			"error", err,
		)

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay):
		}
	}

	return nil, lastErr
}

func (c *HTTPClient) doRequest(ctx context.Context, method, requestURL string, body []byte, apiKey string) ([]byte, error, bool) {
	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, requestURL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err), false
	}

	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/json")
	}
	if apiKey != "" {
		req.Header.Set(apiKeyHeader, apiKey)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		// Network errors are retryable
		return nil, err, true
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err, true
	}

	// Handle rate limiting (429)
	if resp.StatusCode == http.StatusTooManyRequests {
		retryAfter := c.parseRetryAfter(resp.Header.Get("Retry-After"))
		c.logger.Warn("Rate limited by API",
			"retry_after", retryAfter,
			"url", requestURL,
		)
		return nil, fmt.Errorf("rate limited (429): retry after %v", retryAfter), true
	}

	// Server errors are retryable
	if resp.StatusCode >= 500 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody)), true
	}

	// Client errors (except 429) are not retryable
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody)), false
	}

	return respBody, nil, false
}

func (c *HTTPClient) calculateBackoff(attempt int) time.Duration {
	// Exponential backoff: baseDelay * 2^attempt
	delay := baseRetryDelay * (1 << attempt)
	if delay > maxRetryDelay {
		delay = maxRetryDelay
	}

	// Add jitter (±30%)
	jitter := time.Duration(float64(delay) * retryJitterRatio * (2*rand.Float64() - 1))
	delay += jitter

	return delay
}

func (c *HTTPClient) parseRetryAfter(header string) time.Duration {
	if header == "" {
		return baseRetryDelay
	}

	// Try to parse as seconds
	if seconds, err := strconv.Atoi(header); err == nil {
		return time.Duration(seconds) * time.Second
	}

	// Try to parse as HTTP date (RFC 1123)
	if t, err := time.Parse(time.RFC1123, header); err == nil {
		return time.Until(t)
	}

	return baseRetryDelay
}
