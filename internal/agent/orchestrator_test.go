package agent

import (
	"big-files/internal/llm"
	"big-files/internal/llm/mock"
	"context"
	"encoding/json"
	"testing"
)

func TestToolDefinitionsContainValidJSONSchemas(t *testing.T) {
	for _, definition := range toolDefinitions() {
		if len(definition.Parameters) == 0 || !json.Valid(definition.Parameters) {
			t.Errorf("tool %q has invalid parameters schema: %s", definition.Name, definition.Parameters)
		}
	}
}

func TestOrchestratorExecutesToolAndFinishes(t *testing.T) {
	client := &mock.Client{Responses: []llm.CompletionResponse{
		{Message: llm.Message{Role: "assistant", ToolCalls: []llm.ToolCall{{ID: "1", Name: "inspect_path", Arguments: json.RawMessage(`{"path":"cache"}`)}}}, ToolCalls: []llm.ToolCall{{ID: "1", Name: "inspect_path", Arguments: json.RawMessage(`{"path":"cache"}`)}}},
		{Message: llm.Message{Role: "assistant", ToolCalls: []llm.ToolCall{{ID: "2", Name: "finish_analysis", Arguments: json.RawMessage(`{"summary":"done","recommendations":[{"path":"cache","category":"cache","size_bytes":1,"risk":"review","confidence":0.8,"reason":"large","evidence":["metadata"],"verify_before_delete":["close app"]}],"keep":[],"unknown":[],"warnings":[]}`)}}}, ToolCalls: []llm.ToolCall{{ID: "2", Name: "finish_analysis", Arguments: json.RawMessage(`{"summary":"done","recommendations":[{"path":"cache","category":"cache","size_bytes":1,"risk":"review","confidence":0.8,"reason":"large","evidence":["metadata"],"verify_before_delete":["close app"]}],"keep":[],"unknown":[],"warnings":[]}`)}}},
	}}
	var progress []ProgressEvent
	o := Orchestrator{Client: client, Tools: NewTools(fixtureScan(), 10), Options: Options{Model: "test", MaxRounds: 3, MaxToolCalls: 3, MaxEntriesPerCall: 10, Progress: func(event ProgressEvent) {
		progress = append(progress, event)
	}}}
	got, err := o.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got.Summary != "done" || got.Recommendations[0].SizeBytes != 10 || got.Coverage.ToolCalls != 1 {
		t.Fatalf("unexpected result: %+v", got)
	}
	wantKinds := []string{"model_request", "model_response", "tool_call", "model_request", "model_response", "finished"}
	if len(progress) != len(wantKinds) {
		t.Fatalf("unexpected progress events: %+v", progress)
	}
	for i, kind := range wantKinds {
		if progress[i].Kind != kind {
			t.Fatalf("progress event %d: want %q, got %+v", i, kind, progress[i])
		}
	}
}
