package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

const defaultTimeout = 120 * time.Second

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

// mcpContentResult is the tools/call envelope FastMCP wraps results in.
type mcpContentResult struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	IsError bool `json:"isError"`
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

	// MCP protocol requires tools/call with name and arguments as a JSON object.
	mcpParams := map[string]any{
		"name":      method,
		"arguments": json.RawMessage(paramBytes),
	}

	callPayload, err := json.Marshal(mcpParams)
	if err != nil {
		return nil, fmt.Errorf("mcpclient: marshal call params: %w", err)
	}

	reqPayload := MCPRequest{
		JsonRPC: "2.0",
		Method:  "tools/call",
		Params:  callPayload,
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
	// FastMCP requires text/event-stream; include application/json so it can
	// fall back to plain JSON if supported.
	httpReq.Header.Set("Accept", "application/json, text/event-stream")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("mcpclient: http call to %s: %w", baseURL, err)
	}
	defer resp.Body.Close()

	rawBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("mcpclient: read body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("mcpclient: server returned status %d: %s", resp.StatusCode, string(rawBody))
	}

	mcpResp, err := decodeMCPResponse(rawBody)
	if err != nil {
		return nil, fmt.Errorf("mcpclient: decode response: %w", err)
	}

	if mcpResp.Error != nil {
		return nil, mcpResp.Error
	}

	result, err := unwrapMCPResult(mcpResp.Result)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	c.cache[cacheKey] = result
	c.mu.Unlock()

	return result, nil
}

// decodeMCPResponse parses a raw HTTP body as either plain JSON-RPC or SSE.
func decodeMCPResponse(raw []byte) (*MCPResponse, error) {
	// Try plain JSON first.
	var resp MCPResponse
	if err := json.Unmarshal(raw, &resp); err == nil {
		return &resp, nil
	}

	// Fall back to SSE: look for lines starting with "data: ".
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		jsonData := strings.TrimPrefix(line, "data: ")
		if jsonData == "[DONE]" {
			continue
		}
		var sseResp MCPResponse
		if err := json.Unmarshal([]byte(jsonData), &sseResp); err == nil {
			return &sseResp, nil
		}
	}

	return nil, fmt.Errorf("could not parse body as JSON or SSE: %q", truncate(string(raw), 200))
}

// unwrapMCPResult extracts the actual tool payload from the MCP tools/call
// envelope: {"content":[{"type":"text","text":"..."}],"isError":false}.
func unwrapMCPResult(raw json.RawMessage) (any, error) {
	if len(raw) == 0 {
		return nil, nil
	}

	var envelope mcpContentResult
	if err := json.Unmarshal(raw, &envelope); err == nil && len(envelope.Content) > 0 {
		text := envelope.Content[0].Text
		// The text field itself is a JSON-encoded value — parse it.
		var inner any
		if err := json.Unmarshal([]byte(text), &inner); err == nil {
			return inner, nil
		}
		return text, nil
	}

	// Fallback: return raw result as-is.
	var result any
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("mcpclient: unmarshal result: %w", err)
	}
	return result, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
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
