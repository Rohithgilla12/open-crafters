package progress

import (
	"testing"
)

func TestMergeKeepsEarliestTimestamps(t *testing.T) {
	dst := &File{Challenges: map[string]*ChallengeProgress{
		"build-your-own-wal": {
			Passed: map[string]string{"bind": "2026-01-02T00:00:00Z"},
			Read:   map[string]string{"bind": "2026-01-01T00:00:00Z"},
		},
	}}
	src := &File{Challenges: map[string]*ChallengeProgress{
		"build-your-own-wal": {
			Passed: map[string]string{"bind": "2026-01-03T00:00:00Z", "kv": "2026-01-04T00:00:00Z"},
			Read:   map[string]string{"kv": "2026-01-03T00:00:00Z"},
		},
		"build-your-own-queue": {
			Passed: map[string]string{"bind": "2026-01-05T00:00:00Z"},
		},
	}}
	Merge(dst, src)
	wal := dst.Challenges["build-your-own-wal"]
	if wal.Passed["bind"] != "2026-01-02T00:00:00Z" {
		t.Fatalf("bind passed: got %q, want earlier dst timestamp", wal.Passed["bind"])
	}
	if wal.Passed["kv"] != "2026-01-04T00:00:00Z" {
		t.Fatalf("kv passed: got %q", wal.Passed["kv"])
	}
	if wal.Read["bind"] != "2026-01-01T00:00:00Z" {
		t.Fatalf("bind read: got %q", wal.Read["bind"])
	}
	if wal.Read["kv"] != "2026-01-03T00:00:00Z" {
		t.Fatalf("kv read: got %q", wal.Read["kv"])
	}
	if dst.Challenges["build-your-own-queue"].Passed["bind"] != "2026-01-05T00:00:00Z" {
		t.Fatalf("queue bind not merged")
	}
}

func TestNormalizeSetsVersion(t *testing.T) {
	f := Normalize(&File{})
	if f.Version != FormatVersion {
		t.Fatalf("version = %d, want %d", f.Version, FormatVersion)
	}
}
