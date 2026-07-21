package volcengine

import (
	"big-files/internal/llm"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCompleteMapsToolCallAndAuthorization(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Error("missing authorization")
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if body["model"] != "ep" {
			t.Error("model not mapped")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","tool_calls":[{"id":"c1","type":"function","function":{"name":"inspect_path","arguments":"{\"path\":\"cache\"}"}}]},"finish_reason":"tool_calls"}],"usage":{"total_tokens":12}}`))
	}))
	defer server.Close()
	c, err := New("test-key", server.URL, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	c.http = server.Client()
	got, err := c.Complete(context.Background(), llm.CompletionRequest{Model: "ep", Messages: []llm.Message{{Role: "user", Content: "x"}}, Tools: []llm.ToolDefinition{{Name: "inspect_path", Parameters: json.RawMessage(`{"type":"object"}`)}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.ToolCalls) != 1 || got.ToolCalls[0].Name != "inspect_path" || got.Usage.TotalTokens != 12 {
		t.Fatalf("bad response: %+v", got)
	}
}

func TestNewRejectsHTTP(t *testing.T) {
	if _, err := New("k", "http://example.com", time.Second); err == nil {
		t.Fatal("expected HTTPS validation")
	}
}
