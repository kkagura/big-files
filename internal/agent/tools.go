package agent

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"big-files/internal/model"
)

type Tools struct {
	Scan       *model.ScanResult
	MaxEntries int
	Inspected  map[string]struct{}
}

type listArgs struct {
	Path   string `json:"path"`
	SortBy string `json:"sort_by"`
	Limit  int    `json:"limit"`
}
type inspectArgs struct {
	Path string `json:"path"`
}
type findArgs struct {
	Under         string   `json:"under"`
	OlderThanDays int      `json:"older_than_days"`
	MinSizeBytes  int64    `json:"min_size_bytes"`
	Extensions    []string `json:"extensions"`
	Limit         int      `json:"limit"`
}

func NewTools(scan *model.ScanResult, max int) *Tools {
	if max <= 0 {
		max = 100
	}
	return &Tools{Scan: scan, MaxEntries: max, Inspected: map[string]struct{}{}}
}

func (t *Tools) Execute(name string, raw json.RawMessage) (any, error) {
	switch name {
	case "list_directory":
		var a listArgs
		if err := json.Unmarshal(raw, &a); err != nil {
			return nil, fmt.Errorf("invalid arguments: %w", err)
		}
		return t.list(a)
	case "inspect_path":
		var a inspectArgs
		if err := json.Unmarshal(raw, &a); err != nil {
			return nil, fmt.Errorf("invalid arguments: %w", err)
		}
		return t.inspect(a.Path)
	case "find_candidates":
		var a findArgs
		if err := json.Unmarshal(raw, &a); err != nil {
			return nil, fmt.Errorf("invalid arguments: %w", err)
		}
		return t.find(a)
	default:
		return nil, fmt.Errorf("unsupported tool %q", name)
	}
}

func cleanRelative(path string) (string, error) {
	if path == "" || path == "." {
		return ".", nil
	}
	path = filepath.Clean(filepath.FromSlash(path))
	if filepath.IsAbs(path) || path == ".." || strings.HasPrefix(path, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path must stay inside scan root")
	}
	return filepath.ToSlash(path), nil
}

func (t *Tools) entry(path string) (*model.FileEntry, error) {
	p, err := cleanRelative(path)
	if err != nil {
		return nil, err
	}
	e := t.Scan.Entries[p]
	if e == nil {
		return nil, fmt.Errorf("path %q is not in scan index", p)
	}
	t.Inspected[p] = struct{}{}
	return e, nil
}

func (t *Tools) inspect(path string) (*model.FileEntry, error) { return t.entry(path) }

func (t *Tools) list(a listArgs) (map[string]any, error) {
	p, err := cleanRelative(a.Path)
	if err != nil {
		return nil, err
	}
	e, err := t.entry(p)
	if err != nil {
		return nil, err
	}
	if e.Type != "directory" {
		return nil, fmt.Errorf("path %q is not a directory", p)
	}
	limit := a.Limit
	if limit <= 0 || limit > t.MaxEntries {
		limit = t.MaxEntries
	}
	items := make([]*model.FileEntry, 0)
	for path, item := range t.Scan.Entries {
		if path != p && filepath.ToSlash(filepath.Dir(path)) == p {
			items = append(items, item)
		}
	}
	sort.Slice(items, func(i, j int) bool {
		if a.SortBy == "time" {
			return items[i].ModifiedAt.After(items[j].ModifiedAt)
		}
		if items[i].Size == items[j].Size {
			return items[i].Path < items[j].Path
		}
		return items[i].Size > items[j].Size
	})
	total := len(items)
	if len(items) > limit {
		items = items[:limit]
	}
	for _, item := range items {
		t.Inspected[item.Path] = struct{}{}
	}
	return map[string]any{"path": p, "entries": items, "total": total, "truncated": total - len(items)}, nil
}

func (t *Tools) find(a findArgs) (map[string]any, error) {
	under, err := cleanRelative(a.Under)
	if err != nil {
		return nil, err
	}
	if _, err = t.entry(under); err != nil {
		return nil, err
	}
	limit := a.Limit
	if limit <= 0 || limit > t.MaxEntries {
		limit = t.MaxEntries
	}
	exts := map[string]bool{}
	for _, ext := range a.Extensions {
		exts[strings.ToLower(ext)] = true
	}
	cutoff := time.Now().AddDate(0, 0, -a.OlderThanDays)
	items := make([]*model.FileEntry, 0)
	for path, e := range t.Scan.Entries {
		if path == under || (under != "." && !strings.HasPrefix(path, under+"/")) {
			continue
		}
		if a.MinSizeBytes > 0 && e.Size < a.MinSizeBytes {
			continue
		}
		if a.OlderThanDays > 0 && !e.ModifiedAt.Before(cutoff) {
			continue
		}
		if len(exts) > 0 && !exts[e.Extension] {
			continue
		}
		items = append(items, e)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Size > items[j].Size })
	total := len(items)
	if len(items) > limit {
		items = items[:limit]
	}
	for _, item := range items {
		t.Inspected[item.Path] = struct{}{}
	}
	return map[string]any{"entries": items, "total": total, "truncated": total - len(items)}, nil
}
