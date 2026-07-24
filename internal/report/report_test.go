package report

import (
	"big-files/internal/model"
	"os"
	"strings"
	"testing"
	"time"
)

func TestWriteMarkdownIncludesDisclaimerAndDeduplicatedTotal(t *testing.T) {
	root := &model.FileEntry{Path: ".", Type: "directory", Size: 10, FileCount: 1}
	scan := &model.ScanResult{Root: "test-root", Complete: true, Entries: map[string]*model.FileEntry{".": root}}
	analysis := &model.AnalysisResult{Summary: "summary", Recommendations: []model.Recommendation{{Path: "cache", SizeBytes: 10, Risk: "review"}}}
	path := t.TempDir() + "/report.md"
	if err := WriteMarkdown(path, Document{GeneratedAt: time.Now(), Scan: scan, Analysis: analysis}); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if !strings.Contains(s, "候选项涉及空间") || !strings.Contains(s, "不会对分析目录执行删除") {
		t.Fatalf("missing safety text: %s", s)
	}
}

func TestRiskLabelsAreRenderedInChinese(t *testing.T) {
	tests := map[string]string{
		"likely_safe": "低风险候选",
		"review":      "需人工确认",
		"keep":        "建议保留",
		"unknown":     "信息不足",
	}
	for risk, expected := range tests {
		if got := riskLabel(risk); got != expected {
			t.Errorf("riskLabel(%q) = %q, want %q", risk, got, expected)
		}
	}
}
