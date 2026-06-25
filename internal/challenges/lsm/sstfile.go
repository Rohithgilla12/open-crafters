// Package lsm implements the stage tests for the "Build your own LSM-tree"
// challenge. See challenges/build-your-own-lsm/PROTOCOL.md.
//
// This file is the on-disk-format toolkit: the tester both parses SSTables the
// submission writes and crafts SSTables for the submission to recover.
package lsm

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const sstMagic = "SST1"

// Entry is one key/value pair from an SSTable. Deleted is true when
// value_len was 0 (a tombstone).
type Entry struct {
	Key     string
	Value   string
	Deleted bool
}

func sstDir(dataDir string) string { return filepath.Join(dataDir, "sst") }

func sstPath(dataDir string, seq int) string {
	return filepath.Join(sstDir(dataDir), fmt.Sprintf("%06d.sst", seq))
}

// encodeSST serializes entries into the byte-exact SST1 format.
func encodeSST(entries []Entry) []byte {
	buf := make([]byte, 4+4)
	copy(buf[0:4], sstMagic)
	binary.LittleEndian.PutUint32(buf[4:8], uint32(len(entries)))
	for _, e := range entries {
		keyBytes := []byte(e.Key)
		var valBytes []byte
		if !e.Deleted {
			valBytes = []byte(e.Value)
		}
		rec := make([]byte, 4+len(keyBytes)+4+len(valBytes))
		off := 0
		binary.LittleEndian.PutUint32(rec[off:off+4], uint32(len(keyBytes)))
		off += 4
		copy(rec[off:off+len(keyBytes)], keyBytes)
		off += len(keyBytes)
		binary.LittleEndian.PutUint32(rec[off:off+4], uint32(len(valBytes)))
		off += 4
		copy(rec[off:], valBytes)
		buf = append(buf, rec...)
	}
	return buf
}

// writeSST replaces data-dir/sst/NNNNNN.sst with the given entries.
func writeSST(dataDir string, seq int, entries []Entry) error {
	if err := os.MkdirAll(sstDir(dataDir), 0o755); err != nil {
		return err
	}
	return os.WriteFile(sstPath(dataDir, seq), encodeSST(entries), 0o644)
}

// parseSST reads one SST file and returns its entries in file order.
func parseSST(path string) ([]Entry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(data) < 8 {
		return nil, fmt.Errorf("%s: file is only %d byte(s), expected at least 8 (magic + entry_count)", filepath.Base(path), len(data))
	}
	if string(data[0:4]) != sstMagic {
		return nil, fmt.Errorf("%s: bad magic %q, expected %q", filepath.Base(path), data[0:4], sstMagic)
	}
	count := binary.LittleEndian.Uint32(data[4:8])
	offset := 8
	var entries []Entry
	for i := uint32(0); i < count; i++ {
		if offset+4 > len(data) {
			return entries, fmt.Errorf("%s: entry %d: expected key_len at offset %d but only %d byte(s) remain",
				filepath.Base(path), i+1, offset, len(data)-offset)
		}
		keyLen := binary.LittleEndian.Uint32(data[offset : offset+4])
		offset += 4
		if offset+int(keyLen) > len(data) {
			return entries, fmt.Errorf("%s: entry %d: key_len=%d at offset %d but only %d byte(s) remain",
				filepath.Base(path), i+1, keyLen, offset, len(data)-offset)
		}
		key := string(data[offset : offset+int(keyLen)])
		offset += int(keyLen)
		if offset+4 > len(data) {
			return entries, fmt.Errorf("%s: entry %d: expected value_len after key at offset %d but only %d byte(s) remain",
				filepath.Base(path), i+1, offset, len(data)-offset)
		}
		valLen := binary.LittleEndian.Uint32(data[offset : offset+4])
		offset += 4
		if offset+int(valLen) > len(data) {
			return entries, fmt.Errorf("%s: entry %d: value_len=%d at offset %d but only %d byte(s) remain",
				filepath.Base(path), i+1, valLen, offset, len(data)-offset)
		}
		val := string(data[offset : offset+int(valLen)])
		offset += int(valLen)
		entries = append(entries, Entry{Key: key, Value: val, Deleted: valLen == 0})
	}
	if offset != len(data) {
		return entries, fmt.Errorf("%s: %d trailing byte(s) after %d entries", filepath.Base(path), len(data)-offset, count)
	}
	return entries, nil
}

// listSSTFiles returns paths to *.sst files in data-dir/sst/, sorted by name.
func listSSTFiles(dataDir string) ([]string, error) {
	dir := sstDir(dataDir)
	ents, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, e := range ents {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sst") {
			paths = append(paths, filepath.Join(dir, e.Name()))
		}
	}
	sort.Strings(paths)
	return paths, nil
}

// parseAllSSTs reads every SST in data-dir/sst/ and returns a map of key→value.
// Later files override earlier ones. Tombstones remove keys.
func parseAllSSTs(dataDir string) (map[string]string, error) {
	paths, err := listSSTFiles(dataDir)
	if err != nil {
		return nil, err
	}
	state := map[string]string{}
	for _, p := range paths {
		entries, err := parseSST(p)
		if err != nil {
			return nil, fmt.Errorf("parsing %s: %w", filepath.Base(p), err)
		}
		for _, e := range entries {
			if e.Deleted {
				delete(state, e.Key)
			} else {
				state[e.Key] = e.Value
			}
		}
	}
	return state, nil
}

// checkSSTEntries compares parsed entries against expected {key, value, deleted}
// triples. value is ignored when deleted is true.
func checkSSTEntries(entries []Entry, want [][3]any) error {
	if len(entries) != len(want) {
		return fmt.Errorf("expected %d entr(y/ies) in the SST file, found %d", len(want), len(entries))
	}
	for i, w := range want {
		e := entries[i]
		key := w[0].(string)
		val := w[1].(string)
		deleted := w[2].(bool)
		if e.Key != key || e.Deleted != deleted || (!deleted && e.Value != val) {
			return fmt.Errorf("entry %d: expected {key:%q value:%q deleted:%v}, got {key:%q value:%q deleted:%v}",
				i+1, key, val, deleted, e.Key, e.Value, e.Deleted)
		}
	}
	return nil
}

// countSSTFiles returns the number of .sst files in data-dir/sst/.
func countSSTFiles(dataDir string) (int, error) {
	paths, err := listSSTFiles(dataDir)
	if err != nil {
		return 0, err
	}
	return len(paths), nil
}
