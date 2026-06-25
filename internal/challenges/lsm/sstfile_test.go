package lsm

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEncodeParseSST(t *testing.T) {
	dir := t.TempDir()
	entries := []Entry{
		{Key: "alpha", Value: "1"},
		{Key: "beta", Value: "2"},
		{Key: "gone", Deleted: true},
	}
	if err := writeSST(dir, 1, entries); err != nil {
		t.Fatal(err)
	}
	got, err := parseSST(sstPath(dir, 1))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(got))
	}
	if got[2].Key != "gone" || !got[2].Deleted {
		t.Fatalf("expected tombstone for gone, got %+v", got[2])
	}
}

func TestParseAllSSTs(t *testing.T) {
	dir := t.TempDir()
	if err := writeSST(dir, 1, []Entry{{Key: "a", Value: "old"}}); err != nil {
		t.Fatal(err)
	}
	if err := writeSST(dir, 2, []Entry{{Key: "a", Value: "new"}, {Key: "b", Value: "2"}}); err != nil {
		t.Fatal(err)
	}
	state, err := parseAllSSTs(dir)
	if err != nil {
		t.Fatal(err)
	}
	if state["a"] != "new" || state["b"] != "2" {
		t.Fatalf("unexpected state: %v", state)
	}
}

func TestListSSTFiles(t *testing.T) {
	dir := t.TempDir()
	if err := writeSST(dir, 2, []Entry{{Key: "z", Value: "1"}}); err != nil {
		t.Fatal(err)
	}
	if err := writeSST(dir, 1, []Entry{{Key: "a", Value: "1"}}); err != nil {
		t.Fatal(err)
	}
	paths, err := listSSTFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 2 {
		t.Fatalf("expected 2 files, got %d", len(paths))
	}
	if filepath.Base(paths[0]) != "000001.sst" {
		t.Fatalf("expected sorted order, got %v", paths)
	}
}

func TestParseSSTBadMagic(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(sstDir(dir), 0o755); err != nil {
		t.Fatal(err)
	}
	path := sstPath(dir, 9)
	if err := os.WriteFile(path, []byte("BAD!"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := parseSST(path); err == nil {
		t.Fatal("expected error for bad magic")
	}
}
