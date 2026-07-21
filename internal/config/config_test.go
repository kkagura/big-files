package config

import (
	"os"
	"testing"
)

func TestStoreLoadsSingleConfigFile(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)
	content := []byte(`version: 1
provider: volcengine
base_url: https://ark.cn-beijing.volces.com/api/v3
model: endpoint-id
api_key: secret-key
request_timeout_seconds: 60
analysis:
  max_rounds: 8
  max_tool_calls: 20
  max_entries_per_call: 100
scan:
  max_entries: 1000
  follow_symlinks: false
  upload_file_content: false
`)
	if err := os.WriteFile(s.Paths().Config, content, 0600); err != nil {
		t.Fatal(err)
	}
	got, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.Model != "endpoint-id" || got.APIKey != "secret-key" {
		t.Fatalf("single-file config mismatch: %+v", got)
	}
}

func TestLoadUsesAPIKeyEnvironmentOverride(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)
	content := []byte("version: 1\nprovider: volcengine\nbase_url: https://example.com\nmodel: m\napi_key: file-key\n")
	if err := os.WriteFile(s.Paths().Config, content, 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("BIG_FILES_AI_API_KEY", "environment-key")
	got, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.APIKey != "environment-key" {
		t.Fatalf("environment override not applied: %q", got.APIKey)
	}
}

func TestValidateRejectsUnsafeScanOptions(t *testing.T) {
	c := Defaults()
	c.Model = "m"
	c.APIKey = "k"
	c.Scan.UploadFileContent = true
	if Validate(c) == nil {
		t.Fatal("expected validation error")
	}
}
