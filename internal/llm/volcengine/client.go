package volcengine

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"big-files/internal/llm"
)

const DefaultBaseURL = "https://ark.cn-beijing.volces.com/api/v3"

type Client struct {
	apiKey, baseURL string
	http            *http.Client
	retries         int
}

func New(apiKey, baseURL string, timeout time.Duration) (*Client, error) {
	if strings.TrimSpace(apiKey) == "" {
		return nil, fmt.Errorf("volcengine API key is required")
	}
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid base URL: %w", err)
	}
	if u.Scheme != "https" {
		return nil, fmt.Errorf("volcengine base URL must use HTTPS")
	}
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	return &Client{apiKey: apiKey, baseURL: strings.TrimRight(baseURL, "/"), http: &http.Client{Timeout: timeout}, retries: 2}, nil
}

type apiFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters"`
}
type apiTool struct {
	Type     string      `json:"type"`
	Function apiFunction `json:"function"`
}
type apiCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}
type apiCall struct {
	ID       string          `json:"id"`
	Type     string          `json:"type"`
	Function apiCallFunction `json:"function"`
}
type apiMessage struct {
	Role       string    `json:"role"`
	Content    string    `json:"content,omitempty"`
	ToolCallID string    `json:"tool_call_id,omitempty"`
	ToolCalls  []apiCall `json:"tool_calls,omitempty"`
}
type apiRequest struct {
	Model      string       `json:"model"`
	Messages   []apiMessage `json:"messages"`
	Tools      []apiTool    `json:"tools,omitempty"`
	ToolChoice string       `json:"tool_choice,omitempty"`
}
type apiResponse struct {
	Choices []struct {
		Message      apiMessage `json:"message"`
		FinishReason string     `json:"finish_reason"`
	} `json:"choices"`
	Usage llm.TokenUsage `json:"usage"`
	Error *struct {
		Message string `json:"message"`
		Code    string `json:"code"`
	} `json:"error,omitempty"`
}

func (c *Client) Complete(ctx context.Context, req llm.CompletionRequest) (llm.CompletionResponse, error) {
	body := apiRequest{Model: req.Model, ToolChoice: "auto"}
	for _, m := range req.Messages {
		am := apiMessage{Role: m.Role, Content: m.Content, ToolCallID: m.ToolCallID}
		for _, tc := range m.ToolCalls {
			am.ToolCalls = append(am.ToolCalls, apiCall{ID: tc.ID, Type: "function", Function: apiCallFunction{Name: tc.Name, Arguments: string(tc.Arguments)}})
		}
		body.Messages = append(body.Messages, am)
	}
	for _, tool := range req.Tools {
		if len(tool.Parameters) == 0 || !json.Valid(tool.Parameters) {
			return llm.CompletionResponse{}, fmt.Errorf("invalid JSON parameters schema for tool %q", tool.Name)
		}
		body.Tools = append(body.Tools, apiTool{Type: "function", Function: apiFunction{Name: tool.Name, Description: tool.Description, Parameters: tool.Parameters}})
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return llm.CompletionResponse{}, err
	}
	var last error
	for attempt := 0; attempt <= c.retries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return llm.CompletionResponse{}, ctx.Err()
			case <-time.After(time.Duration(attempt) * 200 * time.Millisecond):
			}
		}
		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(payload))
		if err != nil {
			return llm.CompletionResponse{}, err
		}
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
		httpReq.Header.Set("Content-Type", "application/json")
		resp, err := c.http.Do(httpReq)
		if err != nil {
			last = err
			continue
		}
		data, readErr := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
		resp.Body.Close()
		if readErr != nil {
			last = readErr
			continue
		}
		var decoded apiResponse
		if err := json.Unmarshal(data, &decoded); err != nil {
			return llm.CompletionResponse{}, fmt.Errorf("decode volcengine response (status %d): %w", resp.StatusCode, err)
		}
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			if len(decoded.Choices) == 0 {
				return llm.CompletionResponse{}, fmt.Errorf("volcengine returned no choices")
			}
			choice := decoded.Choices[0]
			out := llm.CompletionResponse{Message: llm.Message{Role: choice.Message.Role, Content: choice.Message.Content}, FinishReason: choice.FinishReason, Usage: decoded.Usage}
			for _, tc := range choice.Message.ToolCalls {
				call := llm.ToolCall{ID: tc.ID, Name: tc.Function.Name, Arguments: json.RawMessage(tc.Function.Arguments)}
				out.ToolCalls = append(out.ToolCalls, call)
				out.Message.ToolCalls = append(out.Message.ToolCalls, call)
			}
			return out, nil
		}
		msg := strings.TrimSpace(string(data))
		if decoded.Error != nil {
			msg = decoded.Error.Message
		}
		last = fmt.Errorf("volcengine API status %d: %s", resp.StatusCode, msg)
		if resp.StatusCode != http.StatusTooManyRequests && resp.StatusCode < 500 {
			break
		}
	}
	return llm.CompletionResponse{}, last
}
