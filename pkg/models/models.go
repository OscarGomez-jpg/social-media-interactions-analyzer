package models

// ToolResult represents the result of executing a tool
type ToolResult struct {
	ToolName string `json:"tool_name"`
	Status   string `json:"status"` // "success" or "error"
	Result   any    `json:"result,omitempty"`
	Error    string `json:"error,omitempty"`
}
