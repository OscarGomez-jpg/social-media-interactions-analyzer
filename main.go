package main

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"social-media-analyzer/pkg/data"
	"social-media-analyzer/pkg/llm"
	"social-media-analyzer/pkg/mcp"
	"social-media-analyzer/pkg/models"
	"social-media-analyzer/pkg/tools"

	"github.com/joho/godotenv"
)

// Agent orchestrates tool execution and LLM integration
type Agent struct {
	tools        *tools.Tools
	ollamaClient *llm.OllamaClient
	mcpClient    *mcp.MCPClient
	queryCache   map[string]string
	cacheMu      sync.RWMutex
	backend      string // "claude" or "ollama"
	useMCP       bool
}

// NewAgent creates a new agent
func NewAgent(backend string, useMCP bool) *Agent {
	ollamaClient := llm.NewOllamaClient(
		"http://localhost:11434",
		"gemma4:e2b",
	)

	var mcpClient *mcp.MCPClient
	if useMCP {
		mcpClient = mcp.NewMCPClient()
	}

	return &Agent{
		tools:        tools.NewTools(),
		ollamaClient: ollamaClient,
		mcpClient:    mcpClient,
		queryCache:   make(map[string]string),
		backend:      backend,
		useMCP:       useMCP,
	}
}

func hashQuery(q string) string {
	h := sha256.New()
	h.Write([]byte(q))
	return hex.EncodeToString(h.Sum(nil))[:16]
}

func (a *Agent) ProcessQuery(query string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 240*time.Second)
	defer cancel()

	log.Printf("🔍 Analyzing query: %s\n", query)

	queryHash := hashQuery(query)
	a.cacheMu.RLock()
	if cached, ok := a.queryCache[queryHash]; ok {
		a.cacheMu.RUnlock()
		log.Printf("📦 Returning cached response")
		return cached, nil
	}
	a.cacheMu.RUnlock()

	// Get tool calls and reasoning from LLM
	var toolCalls []llm.ToolCall
	var err error

	// We'll call a new method that returns the raw response to extract Thought
	rawResp, err := a.ollamaClient.GetRawResponse(ctx, query)
	if err != nil {
		return "", fmt.Errorf("failed to get response from Ollama: %w", err)
	}

	// Print Thought if present
	if thoughtIdx := strings.Index(rawResp, "THOUGHT:"); thoughtIdx != -1 {
		endThought := strings.Index(rawResp[thoughtIdx:], "\n")
		if endThought == -1 {
			endThought = strings.Index(rawResp[thoughtIdx:], "[")
		}
		if endThought != -1 {
			fmt.Printf("\n🧠 %s\n", strings.TrimSpace(rawResp[thoughtIdx:thoughtIdx+endThought]))
		}
	}

	toolCalls, err = a.ollamaClient.ParseToolCalls(rawResp)
	if err != nil {
		return "", fmt.Errorf("failed to parse tool calls: %w", err)
	}

	if len(toolCalls) == 0 {
		return "No se requieren herramientas para esta consulta.", nil
	}

	// Execute tools (MCP or local)
	toolResults := a.executeToolsConcurrently(toolCalls)

	// Convert results to map for final response
	resultsMap := make(map[string]interface{})
	for _, result := range toolResults {
		if result.Status == "success" {
			resultsMap[result.ToolName] = result.Result
		}
	}

	// Generate final response
	finalResponse, err := a.ollamaClient.GenerateFinalResponse(ctx, query, resultsMap)
	if err != nil {
		return "", fmt.Errorf("failed to generate response: %w", err)
	}

	// Cache the response
	a.cacheMu.Lock()
	a.queryCache[queryHash] = finalResponse
	a.cacheMu.Unlock()

	// Print tool results
	fmt.Println("\n📊 Herramientas ejecutadas:")
	for _, result := range toolResults {
		fmt.Printf("  ✓ %s: %s\n", result.ToolName, result.Status)
	}

	return finalResponse, nil
}

// executeToolsConcurrently executes all tools in parallel
func (a *Agent) executeToolsConcurrently(toolCalls []llm.ToolCall) []models.ToolResult {
	results := make([]models.ToolResult, len(toolCalls))
	var wg sync.WaitGroup

	for i, toolCall := range toolCalls {
		wg.Add(1)
		go func(idx int, tc llm.ToolCall) {
			defer wg.Done()
			results[idx] = a.executeTool(tc)
		}(i, toolCall)
	}

	wg.Wait()
	return results
}

// executeTool executes a single tool
func (a *Agent) executeTool(toolCall llm.ToolCall) models.ToolResult {
	result := models.ToolResult{
		ToolName: toolCall.Name,
	}

	// If MCP is enabled, use MCP client
	if a.useMCP && a.mcpClient != nil {
		return a.executeMCPTool(toolCall)
	}

	// Local fallback for testing (mock data)
	switch toolCall.Name {
	case "get_summary":
		result.Result = a.tools.GetConversationSummary(10)
		result.Status = "success"

	case "get_metrics":
		result.Result = a.tools.GetSocialMetrics("top_engagement")
		result.Status = "success"

	case "analyze_propagation":
		result.Result = a.tools.AnalyzePropagation(1)
		result.Status = "success"

	default:
		result.Status = "error"
		result.Error = fmt.Sprintf("Unknown tool: %s", toolCall.Name)
	}

	return result
}

// executeMCPTool executes a tool via the unified MCP client
func (a *Agent) executeMCPTool(toolCall llm.ToolCall) models.ToolResult {
	result := models.ToolResult{
		ToolName: toolCall.Name,
	}

	params := make(map[string]interface{})
	for k, v := range toolCall.Args {
		params[k] = v
	}

	// All tools now point to the unified Metrics endpoint (port 8001)
	mcpResult, err := a.mcpClient.CallMetrics(toolCall.Name, params)

	if err != nil {
		result.Status = "error"
		result.Error = err.Error()
	} else {
		result.Result = mcpResult
		result.Status = "success"
	}

	return result
}

// InteractiveLoop starts the interactive query loop
func (a *Agent) InteractiveLoop() {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("\n" + strings.Repeat("=", 70))
	fmt.Println("🚀 SOCIAL LISTENING AGENT - Interactive Mode")
	fmt.Println(strings.Repeat("=", 70))
	fmt.Printf("Backend: %s\n\n", strings.ToUpper(a.backend))
	fmt.Println("Escribe tus preguntas sobre análisis de redes sociales.")
	fmt.Println("Escribe 'quit' o 'exit' para salir.")

	for {
		fmt.Print("You: ")
		input, _ := reader.ReadString('\n')
		query := strings.TrimSpace(input)

		if query == "" {
			continue
		}

		if strings.ToLower(query) == "quit" || strings.ToLower(query) == "exit" {
			fmt.Println("\nAgent: ¡Hasta luego! 👋")
			break
		}

		response, err := a.ProcessQuery(query)
		if err != nil {
			fmt.Printf("❌ Error: %v\n\n", err)
			continue
		}

		fmt.Printf("\nAgent: %s\n\n", response)
	}
}

func main() {
	// Load mock data
	store := data.GetStore()
	log.Printf("✓ Loaded %d posts from mock data\n", len(store.GetPosts()))

	// Determine LLM backend
	err := godotenv.Load()
	if err != nil {
		log.Printf("⚠️  No .env file found, using default settings\n")
	}

	backend := os.Getenv("LLM_BACKEND")

	if backend != "ollama" {
		log.Fatalf("Invalid LLM_BACKEND: %s (must be 'ollama')\n", backend)
	}

	useMCP := os.Getenv("USE_MCP") == "true"
	if useMCP {
		log.Printf("🔌 Using MCP services for tool execution\n")
	}

	// Create agent
	agent := NewAgent(backend, useMCP)

	// Start interactive loop
	agent.InteractiveLoop()
}
