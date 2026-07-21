package scanner

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestScanAggregatesMetadataWithoutFollowingSymlink(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "cache"), 0755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(root, "cache", "old.log")
	if err := os.WriteFile(file, []byte("12345"), 0644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(file, old, old); err != nil {
		t.Fatal(err)
	}
	_ = os.Symlink(filepath.Join(root, "cache"), filepath.Join(root, "link"))
	got, err := Scan(context.Background(), root, Options{MaxEntries: 100})
	if err != nil {
		t.Fatal(err)
	}
	r := got.RootEntry()
	if r.Size != 5 || r.FileCount != 1 || r.DirCount != 1 {
		t.Fatalf("unexpected aggregate: %+v", r)
	}
	if got.Entries["cache/old.log"].Extension != ".log" {
		t.Fatal("extension missing")
	}
	if link := got.Entries["link"]; link != nil && link.Type != "symlink" {
		t.Fatalf("link followed: %+v", link)
	}
}

func TestScanHonorsEntryLimit(t *testing.T) {
	root := t.TempDir()
	for _, n := range []string{"a", "b"} {
		if err := os.WriteFile(filepath.Join(root, n), []byte(n), 0644); err != nil {
			t.Fatal(err)
		}
	}
	got, err := Scan(context.Background(), root, Options{MaxEntries: 2})
	if err != nil {
		t.Fatal(err)
	}
	if got.Complete {
		t.Fatal("expected incomplete scan")
	}
}
