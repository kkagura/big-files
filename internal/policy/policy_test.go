package policy

import (
	"big-files/internal/model"
	"testing"
)

func TestApplyUsesLocalSizeAndProtectsSensitiveTypes(t *testing.T) {
	scan := &model.ScanResult{Entries: map[string]*model.FileEntry{"data.db": {Path: "data.db", Name: "data.db", Type: "file", Extension: ".db", Size: 42}}}
	in := model.AnalysisResult{Recommendations: []model.Recommendation{{Path: "data.db", SizeBytes: 999, Risk: "likely_safe", Confidence: 1}, {Path: "../outside", Risk: "likely_safe"}}}
	out := Apply(in, scan)
	if len(out.Recommendations) != 1 {
		t.Fatalf("got %d", len(out.Recommendations))
	}
	r := out.Recommendations[0]
	if r.SizeBytes != 42 || r.Risk != "review" || r.Confidence > .5 {
		t.Fatalf("policy not applied: %+v", r)
	}
	if len(out.Warnings) == 0 {
		t.Fatal("missing warning")
	}
}

func TestApplyDeduplicatesParentChildSpace(t *testing.T) {
	scan := &model.ScanResult{Entries: map[string]*model.FileEntry{"cache": {Path: "cache", Name: "cache", Type: "directory", Size: 10}, "cache/a.tmp": {Path: "cache/a.tmp", Name: "a.tmp", Type: "file", Size: 10}}}
	in := model.AnalysisResult{Recommendations: []model.Recommendation{{Path: "cache", Risk: "review"}, {Path: "cache/a.tmp", Risk: "review"}}}
	out := Apply(in, scan)
	if len(out.Recommendations) != 1 || out.Recommendations[0].Path != "cache" {
		t.Fatalf("not deduplicated: %+v", out.Recommendations)
	}
}
