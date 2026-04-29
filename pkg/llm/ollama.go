package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// OllamaClient handles interactions with Ollama API
type OllamaClient struct {
	baseURL string
	model   string
	client  *http.Client
}

type ToolCall struct {
	Name string                 `json:"name"`
	Args map[string]interface{} `json:"args"`
}

// NewOllamaClient creates a new Ollama client
func NewOllamaClient(baseURL, model string) *OllamaClient {
	return &OllamaClient{
		baseURL: baseURL,
		model:   model,
		client:  &http.Client{},
	}
}

// ollamaRequest represents a request to Ollama
type ollamaRequest struct {
	Model   string                 `json:"model"`
	Prompt  string                 `json:"prompt"`
	Stream  bool                   `json:"stream"`
	Options map[string]interface{} `json:"options,omitempty"`
}

// ollamaResponse represents a response from Ollama
type ollamaResponse struct {
	Response string `json:"response"`
}

// GetToolCalls determines which tools to call based on user query
func (oc *OllamaClient) GetToolCalls(ctx context.Context, userQuery string) ([]ToolCall, error) {
	prompt := fmt.Sprintf(`You are a social media analysis assistant. Based on the user query, determine which tools to call.

Available tools:
1. get_conversation_summary - Generates an executive summary of social media conversations. Parameters: num_posts (int, 1-50)
2. get_social_metrics - Analyzes social media engagement metrics. Parameters: metric_type (string, "top_engagement" or "user_activity")
3. analyze_propagation - Analyzes how a post propagates. Parameters: post_id (int)

User query: %s

Respond ONLY with valid JSON array of tool calls in this format:
[{"name": "tool_name", "args": {"param": value}}]

If no tools are needed, respond with: []`, userQuery)

	resp, err := oc.callOllama(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("failed to get response from Ollama: %w", err)
	}

	// Parse tool calls from response
	var toolCalls []ToolCall

	// Try to extract JSON array from response
	startIdx := strings.Index(resp, "[")
	endIdx := strings.LastIndex(resp, "]")

	if startIdx != -1 && endIdx != -1 && endIdx > startIdx {
		jsonStr := resp[startIdx : endIdx+1]

		var parsedCalls []ToolCall
		if err := json.Unmarshal([]byte(jsonStr), &parsedCalls); err == nil {
			toolCalls = parsedCalls
		}
	}

	return toolCalls, nil
}

// GenerateFinalResponse generates a natural language response
func (oc *OllamaClient) GenerateFinalResponse(ctx context.Context, userQuery string, toolResults map[string]interface{}) (string, error) {
	resultsJSON, _ := json.MarshalIndent(toolResults, "", "  ")

	prompt := fmt.Sprintf(`El usuario preguntó: "%s"

Basándote en los siguientes resultados de análisis:
%s

Proporciona una respuesta natural y útil en español sobre el análisis de redes sociales.`, userQuery, string(resultsJSON))

	response, err := oc.callOllama(ctx, prompt)
	if err != nil {
		return "", fmt.Errorf("failed to generate final response: %w", err)
	}

	return response, nil
}

// callOllama makes a request to the Ollama API
func (oc *OllamaClient) callOllama(ctx context.Context, prompt string) (string, error) {
	reqBody := ollamaRequest{
		Model:   oc.model,
		Prompt:  prompt,
		Stream:  false,
		Options: map[string]any{"temperature": 0.0, "top_p": 0.9, "thinking": false},
	}

	bodyJSON, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", oc.baseURL+"/api/generate", bytes.NewReader(bodyJSON))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := oc.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to call Ollama: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("ollama returned status %d: %s", resp.StatusCode, string(body))
	}

	var ollamaResp ollamaResponse
	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	return ollamaResp.Response, nil
}
