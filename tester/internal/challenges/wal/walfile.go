// Package wal implements the stage tests for the "Build your own WAL"
// challenge. See challenges/build-your-own-wal/PROTOCOL.md.
//
// This file is the on-disk-format toolkit: the tester both parses logs the
// submission writes and crafts logs/snapshots for the submission to recover.
package wal

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"os"
	"path/filepath"
)

// Record is one parsed WAL record plus its location in the file.
type Record struct {
	Op    string `json:"op"`
	Key   string `json:"key"`
	Value string `json:"value"`

	Offset        int64 // file offset of the record's CRC field
	PayloadOffset int64 // file offset of the first payload byte
	End           int64 // file offset just past the payload
}

func encodeRecord(op, key, value string) []byte {
	payload := map[string]string{"op": op, "key": key}
	if op == "set" {
		payload["value"] = value
	}
	body, _ := json.Marshal(payload)
	buf := make([]byte, 8+len(body))
	binary.LittleEndian.PutUint32(buf[4:8], uint32(len(body)))
	copy(buf[8:], body)
	binary.LittleEndian.PutUint32(buf[0:4], crc32.ChecksumIEEE(buf[4:]))
	return buf
}

func walPath(dataDir string) string      { return filepath.Join(dataDir, "wal.log") }
func snapshotPath(dataDir string) string { return filepath.Join(dataDir, "snapshot.json") }

// writeWAL replaces wal.log with the given operations, encoded per spec.
// ops entries are {op, key, value} triples; value is ignored for "del".
func writeWAL(dataDir string, ops [][3]string) error {
	var buf []byte
	for _, op := range ops {
		buf = append(buf, encodeRecord(op[0], op[1], op[2])...)
	}
	return os.WriteFile(walPath(dataDir), buf, 0o644)
}

func writeSnapshot(dataDir string, data map[string]string) error {
	body, _ := json.Marshal(map[string]any{"data": data})
	return os.WriteFile(snapshotPath(dataDir), body, 0o644)
}

// parseWAL reads wal.log and returns all valid records. A missing file
// parses as zero records. If strict is true, anything after the last valid
// record (torn bytes, bad CRC) is an error; otherwise parsing stops silently
// at the first invalid record.
func parseWAL(dataDir string, strict bool) ([]Record, error) {
	data, err := os.ReadFile(walPath(dataDir))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var records []Record
	offset := int64(0)
	for offset < int64(len(data)) {
		rest := data[offset:]
		if len(rest) < 8 {
			if strict {
				return records, fmt.Errorf("wal.log has %d trailing byte(s) at offset %d — too short to be a record header", len(rest), offset)
			}
			return records, nil
		}
		storedCRC := binary.LittleEndian.Uint32(rest[0:4])
		length := binary.LittleEndian.Uint32(rest[4:8])
		if int64(length) > int64(len(rest))-8 {
			if strict {
				return records, fmt.Errorf("record %d at offset %d declares payload length %d but only %d byte(s) remain (torn record?)",
					len(records)+1, offset, length, len(rest)-8)
			}
			return records, nil
		}
		if crc32.ChecksumIEEE(rest[4:8+length]) != storedCRC {
			if strict {
				return records, fmt.Errorf("record %d at offset %d has a CRC mismatch", len(records)+1, offset)
			}
			return records, nil
		}
		var rec Record
		if err := json.Unmarshal(rest[8:8+length], &rec); err != nil {
			return records, fmt.Errorf("record %d at offset %d: payload is not valid JSON: %v", len(records)+1, offset, err)
		}
		if rec.Op != "set" && rec.Op != "del" {
			return records, fmt.Errorf("record %d at offset %d: unknown op %q (expected \"set\" or \"del\")", len(records)+1, offset, rec.Op)
		}
		rec.Offset = offset
		rec.PayloadOffset = offset + 8
		rec.End = offset + 8 + int64(length)
		records = append(records, rec)
		offset = rec.End
	}
	return records, nil
}

// readSnapshot returns the snapshot's data map; a missing file returns nil.
func readSnapshot(dataDir string) (map[string]string, error) {
	body, err := os.ReadFile(snapshotPath(dataDir))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var snap struct {
		Data map[string]string `json:"data"`
	}
	if err := json.Unmarshal(body, &snap); err != nil {
		return nil, fmt.Errorf("snapshot.json is not valid JSON: %v", err)
	}
	if snap.Data == nil {
		return nil, fmt.Errorf(`snapshot.json has no "data" object`)
	}
	return snap.Data, nil
}

// reconstructState computes the state implied by snapshot.json + wal.log,
// exactly as recovery should: snapshot first, log replayed on top.
func reconstructState(dataDir string) (map[string]string, error) {
	state := map[string]string{}
	snap, err := readSnapshot(dataDir)
	if err != nil {
		return nil, err
	}
	for k, v := range snap {
		state[k] = v
	}
	records, err := parseWAL(dataDir, true)
	if err != nil {
		return nil, err
	}
	for _, rec := range records {
		if rec.Op == "set" {
			state[rec.Key] = rec.Value
		} else {
			delete(state, rec.Key)
		}
	}
	return state, nil
}

// truncateFile shortens wal.log by n bytes, simulating a torn tail.
func truncateWALBy(dataDir string, n int64) error {
	info, err := os.Stat(walPath(dataDir))
	if err != nil {
		return err
	}
	if info.Size() < n {
		return fmt.Errorf("wal.log is only %d bytes, cannot truncate %d", info.Size(), n)
	}
	return os.Truncate(walPath(dataDir), info.Size()-n)
}

// flipByte XORs one byte of wal.log at the given offset, simulating media
// corruption.
func flipByte(dataDir string, offset int64) error {
	f, err := os.OpenFile(walPath(dataDir), os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer f.Close()
	b := make([]byte, 1)
	if _, err := f.ReadAt(b, offset); err != nil {
		return err
	}
	b[0] ^= 0xFF
	_, err = f.WriteAt(b, offset)
	return err
}
