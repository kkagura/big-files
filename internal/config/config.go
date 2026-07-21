package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Analysis struct {
	MaxRounds         int `yaml:"max_rounds" json:"max_rounds"`
	MaxToolCalls      int `yaml:"max_tool_calls" json:"max_tool_calls"`
	MaxEntriesPerCall int `yaml:"max_entries_per_call" json:"max_entries_per_call"`
}
type Scan struct {
	MaxEntries        int  `yaml:"max_entries" json:"max_entries"`
	FollowSymlinks    bool `yaml:"follow_symlinks" json:"follow_symlinks"`
	UploadFileContent bool `yaml:"upload_file_content" json:"upload_file_content"`
}
type Config struct {
	Version               int      `yaml:"version" json:"version"`
	Provider              string   `yaml:"provider" json:"provider"`
	BaseURL               string   `yaml:"base_url" json:"base_url"`
	Model                 string   `yaml:"model" json:"model"`
	APIKey                string   `yaml:"api_key" json:"api_key"`
	RequestTimeoutSeconds int      `yaml:"request_timeout_seconds" json:"request_timeout_seconds"`
	Analysis              Analysis `yaml:"analysis" json:"analysis"`
	Scan                  Scan     `yaml:"scan" json:"scan"`
}
type Paths struct{ Directory, Config string }
type Store struct{ paths Paths }

func Defaults() Config {
	return Config{Version: 1, Provider: "volcengine", BaseURL: "https://ark.cn-beijing.volces.com/api/v3", RequestTimeoutSeconds: 60, Analysis: Analysis{MaxRounds: 8, MaxToolCalls: 20, MaxEntriesPerCall: 100}, Scan: Scan{MaxEntries: 1000000}}
}
func NewStore(dir string) *Store {
	return &Store{paths: Paths{Directory: dir, Config: filepath.Join(dir, "config.yaml")}}
}
func UserStore() (*Store, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	return NewStore(filepath.Join(home, ".big-files")), nil
}
func (s *Store) Paths() Paths { return s.paths }
func (s *Store) Exists() (bool, error) {
	_, err := os.Stat(s.paths.Config)
	if err != nil && !os.IsNotExist(err) {
		return false, err
	}
	return err == nil, nil
}
func (s *Store) Load() (Config, error) {
	c := Defaults()
	b, err := os.ReadFile(s.paths.Config)
	if err != nil {
		return c, err
	}
	if err = yaml.Unmarshal(b, &c); err != nil {
		return c, fmt.Errorf("parse config: %w", err)
	}
	if key := os.Getenv("BIG_FILES_AI_API_KEY"); key != "" {
		c.APIKey = key
	}
	if err = Validate(c); err != nil {
		return c, err
	}
	return c, nil
}
func Validate(c Config) error {
	if c.Version != 1 {
		return fmt.Errorf("unsupported config version %d", c.Version)
	}
	if c.Provider != "volcengine" {
		return fmt.Errorf("unsupported provider %q", c.Provider)
	}
	if c.Model == "" {
		return fmt.Errorf("model is required")
	}
	if c.APIKey == "" {
		return fmt.Errorf("API key is required")
	}
	if c.Scan.FollowSymlinks {
		return fmt.Errorf("follow_symlinks is not supported for safety")
	}
	if c.Scan.UploadFileContent {
		return fmt.Errorf("upload_file_content must remain false")
	}
	return nil
}
