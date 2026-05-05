package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/joho/godotenv"
	"social-media-analyzer/pkg/data"
	"social-media-analyzer/pkg/llm"
	"social-media-analyzer/pkg/models"
	"social-media-analyzer/pkg/tools"
)

// Agent orchestrates tool execution and LLM integration
type Agent struct {
	tools        *tools.Tools
	ollamaClient *llm.OllamaClient
	backend      string // "claude" or "ollama"
}

// NewAgent creates a new agent
func NewAgent(backend string) *Agent {

	ollamaClient := llm.NewOllamaClient(
		"http://localhost:11434",
		"gemma4:e2b",
	)

	return &Agent{
		tools:        tools.NewTools(),
		ollamaClient: ollamaClient,
		backend:      backend,
	}
}

// ProcessQuery processes a user query with concurrent tool execution
func (a *Agent) ProcessQuery(query string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	log.Printf("🔍 Analyzing query: %s\n", query)

	// Get tool calls from LLM
	var toolCalls []llm.ToolCall
	var err error

	toolCalls, err = a.ollamaClient.GetToolCalls(ctx, query)

	if err != nil {
		return "", fmt.Errorf("failed to get tool calls: %w", err)
	}

	if len(toolCalls) == 0 {
		return "No se requieren herramientas para esta consulta.", nil
	}

	// Execute tools concurrently with goroutines
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

	switch toolCall.Name {
	case "get_conversation_summary":
		numPosts := 10
		if np, ok := toolCall.Args["num_posts"].(float64); ok {
			numPosts = int(np)
		}
		result.Result = a.tools.GetConversationSummary(numPosts)
		result.Status = "success"

	case "get_social_metrics":
		metricType := "top_engagement"
		if mt, ok := toolCall.Args["metric_type"].(string); ok {
			metricType = mt
		}
		result.Result = a.tools.GetSocialMetrics(metricType)
		result.Status = "success"

	case "analyze_propagation":
		postID := int64(1)
		if pid, ok := toolCall.Args["post_id"].(float64); ok {
			postID = int64(pid)
		}
		result.Result = a.tools.AnalyzePropagation(postID)
		result.Status = "success"

	default:
		result.Status = "error"
		result.Error = fmt.Sprintf("Unknown tool: %s", toolCall.Name)
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

	// Create agent
	agent := NewAgent(backend)

	// Start interactive loop
	agent.InteractiveLoop()
}
