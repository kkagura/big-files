package policy

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"big-files/internal/model"
)

var allowed = map[string]bool{"likely_safe": true, "review": true, "keep": true, "unknown": true}
var protectedExt = map[string]bool{".db": true, ".sqlite": true, ".sql": true, ".pem": true, ".key": true, ".pfx": true, ".env": true, ".doc": true, ".docx": true, ".xls": true, ".xlsx": true, ".jpg": true, ".jpeg": true, ".png": true, ".mp4": true}

func Apply(in model.AnalysisResult, scan *model.ScanResult) model.AnalysisResult {
	out := in
	out.Recommendations = validateList(in.Recommendations, scan, &out.Warnings)
	out.Keep = validateList(in.Keep, scan, &out.Warnings)
	out.Unknown = validateList(in.Unknown, scan, &out.Warnings)
	out.Recommendations = deduplicate(out.Recommendations)
	return out
}

func validateList(items []model.Recommendation, scan *model.ScanResult, warnings *[]string) []model.Recommendation {
	out := make([]model.Recommendation, 0, len(items))
	for _, r := range items {
		p := filepath.ToSlash(filepath.Clean(filepath.FromSlash(r.Path)))
		if filepath.IsAbs(filepath.FromSlash(r.Path)) || p == ".." || strings.HasPrefix(p, "../") {
			*warnings = append(*warnings, fmt.Sprintf("ignored out-of-root recommendation %q", r.Path))
			continue
		}
		e := scan.Entries[p]
		if e == nil {
			*warnings = append(*warnings, fmt.Sprintf("ignored unknown path %q", r.Path))
			continue
		}
		r.Path = p
		r.SizeBytes = e.Size
		if !allowed[r.Risk] {
			r.Risk = "unknown"
			*warnings = append(*warnings, fmt.Sprintf("corrected invalid risk for %q", p))
		}
		if r.Confidence < 0 {
			r.Confidence = 0
		}
		if r.Confidence > 1 {
			r.Confidence = 1
		}
		lower := strings.ToLower(e.Name)
		protected := protectedExt[e.Extension] || strings.Contains(lower, "credential") || strings.Contains(lower, "secret") || strings.Contains(lower, "backup") || e.Error != ""
		if protected && r.Risk == "likely_safe" {
			r.Risk = "review"
			if r.Confidence > .5 {
				r.Confidence = .5
			}
			r.Evidence = append(r.Evidence, "本地保护规则识别到高风险类型或名称")
		}
		out = append(out, r)
	}
	return out
}

func deduplicate(items []model.Recommendation) []model.Recommendation {
	sort.SliceStable(items, func(i, j int) bool { return strings.Count(items[i].Path, "/") < strings.Count(items[j].Path, "/") })
	out := make([]model.Recommendation, 0, len(items))
	for _, item := range items {
		overlap := false
		for _, p := range out {
			if strings.HasPrefix(item.Path, p.Path+"/") {
				overlap = true
				break
			}
		}
		if !overlap {
			out = append(out, item)
		}
	}
	return out
}
