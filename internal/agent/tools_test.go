package agent

import (
	"big-files/internal/model"
	"encoding/json"
	"testing"
	"time"
)

func fixtureScan() *model.ScanResult {
	return &model.ScanResult{Complete: true, Entries: map[string]*model.FileEntry{".": {Path: ".", Type: "directory"}, "cache": {Path: "cache", Type: "directory", Size: 10}, "cache/a.log": {Path: "cache/a.log", Type: "file", Size: 10, Extension: ".log", ModifiedAt: time.Now().Add(-100 * 24 * time.Hour)}}}
}
func TestToolsRejectTraversalAndBoundResults(t *testing.T) {
	tools := NewTools(fixtureScan(), 1)
	if _, err := tools.Execute("inspect_path", json.RawMessage(`{"path":"../secret"}`)); err == nil {
		t.Fatal("expected traversal rejection")
	}
	v, err := tools.Execute("list_directory", json.RawMessage(`{"path":".","limit":99}`))
	if err != nil {
		t.Fatal(err)
	}
	if v.(map[string]any)["truncated"].(int) != 0 {
		t.Fatal("unexpected truncation")
	}
}
func TestFindCandidates(t *testing.T) {
	tools := NewTools(fixtureScan(), 10)
	v, err := tools.Execute("find_candidates", json.RawMessage(`{"under":"cache","older_than_days":90,"extensions":[".log"]}`))
	if err != nil {
		t.Fatal(err)
	}
	if v.(map[string]any)["total"].(int) != 1 {
		t.Fatal("candidate not found")
	}
}
