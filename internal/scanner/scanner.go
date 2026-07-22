package scanner

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"big-files/internal/model"
)

type Options struct {
	MaxEntries int
	Progress   func(Progress)
}

type Progress struct {
	Entries int
	Files   int
	Dirs    int
}

func Scan(ctx context.Context, root string, opts Options) (*model.ScanResult, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve root: %w", err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return nil, fmt.Errorf("stat root: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("root is not a directory: %s", abs)
	}
	res := &model.ScanResult{Root: abs, StartedAt: time.Now(), Entries: map[string]*model.FileEntry{}, Complete: true}
	res.Entries["."] = &model.FileEntry{Path: ".", Name: filepath.Base(abs), Type: "directory", ModifiedAt: info.ModTime(), Extensions: map[string]model.ExtensionStat{}}
	files, dirs := 0, 0
	lastProgress := time.Now()
	err = filepath.WalkDir(abs, func(path string, d fs.DirEntry, walkErr error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		rel, relErr := filepath.Rel(abs, path)
		if relErr != nil {
			return relErr
		}
		rel = filepath.ToSlash(rel)
		if walkErr != nil {
			res.Complete = false
			res.Errors = append(res.Errors, fmt.Sprintf("%s: %v", rel, walkErr))
			return nil
		}
		if rel == "." {
			return nil
		}
		if opts.MaxEntries > 0 && len(res.Entries) >= opts.MaxEntries {
			res.Complete = false
			return fmt.Errorf("scan entry limit %d exceeded", opts.MaxEntries)
		}
		fi, infoErr := d.Info()
		if infoErr != nil {
			res.Complete = false
			res.Errors = append(res.Errors, fmt.Sprintf("%s: %v", rel, infoErr))
			res.Entries[rel] = &model.FileEntry{Path: rel, Name: d.Name(), Type: "unknown", Error: infoErr.Error()}
			return nil
		}
		typ := "file"
		if d.Type()&os.ModeSymlink != 0 {
			typ = "symlink"
		} else if d.IsDir() {
			typ = "directory"
		}
		e := &model.FileEntry{Path: rel, Name: d.Name(), Type: typ, ModifiedAt: fi.ModTime()}
		if typ == "file" {
			e.Size = fi.Size()
			e.Extension = strings.ToLower(filepath.Ext(d.Name()))
			files++
		}
		if typ == "directory" {
			e.Extensions = map[string]model.ExtensionStat{}
			dirs++
		}
		res.Entries[rel] = e
		if opts.Progress != nil && (len(res.Entries)%1000 == 0 || time.Since(lastProgress) >= time.Second) {
			opts.Progress(Progress{Entries: len(res.Entries), Files: files, Dirs: dirs})
			lastProgress = time.Now()
		}
		parent := filepath.ToSlash(filepath.Dir(rel))
		if p := res.Entries[parent]; p != nil {
			p.ChildCount++
		}
		return nil
	})
	if err != nil {
		res.Complete = false
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		res.Errors = append(res.Errors, err.Error())
	}
	aggregate(res)
	res.FinishedAt = time.Now()
	if opts.Progress != nil {
		opts.Progress(Progress{Entries: len(res.Entries), Files: files, Dirs: dirs})
	}
	return res, nil
}

func aggregate(res *model.ScanResult) {
	for path, e := range res.Entries {
		if path == "." {
			continue
		}
		parent := filepath.ToSlash(filepath.Dir(path))
		for {
			p := res.Entries[parent]
			if p == nil {
				break
			}
			if e.Type == "file" {
				p.Size += e.Size
				p.FileCount++
				st := p.Extensions[e.Extension]
				st.Count++
				st.Size += e.Size
				p.Extensions[e.Extension] = st
				if p.OldestChildAt.IsZero() || e.ModifiedAt.Before(p.OldestChildAt) {
					p.OldestChildAt = e.ModifiedAt
				}
				if p.NewestChildAt.IsZero() || e.ModifiedAt.After(p.NewestChildAt) {
					p.NewestChildAt = e.ModifiedAt
				}
			} else if e.Type == "directory" {
				p.DirCount++
			}
			if parent == "." {
				break
			}
			parent = filepath.ToSlash(filepath.Dir(parent))
		}
	}
}
