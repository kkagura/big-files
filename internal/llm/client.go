package llm

import (
	"context"
	"encoding/json"
)

type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
}

type ToolCall struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

type ToolDefinition struct {
	Name, Description string
	Parameters        json.RawMessage
}
type CompletionRequest struct {
	Model          string
	Messages       []Message
	Tools          []ToolDefinition
	ResponseSchema json.RawMessage
}
type TokenUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}
type CompletionResponse struct {
	Message      Message
	ToolCalls    []ToolCall
	FinishReason string
	Usage        TokenUsage
}

type Client interface {
	Complete(context.Context, CompletionRequest) (CompletionResponse, error)
}
