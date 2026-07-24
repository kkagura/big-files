package report

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"big-files/internal/model"
)

type Document struct {
	GeneratedAt time.Time             `json:"generated_at"`
	Model       string                `json:"model,omitempty"`
	Scan        *model.ScanResult     `json:"scan"`
	Analysis    *model.AnalysisResult `json:"analysis,omitempty"`
}

func WriteJSON(path string, doc Document) error {
	b, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	return write(path, append(b, '\n'))
}
func WriteMarkdown(path string, doc Document) error {
	var b strings.Builder
	root := doc.Scan.RootEntry()
	b.WriteString("# 磁盘空间分析报告\n\n")
	fmt.Fprintf(&b, "生成时间：%s\n\n", doc.GeneratedAt.Format(time.RFC3339))
	b.WriteString("## 扫描概况\n\n")
	fmt.Fprintf(&b, "- 根目录：`%s`\n- 总大小：%s\n- 文件数：%d\n- 目录数：%d\n- 扫描完整：%v\n", doc.Scan.Root, formatSize(root.Size), root.FileCount, root.DirCount, doc.Scan.Complete)
	if doc.Analysis == nil {
		b.WriteString("\n## 第一层项目（按大小）\n\n")
		for _, e := range direct(doc.Scan) {
			fmt.Fprintf(&b, "- `%s`：%s（%s）\n", e.Path, formatSize(e.Size), e.Type)
		}
	} else {
		a := doc.Analysis
		b.WriteString("\n## AI 分析摘要\n\n")
		fmt.Fprintln(&b, a.Summary)
		section(&b, "优先检查的空间候选项", a.Recommendations)
		section(&b, "建议保留", a.Keep)
		section(&b, "无法判断 / 需要人工确认", a.Unknown)
		b.WriteString("\n## 分析覆盖\n\n")
		fmt.Fprintf(&b, "- 模型：`%s`\n- 轮次：%d\n- 工具调用：%d\n- 已查看条目：%d / %d\n", doc.Model, a.Coverage.Rounds, a.Coverage.ToolCalls, a.Coverage.EntriesInspected, a.Coverage.EntriesScanned)
		var candidateBytes int64
		for _, item := range a.Recommendations {
			candidateBytes += item.SizeBytes
		}
		fmt.Fprintf(&b, "- 候选项涉及空间（已去除父子重复）：%s\n", formatSize(candidateBytes))
		if len(a.Warnings) > 0 {
			b.WriteString("\n## 警告\n\n")
			for _, w := range a.Warnings {
				fmt.Fprintf(&b, "- %s\n", w)
			}
		}
	}
	b.WriteString("\n## 免责声明\n\n本报告只基于文件系统元数据给出候选项，不代表文件已被删除或确认可安全删除。软件不会对分析目录执行删除、移动或修改；请在本软件之外自行核实并决策。\n")
	return write(path, []byte(b.String()))
}
func section(b *strings.Builder, title string, items []model.Recommendation) {
	fmt.Fprintf(b, "\n## %s\n", title)
	if len(items) == 0 {
		fmt.Fprintln(b, "\n无。")
	}
	for _, r := range items {
		fmt.Fprintf(b, "\n### `%s`\n\n- 涉及空间：%s\n- 等级：%s，置信度：%.0f%%\n- 理由：%s\n", r.Path, formatSize(r.SizeBytes), riskLabel(r.Risk), r.Confidence*100, r.Reason)
		if len(r.Evidence) > 0 {
			fmt.Fprintf(b, "- 依据：%s\n", strings.Join(r.Evidence, "；"))
		}
		if len(r.VerifyBefore) > 0 {
			fmt.Fprintf(b, "- 操作前核实：%s\n", strings.Join(r.VerifyBefore, "；"))
		}
	}
}

func riskLabel(risk string) string {
	switch risk {
	case "likely_safe":
		return "低风险候选"
	case "review":
		return "需人工确认"
	case "keep":
		return "建议保留"
	case "unknown":
		return "信息不足"
	default:
		return "未识别等级"
	}
}
func direct(scan *model.ScanResult) []*model.FileEntry {
	var out []*model.FileEntry
	for p, e := range scan.Entries {
		if p != "." && filepath.ToSlash(filepath.Dir(p)) == "." {
			out = append(out, e)
		}
	}
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if out[j].Size > out[i].Size {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}
func write(path string, data []byte) error {
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	return os.WriteFile(path, data, 0644)
}
func formatSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	v := float64(size)
	for _, s := range []string{"KiB", "MiB", "GiB", "TiB", "PiB"} {
		v /= unit
		if v < unit {
			return fmt.Sprintf("%.1f %s", v, s)
		}
	}
	return fmt.Sprintf("%.1f EiB", v/unit)
}
