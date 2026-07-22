package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"big-files/internal/agent"
	"big-files/internal/config"
	"big-files/internal/llm"
)

func TestAnalysisProgressRendererUsesStaticOutputWhenNotInteractive(t *testing.T) {
	var output bytes.Buffer
	renderer := newAnalysisProgressRenderer(&output, false)
	renderer.Handle(agent.ProgressEvent{Kind: "model_request", Round: 1, MaxRounds: 8})
	renderer.Handle(agent.ProgressEvent{Kind: "model_response", Round: 1, MaxRounds: 8})
	renderer.Close()
	text := output.String()
	if !strings.Contains(text, "等待模型响应") || !strings.Contains(text, "已收到模型响应") || strings.Contains(text, "\r") {
		t.Fatalf("unexpected non-interactive loading output: %q", text)
	}
}

func TestStartLoadingAnimatesAndStopsIdempotently(t *testing.T) {
	var output bytes.Buffer
	stop := startLoading(&output, true, "waiting")
	time.Sleep(150 * time.Millisecond)
	stop()
	stop()
	text := output.String()
	if !strings.Contains(text, "waiting") || !strings.ContainsAny(text, "|/-\\") || !strings.Contains(text, "\r") {
		t.Fatalf("loading animation was not rendered: %q", text)
	}
}

func TestEnsureConfigExitsWithManualConfigurationGuide(t *testing.T) {
	dir := t.TempDir()
	store := config.NewStore(dir)
	_, err := ensureConfig(store)
	if err == nil {
		t.Fatal("expected missing configuration error")
	}
	var notice *configurationRequiredError
	if !errors.As(err, &notice) {
		t.Fatalf("missing configuration must be a normal-exit notice, got %T", err)
	}
	message := err.Error()
	for _, expected := range []string{
		"程序已退出",
		store.Paths().Directory,
		store.Paths().Config,
		"config.yaml 示例",
		"provider: \"volcengine\"",
		"api_key: \"请填写 API Key\"",
	} {
		if !strings.Contains(message, expected) {
			t.Fatalf("missing %q in guidance:\n%s", expected, message)
		}
	}
	entries, readErr := os.ReadDir(dir)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if len(entries) != 0 {
		t.Fatalf("program must not create configuration files: %+v", entries)
	}
}

func TestEnsureConfigLoadsExistingFiles(t *testing.T) {
	store := config.NewStore(t.TempDir())
	content := []byte("version: 1\nprovider: volcengine\nbase_url: https://example.com\nmodel: endpoint-id\napi_key: secret\n")
	if err := os.WriteFile(filepath.Clean(store.Paths().Config), content, 0600); err != nil {
		t.Fatal(err)
	}
	gotConfig, err := ensureConfig(store)
	if err != nil {
		t.Fatal(err)
	}
	if gotConfig.Model != "endpoint-id" || gotConfig.APIKey != "secret" {
		t.Fatalf("unexpected loaded configuration: %+v", gotConfig)
	}
}

func TestProviderRegistryContainsOnlyImplementedProviders(t *testing.T) {
	providers := llm.Providers()
	if len(providers) != 1 || providers[0].ID != "volcengine" || !providers[0].Capabilities.ToolCalling {
		t.Fatalf("unexpected provider registry: %+v", providers)
	}
}
