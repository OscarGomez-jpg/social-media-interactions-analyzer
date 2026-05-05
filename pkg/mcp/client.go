package mcp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
)

type MCPClient struct {
	metricsURL      string
	propagationURL string
	summaryURL    string
	cache         map[string]interface{}
	mu            sync.RWMutex
}

type MCPRequest struct {
	JsonRPC string          `json:"jsonrpc"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params,omitempty"`
	ID    int             `json:"id"`
}

type MCPResponse struct {
	JsonRPC string          `json:"jsonrpc"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *MCPError       `json:"error,omitempty"`
	ID    int             `json:"id"`
}

type MCPError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func NewMCPClient() *MCPClient {
	return &MCPClient{
		metricsURL:      "http://localhost:8001",
		propagationURL: "http://localhost:8002",
		summaryURL:    "http://localhost:8003",
		cache:        make(map[string]interface{}),
	}
}

func (c *MCPClient) CallMetrics(method string, params map[string]interface{}) (interface{}, error) {
	return c.call(c.metricsURL, method, params)
}

func (c *MCPClient) CallPropagation(method string, params map[string]interface{}) (interface{}, error) {
	return c.call(c.propagationURL, method, params)
}

func (c *MCPClient) CallSummary(method string, params map[string]interface{}) (interface{}, error) {
	return c.call(c.summaryURL, method, params)
}

func (c *MCPClient) call(baseURL, method string, params map[string]interface{}) (interface{}, error) {
	cacheKey := fmt.Sprintf("%s:%s:%v", baseURL, method, params)
	
	c.mu.RLock()
	if cached, ok := c.cache[cacheKey]; ok {
		c.mu.RUnlock()
		return cached, nil
	}
	c.mu.RUnlock()

	paramBytes, _ := json.Marshal(params)
	req := MCPRequest{
		JsonRPC: "2.0",
		Method:  method,
		Params:  paramBytes,
		ID:      1,
	}

	body, _ := json.Marshal(req)
	resp, err := http.Post(baseURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to call MCP: %w", err)
	}
	defer resp.Body.Close()

	var mcpResp MCPResponse
	if err := json.NewDecoder(resp.Body).Decode(&mcpResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if mcpResp.Error != nil {
		return nil, fmt.Errorf("MCP error: %s", mcpResp.Error.Message)
	}

	var result interface{}
	if len(mcpResp.Result) > 0 {
		json.Unmarshal(mcpResp.Result, &result)
	}

	c.mu.Lock()
	c.cache[cacheKey] = result
	c.mu.Unlock()

	return result, nil
}

func (c *MCPClient) GetCached(key string) (interface{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	val, ok := c.cache[key]
	return val, ok
}

func (c *MCPClient) ClearCache() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache = make(map[string]interface{})
}