package mock

import (
	"big-files/internal/llm"
	"context"
	"fmt"
	"sync"
)

type Client struct {
	mu        sync.Mutex
	Responses []llm.CompletionResponse
	Requests  []llm.CompletionRequest
	Err       error
}

func (c *Client) Complete(_ context.Context, r llm.CompletionRequest) (llm.CompletionResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Requests = append(c.Requests, r)
	if c.Err != nil {
		return llm.CompletionResponse{}, c.Err
	}
	if len(c.Responses) == 0 {
		return llm.CompletionResponse{}, fmt.Errorf("mock responses exhausted")
	}
	out := c.Responses[0]
	c.Responses = c.Responses[1:]
	return out, nil
}
