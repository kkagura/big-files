package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"time"
)

type entry struct {
	name  string
	size  int64
	isDir bool
	err   error
}

func main() {
	dir, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to get working directory: %v\n", err)
		os.Exit(1)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to read directory %q: %v\n", dir, err)
		os.Exit(1)
	}

	stopLoading := startLoading("Calculating sizes")
	results := make([]entry, 0, len(entries))
	for _, item := range entries {
		path := filepath.Join(dir, item.Name())
		info, infoErr := item.Info()
		if infoErr != nil {
			results = append(results, entry{name: item.Name(), err: infoErr})
			continue
		}

		result := entry{
			name:  item.Name(),
			isDir: info.IsDir(),
		}

		if info.IsDir() {
			result.size, result.err = dirSize(path)
		} else {
			result.size = info.Size()
		}

		results = append(results, result)
	}
	stopLoading()

	sort.Slice(results, func(i, j int) bool {
		if results[i].size == results[j].size {
			return results[i].name < results[j].name
		}
		return results[i].size > results[j].size
	})

	fmt.Printf("Items in %s sorted by size:\n\n", dir)
	for _, result := range results {
		kind := "file"
		if result.isDir {
			kind = "dir "
		}

		if result.err != nil {
			fmt.Printf("%12s  %s  %s  error: %v\n", "-", kind, result.name, result.err)
			continue
		}

		fmt.Printf("%12s  %s  %s\n", formatSize(result.size), kind, result.name)
	}
}

func startLoading(message string) func() {
	done := make(chan struct{})
	stopped := make(chan struct{})

	go func() {
		defer close(stopped)

		frames := []byte{'|', '/', '-', '\\'}
		ticker := time.NewTicker(120 * time.Millisecond)
		defer ticker.Stop()

		i := 0
		for {
			select {
			case <-done:
				fmt.Print("\r", clearLine(80), "\r")
				return
			case <-ticker.C:
				fmt.Printf("\r%s %c", message, frames[i%len(frames)])
				i++
			}
		}
	}()

	return func() {
		close(done)
		<-stopped
	}
}

func clearLine(width int) string {
	line := make([]byte, width)
	for i := range line {
		line[i] = ' '
	}
	return string(line)
}

func dirSize(root string) (int64, error) {
	var size int64
	var firstErr error

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			return nil
		}

		if d.IsDir() {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			return nil
		}

		size += info.Size()
		return nil
	})
	if err != nil && firstErr == nil {
		firstErr = err
	}

	return size, firstErr
}

func formatSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}

	value := float64(size)
	for _, suffix := range []string{"KB", "MB", "GB", "TB", "PB"} {
		value /= unit
		if value < unit {
			return fmt.Sprintf("%.1f %s", value, suffix)
		}
	}

	return fmt.Sprintf("%.1f EB", value/unit)
}
