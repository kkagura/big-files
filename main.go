package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
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
	s, err := scanner.Scan(context.Background(), *root, scanner.Options{MaxEntries: *max})
	if err != nil {
		return err
	}
	doc := report.Document{GeneratedAt: time.Now(), Scan: s}
	if err = report.WriteMarkdown(*md, doc); err != nil {
		return err
	}
	if err = report.WriteJSON(*js, doc); err != nil {
		return err
	}
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
	s, err := scanner.Scan(context.Background(), *root, scanner.Options{MaxEntries: cfg.Scan.MaxEntries})
	if err != nil {
		return err
	}
	client, err := volcengine.New(cfg.APIKey, cfg.BaseURL, time.Duration(cfg.RequestTimeoutSeconds)*time.Second)
	if err != nil {
		return err
	}
	orch := agent.Orchestrator{Client: client, Tools: agent.NewTools(s, cfg.Analysis.MaxEntriesPerCall), Options: agent.Options{Model: cfg.Model, MaxRounds: cfg.Analysis.MaxRounds, MaxToolCalls: cfg.Analysis.MaxToolCalls, MaxEntriesPerCall: cfg.Analysis.MaxEntriesPerCall}}
	result, analysisErr := orch.Run(context.Background())
	doc := report.Document{GeneratedAt: time.Now(), Model: cfg.Model, Scan: s, Analysis: &result}
	if err = report.WriteMarkdown(*md, doc); err != nil {
		return err
	}
	if err = report.WriteJSON(*js, doc); err != nil {
		return err
	}
	printSummary(s, *md, *js)
	if analysisErr != nil {
		return fmt.Errorf("AI analysis degraded; partial report written: %w", analysisErr)
	}
	return nil
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
