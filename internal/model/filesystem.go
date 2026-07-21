package model

import "time"

type ExtensionStat struct {
	Count int   `json:"count"`
	Size  int64 `json:"size_bytes"`
}

type FileEntry struct {
	Path          string                   `json:"path"`
	Name          string                   `json:"name"`
	Type          string                   `json:"type"`
	Size          int64                    `json:"size_bytes"`
	ModifiedAt    time.Time                `json:"modified_at"`
	Extension     string                   `json:"extension,omitempty"`
	ChildCount    int                      `json:"child_count,omitempty"`
	FileCount     int                      `json:"file_count,omitempty"`
	DirCount      int                      `json:"dir_count,omitempty"`
	OldestChildAt time.Time                `json:"oldest_child_at,omitempty"`
	NewestChildAt time.Time                `json:"newest_child_at,omitempty"`
	Extensions    map[string]ExtensionStat `json:"extensions,omitempty"`
	Error         string                   `json:"error,omitempty"`
}

type ScanResult struct {
	Root       string                `json:"root"`
	StartedAt  time.Time             `json:"started_at"`
	FinishedAt time.Time             `json:"finished_at"`
	Entries    map[string]*FileEntry `json:"entries"`
	Complete   bool                  `json:"complete"`
	Errors     []string              `json:"errors,omitempty"`
}

func (s *ScanResult) RootEntry() *FileEntry { return s.Entries["."] }
