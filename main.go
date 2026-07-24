package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"big-files/internal/agent"
	"big-files/internal/config"
	"big-files/internal/llm"
	"big-files/internal/llm/volcengine"
	"big-files/internal/model"
	"big-files/internal/report"
	"big-files/internal/scanner"
)

var version = "dev"

func main() {
	if err := run(os.Args[1:]); err != nil {
		var notice *configurationRequiredError
		if errors.As(err, &notice) {
			fmt.Fprintln(os.Stderr, notice)
			return
		}
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

type configurationRequiredError struct{ message string }

func (e *configurationRequiredError) Error() string { return e.message }
func run(args []string) error {
	if len(args) == 0 {
		return runAnalyze(nil)
	}
	switch args[0] {
	case "scan":
		return runScan(args[1:])
	case "analyze":
		return runAnalyze(args[1:])
	case "config":
		return runConfig(args[1:])
	case "version", "--version":
		fmt.Println(version)
		return nil
	case "help", "--help", "-h":
		usage()
		return nil
	default:
		return fmt.Errorf("unknown command %q (use --help)", args[0])
	}
}
func usage() {
	fmt.Print(`big-files - 只读磁盘空间分析工具

Usage:
  big-files scan [--root DIR] [--report FILE] [--json-report FILE]
  big-files analyze [--root DIR] [--report FILE] [--json-report FILE]
  big-files config show|path
`)
}

func runScan(args []string) error {
	fs := flag.NewFlagSet("scan", flag.ContinueOnError)
	root := fs.String("root", ".", "scan root")
	md := fs.String("report", "scan-report.md", "Markdown report")
	js := fs.String("json-report", "scan-report.json", "JSON report")
	max := fs.Int("max-entries", 1000000, "maximum indexed entries")
	if err := fs.Parse(args); err != nil {
		return err
	}
	progressf("开始扫描目录：%s", absolutePath(*root))
	s, err := scanner.Scan(context.Background(), *root, scanner.Options{MaxEntries: *max, Progress: printScanProgress})
	if err != nil {
		return err
	}
	progressf("目录扫描完成，正在生成本地报告")
	doc := report.Document{GeneratedAt: time.Now(), Scan: s}
	progressf("写入 Markdown 报告：%s", *md)
	if err = report.WriteMarkdown(*md, doc); err != nil {
		return err
	}
	progressf("写入 JSON 报告：%s", *js)
	if err = report.WriteJSON(*js, doc); err != nil {
		return err
	}
	progressf("本地扫描任务完成")
	printSummary(s, *md, *js)
	return nil
}
func runAnalyze(args []string) error {
	fs := flag.NewFlagSet("analyze", flag.ContinueOnError)
	root := fs.String("root", ".", "scan root")
	md := fs.String("report", "ai-analysis-report.md", "Markdown report")
	js := fs.String("json-report", "ai-analysis-report.json", "JSON report")
	modelOverride := fs.String("model", "", "temporary model override")
	if err := fs.Parse(args); err != nil {
		return err
	}
	progressf("加载 AI 配置")
	store, err := config.UserStore()
	if err != nil {
		return err
	}
	cfg, err := ensureConfig(store)
	if err != nil {
		return err
	}
	if *modelOverride != "" {
		cfg.Model = *modelOverride
	}
	providerName := cfg.Provider
	if provider, ok := llm.Provider(cfg.Provider); ok {
		providerName = provider.DisplayName
	}
	progressf("配置加载完成：厂商=%s，模型=%s", providerName, cfg.Model)
	progressf("开始扫描目录：%s", absolutePath(*root))
	s, err := scanner.Scan(context.Background(), *root, scanner.Options{MaxEntries: cfg.Scan.MaxEntries, Progress: printScanProgress})
	if err != nil {
		return err
	}
	progressf("目录扫描完成：文件=%d，目录=%d，大小=%d bytes", s.RootEntry().FileCount, s.RootEntry().DirCount, s.RootEntry().Size)
	progressf("创建 %s 模型客户端", providerName)
	client, err := volcengine.New(cfg.APIKey, cfg.BaseURL, time.Duration(cfg.RequestTimeoutSeconds)*time.Second)
	if err != nil {
		return err
	}
	progressf("开始 AI 分析：最多 %d 轮、%d 次只读工具调用", cfg.Analysis.MaxRounds, cfg.Analysis.MaxToolCalls)
	progressRenderer := newAnalysisProgressRenderer(os.Stdout, isTerminal(os.Stdout))
	defer progressRenderer.Close()
	orch := agent.Orchestrator{Client: client, Tools: agent.NewTools(s, cfg.Analysis.MaxEntriesPerCall), Options: agent.Options{Model: cfg.Model, MaxRounds: cfg.Analysis.MaxRounds, MaxToolCalls: cfg.Analysis.MaxToolCalls, MaxEntriesPerCall: cfg.Analysis.MaxEntriesPerCall, Progress: progressRenderer.Handle}}
	result, analysisErr := orch.Run(context.Background())
	doc := report.Document{GeneratedAt: time.Now(), Model: cfg.Model, Scan: s, Analysis: &result}
	progressf("写入 Markdown 报告：%s", *md)
	if err = report.WriteMarkdown(*md, doc); err != nil {
		return err
	}
	progressf("写入 JSON 报告：%s", *js)
	if err = report.WriteJSON(*js, doc); err != nil {
		return err
	}
	progressf("分析任务完成")
	printSummary(s, *md, *js)
	if analysisErr != nil {
		return fmt.Errorf("AI analysis degraded; partial report written: %w", analysisErr)
	}
	return nil
}

func progressf(format string, args ...any) {
	writeProgress(os.Stdout, format, args...)
}

func writeProgress(output io.Writer, format string, args ...any) {
	fmt.Fprintf(output, "[%s] %s\n", time.Now().Format("15:04:05"), fmt.Sprintf(format, args...))
}

func absolutePath(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	return abs
}

func printScanProgress(progress scanner.Progress) {
	progressf("扫描进度：已索引 %d 项（文件 %d，目录 %d）", progress.Entries, progress.Files, progress.Dirs)
}

type analysisProgressRenderer struct {
	output            io.Writer
	animate           bool
	stop              func()
	startedAt         time.Time
	lastDecisionRound int
	lastDecision      string
}

func newAnalysisProgressRenderer(output io.Writer, animate bool) *analysisProgressRenderer {
	return &analysisProgressRenderer{output: output, animate: animate}
}

func (r *analysisProgressRenderer) Handle(event agent.ProgressEvent) {
	switch event.Kind {
	case "model_request":
		r.stopLoading()
		r.startedAt = time.Now()
		r.stop = startLoading(r.output, r.animate, fmt.Sprintf("AI 分析第 %d/%d 轮：等待模型响应", event.Round, event.MaxRounds))
	case "model_response":
		r.stopLoading()
		writeProgress(r.output, "AI 分析第 %d/%d 轮：已收到模型响应（耗时 %s）", event.Round, event.MaxRounds, time.Since(r.startedAt).Round(time.Millisecond))
	case "model_error":
		r.stopLoading()
		writeProgress(r.output, "AI 分析第 %d/%d 轮：模型请求失败（耗时 %s）", event.Round, event.MaxRounds, time.Since(r.startedAt).Round(time.Millisecond))
	case "tool_call":
		r.printDecision(event.Round, event.DecisionSummary)
		if reason := sanitizeAuditText(event.ToolReason); reason != "" {
			writeProgress(r.output, "工具调用理由：%s", reason)
		}
		writeProgress(r.output, "执行只读工具 %s（%d/%d）", event.ToolName, event.ToolCalls, event.MaxToolCalls)
	case "finished":
		r.stopLoading()
		if summary := sanitizeAuditText(event.DecisionSummary); summary != "" {
			writeProgress(r.output, "最终决策摘要：%s", summary)
		}
		writeProgress(r.output, "AI 已提交最终分析结果")
	}
}

func (r *analysisProgressRenderer) Close() { r.stopLoading() }

func (r *analysisProgressRenderer) printDecision(round int, value string) {
	decision := sanitizeAuditText(value)
	if decision == "" || (r.lastDecisionRound == round && r.lastDecision == decision) {
		return
	}
	writeProgress(r.output, "第 %d 轮决策摘要：%s", round, decision)
	r.lastDecisionRound = round
	r.lastDecision = decision
}

func (r *analysisProgressRenderer) stopLoading() {
	if r.stop != nil {
		r.stop()
		r.stop = nil
	}
}

func startLoading(output io.Writer, animate bool, message string) func() {
	if !animate {
		writeProgress(output, "%s", message)
		return func() {}
	}
	done := make(chan struct{})
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		frames := []byte{'|', '/', '-', '\\'}
		ticker := time.NewTicker(120 * time.Millisecond)
		defer ticker.Stop()
		frame := 0
		for {
			fmt.Fprintf(output, "\r[%s] %s %c", time.Now().Format("15:04:05"), message, frames[frame%len(frames)])
			select {
			case <-done:
				fmt.Fprintf(output, "\r%s\r", strings.Repeat(" ", len(message)+24))
				return
			case <-ticker.C:
				frame++
			}
		}
	}()
	var once sync.Once
	return func() {
		once.Do(func() {
			close(done)
			<-stopped
		})
	}
}

func isTerminal(file *os.File) bool {
	info, err := file.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

func sanitizeAuditText(value string) string {
	value = strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == '\t' {
			return ' '
		}
		if r < 32 || r == 127 {
			return -1
		}
		return r
	}, value)
	value = strings.Join(strings.Fields(value), " ")
	runes := []rune(value)
	if len(runes) > 200 {
		return string(runes[:200]) + "…"
	}
	return value
}

func ensureConfig(store *config.Store) (config.Config, error) {
	exists, err := store.Exists()
	if err != nil {
		return config.Config{}, fmt.Errorf("check AI configuration: %w", err)
	}
	if !exists {
		return config.Config{}, missingConfigError(store)
	}
	cfg, err := store.Load()
	if err != nil {
		return config.Config{}, fmt.Errorf("加载 AI 配置失败，请检查 %s：%w", store.Paths().Config, err)
	}
	return cfg, nil
}

func missingConfigError(store *config.Store) error {
	providers := llm.Providers()
	providerList := make([]string, 0, len(providers))
	for _, provider := range providers {
		providerList = append(providerList, fmt.Sprintf("%s (%s)", provider.DisplayName, provider.ID))
	}
	defaults := config.Defaults()
	return &configurationRequiredError{message: fmt.Sprintf(`未检测到 AI 配置文件，程序已退出。

请手动创建配置目录和文件：
  配置目录：%s
  配置文件：%s

当前支持的模型厂商：%s

config.yaml 示例：
version: 1
provider: %q
base_url: %q
model: "请填写模型名称或推理接入点 ID"
api_key: "请填写 API Key"
request_timeout_seconds: 60
analysis:
  max_rounds: 8
  max_tool_calls: 20
  max_entries_per_call: 100
scan:
  max_entries: 1000000
  follow_symlinks: false
  upload_file_content: false`,
		store.Paths().Directory,
		store.Paths().Config,
		strings.Join(providerList, "、"),
		defaults.Provider,
		defaults.BaseURL,
	)}
}
func printSummary(s *model.ScanResult, md, js string) {
	root := s.RootEntry()
	fmt.Printf("Scanned %d files and %d directories (%d bytes).\nReports: %s, %s\n", root.FileCount, root.DirCount, root.Size, md, js)
}

func runConfig(args []string) error {
	if len(args) == 0 {
		return errors.New("config requires show or path")
	}
	store, err := config.UserStore()
	if err != nil {
		return err
	}
	switch args[0] {
	case "path":
		fmt.Println(store.Paths().Directory)
		return nil
	case "show":
		cfg, err := store.Load()
		if err != nil {
			return err
		}
		cfg.APIKey = mask(cfg.APIKey)
		b, _ := json.MarshalIndent(cfg, "", "  ")
		fmt.Println(string(b))
		return nil
	default:
		return fmt.Errorf("unknown config command %q", args[0])
	}
}
func mask(key string) string {
	if len(key) <= 8 {
		return "****"
	}
	return key[:3] + "****" + key[len(key)-4:]
}
