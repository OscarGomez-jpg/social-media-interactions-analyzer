package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

const defaultTimeout = 10 * time.Second

// MCPClient is a JSON-RPC 2.0 HTTP client for FastMCP microservices.
type MCPClient struct {
	metricsURL     string
	propagationURL string
	summaryURL     string
	httpClient     *http.Client
	cache          map[string]any
	mu             sync.RWMutex
}

// MCPRequest represents a JSON-RPC 2.0 request payload.
type MCPRequest struct {
	JsonRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
	ID      int             `json:"id"`
}

// MCPResponse represents a JSON-RPC 2.0 response payload.
type MCPResponse struct {
	JsonRPC string          `json:"jsonrpc"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *MCPError       `json:"error,omitempty"`
	ID      int             `json:"id"`
}

// MCPError represents a JSON-RPC 2.0 error object.
type MCPError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *MCPError) Error() string {
	return fmt.Sprintf("rpc error %d: %s", e.Code, e.Message)
}

// NewMCPClient creates a new MCPClient with an explicit HTTP timeout.
func NewMCPClient() *MCPClient {
	return &MCPClient{
		metricsURL:     "http://localhost:8001/mcp",
		propagationURL: "http://localhost:8001/mcp",
		summaryURL:     "http://localhost:8001/mcp",
		httpClient: &http.Client{
			Timeout: defaultTimeout,
		},
		cache: make(map[string]any),
	}
}

// CallMetrics invokes a tool on the Metrics MCP service.
func (c *MCPClient) CallMetrics(method string, params map[string]any) (any, error) {
	return c.call(context.Background(), c.metricsURL, method, params)
}

// CallPropagation invokes a tool on the Propagation MCP service.
func (c *MCPClient) CallPropagation(method string, params map[string]any) (any, error) {
	return c.call(context.Background(), c.propagationURL, method, params)
}

// CallSummary invokes a tool on the Summary MCP service.
func (c *MCPClient) CallSummary(method string, params map[string]any) (any, error) {
	return c.call(context.Background(), c.summaryURL, method, params)
}

// call performs a JSON-RPC 2.0 HTTP POST request with context and caching.
func (c *MCPClient) call(ctx context.Context, baseURL, method string, params map[string]any) (any, error) {
	cacheKey := fmt.Sprintf("%s:%s:%v", baseURL, method, params)

	c.mu.RLock()
	if cached, ok := c.cache[cacheKey]; ok {
		c.mu.RUnlock()
		return cached, nil
	}
	c.mu.RUnlock()

	paramBytes, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("mcpclient: marshal params: %w", err)
	}

	reqPayload := MCPRequest{
		JsonRPC: "2.0",
		Method:  method,
		Params:  paramBytes,
		ID:      1,
	}

	body, err := json.Marshal(reqPayload)
	if err != nil {
		return nil, fmt.Errorf("mcpclient: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("mcpclient: create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("mcpclient: http call to %s: %w", baseURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("mcpclient: server returned status %d: %s", resp.StatusCode, string(body))
	}

	var mcpResp MCPResponse
	if err := json.NewDecoder(resp.Body).Decode(&mcpResp); err != nil {
		return nil, fmt.Errorf("mcpclient: decode response: %w", err)
	}

	if mcpResp.Error != nil {
		return nil, mcpResp.Error
	}

	var result any
	if len(mcpResp.Result) > 0 {
		if err := json.Unmarshal(mcpResp.Result, &result); err != nil {
			return nil, fmt.Errorf("mcpclient: unmarshal result: %w", err)
		}
	}

	c.mu.Lock()
	c.cache[cacheKey] = result
	c.mu.Unlock()

	return result, nil
}

// GetCached returns a cached value by key.
func (c *MCPClient) GetCached(key string) (any, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	val, ok := c.cache[key]
	return val, ok
}

// ClearCache removes all cached entries.
func (c *MCPClient) ClearCache() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache = make(map[string]any)
}

