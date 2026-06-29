// Reference solution for "Build your own object store" (Go). Passes all 9 stages.
package main

import (
	"bufio"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"sort"
	"sync"
)

type osError struct{ code, message string }

func (e *osError) Error() string { return e.code + ": " + e.message }

type request struct {
	ID     string          `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

func bodyETag(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func randID() string {
	var b [16]byte
	rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

type multipart struct {
	key   string
	parts map[int][]byte
}

type store struct {
	mu       sync.Mutex
	objects  map[string][]byte
	uploads  map[string]*multipart
	snapPath string
}

func newStore(dataDir string) *store {
	s := &store{
		objects:  map[string][]byte{},
		uploads:  map[string]*multipart{},
		snapPath: filepath.Join(dataDir, "state.json"),
	}
	s.load()
	return s
}

type persistPart struct {
	Num  int    `json:"part_number"`
	Body string `json:"body_b64"`
}
type persistUpload struct {
	Key   string        `json:"key"`
	Parts []persistPart `json:"parts"`
}
type persistState struct {
	Objects map[string]string       `json:"objects_b64"`
	Uploads map[string]persistUpload `json:"uploads"`
}

func (s *store) load() {
	data, err := os.ReadFile(s.snapPath)
	if err != nil {
		return
	}
	var st persistState
	if json.Unmarshal(data, &st) != nil {
		return
	}
	for k, b64 := range st.Objects {
		raw, err := base64.StdEncoding.DecodeString(b64)
		if err == nil {
			s.objects[k] = raw
		}
	}
	for id, pu := range st.Uploads {
		mp := &multipart{key: pu.Key, parts: map[int][]byte{}}
		for _, p := range pu.Parts {
			raw, err := base64.StdEncoding.DecodeString(p.Body)
			if err == nil {
				mp.parts[p.Num] = raw
			}
		}
		s.uploads[id] = mp
	}
}

func (s *store) persist() {
	st := persistState{Objects: map[string]string{}, Uploads: map[string]persistUpload{}}
	for k, body := range s.objects {
		st.Objects[k] = base64.StdEncoding.EncodeToString(body)
	}
	for id, mp := range s.uploads {
		pu := persistUpload{Key: mp.key}
		for num, body := range mp.parts {
			pu.Parts = append(pu.Parts, persistPart{Num: num, Body: base64.StdEncoding.EncodeToString(body)})
		}
		sort.Slice(pu.Parts, func(i, j int) bool { return pu.Parts[i].Num < pu.Parts[j].Num })
		st.Uploads[id] = pu
	}
	data, _ := json.Marshal(st)
	tmp := s.snapPath + ".tmp"
	if os.WriteFile(tmp, data, 0o644) == nil {
		os.Rename(tmp, s.snapPath)
	}
}

func (s *store) put(key, body string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	raw := []byte(body)
	s.objects[key] = raw
	s.persist()
	return bodyETag(raw), nil
}

func (s *store) get(key string) (map[string]any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	raw, ok := s.objects[key]
	if !ok {
		return nil, &osError{"NOT_FOUND", fmt.Sprintf("no such key %q", key)}
	}
	return map[string]any{
		"found": true,
		"body":  string(raw),
		"etag":  bodyETag(raw),
		"size":  len(raw),
	}, nil
}

func (s *store) head(key string) (map[string]any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	raw, ok := s.objects[key]
	if !ok {
		return nil, &osError{"NOT_FOUND", fmt.Sprintf("no such key %q", key)}
	}
	return map[string]any{
		"found": true,
		"etag":  bodyETag(raw),
		"size":  len(raw),
	}, nil
}

func (s *store) delete(key string) map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.objects[key]; !ok {
		return map[string]any{"deleted": false}
	}
	delete(s.objects, key)
	s.persist()
	return map[string]any{"deleted": true}
}

func (s *store) list(prefix string) map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	var keys []string
	for k := range s.objects {
		if prefix == "" || len(k) >= len(prefix) && k[:len(prefix)] == prefix {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	return map[string]any{"keys": keys}
}

func (s *store) createMultipart(key string) map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	id := randID()
	s.uploads[id] = &multipart{key: key, parts: map[int][]byte{}}
	s.persist()
	return map[string]any{"upload_id": id}
}

func (s *store) uploadPart(uploadID string, partNumber int, body string) (map[string]any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	mp, ok := s.uploads[uploadID]
	if !ok {
		return nil, &osError{"NO_SUCH_UPLOAD", fmt.Sprintf("no upload %q", uploadID)}
	}
	raw := []byte(body)
	mp.parts[partNumber] = raw
	s.persist()
	return map[string]any{"etag": bodyETag(raw)}, nil
}

type partSpec struct {
	PartNumber int    `json:"part_number"`
	ETag       string `json:"etag"`
}

func (s *store) completeMultipart(uploadID string, parts []partSpec) (map[string]any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	mp, ok := s.uploads[uploadID]
	if !ok {
		return nil, &osError{"NO_SUCH_UPLOAD", fmt.Sprintf("no upload %q", uploadID)}
	}
	if len(parts) == 0 {
		return nil, &osError{"INVALID_PART", "no parts provided"}
	}
	prev := 0
	var assembled []byte
	for i, p := range parts {
		if i > 0 && p.PartNumber <= prev {
			return nil, &osError{"INVALID_PART", "parts must be in ascending part_number order"}
		}
		raw, ok := mp.parts[p.PartNumber]
		if !ok {
			return nil, &osError{"INVALID_PART", fmt.Sprintf("missing part %d", p.PartNumber)}
		}
		etag := bodyETag(raw)
		if etag != p.ETag {
			return nil, &osError{"INVALID_PART", fmt.Sprintf("etag mismatch for part %d", p.PartNumber)}
		}
		assembled = append(assembled, raw...)
		prev = p.PartNumber
	}
	delete(s.uploads, uploadID)
	s.objects[mp.key] = assembled
	s.persist()
	return map[string]any{"etag": bodyETag(assembled)}, nil
}

type keyParams struct {
	Key string `json:"key"`
}
type putParams struct {
	Key  string `json:"key"`
	Body string `json:"body"`
}
type listParams struct {
	Prefix string `json:"prefix"`
}
type uploadPartParams struct {
	UploadID   string `json:"upload_id"`
	PartNumber int    `json:"part_number"`
	Body       string `json:"body"`
}
type completeParams struct {
	UploadID string     `json:"upload_id"`
	Parts    []partSpec `json:"parts"`
}

func (s *store) handle(method string, raw json.RawMessage) (any, error) {
	switch method {
	case "ping":
		return map[string]string{"message": "pong"}, nil
	case "put":
		var p putParams
		if json.Unmarshal(raw, &p) != nil || p.Key == "" {
			return nil, &osError{"INVALID_PARAMS", "put requires key and body"}
		}
		etag, err := s.put(p.Key, p.Body)
		return map[string]any{"etag": etag}, err
	case "get":
		var p keyParams
		if json.Unmarshal(raw, &p) != nil || p.Key == "" {
			return nil, &osError{"INVALID_PARAMS", "get requires key"}
		}
		return s.get(p.Key)
	case "head":
		var p keyParams
		if json.Unmarshal(raw, &p) != nil || p.Key == "" {
			return nil, &osError{"INVALID_PARAMS", "head requires key"}
		}
		return s.head(p.Key)
	case "delete":
		var p keyParams
		if json.Unmarshal(raw, &p) != nil || p.Key == "" {
			return nil, &osError{"INVALID_PARAMS", "delete requires key"}
		}
		return s.delete(p.Key), nil
	case "list":
		var p listParams
		if len(raw) > 0 {
			json.Unmarshal(raw, &p)
		}
		return s.list(p.Prefix), nil
	case "create_multipart":
		var p keyParams
		if json.Unmarshal(raw, &p) != nil || p.Key == "" {
			return nil, &osError{"INVALID_PARAMS", "create_multipart requires key"}
		}
		return s.createMultipart(p.Key), nil
	case "upload_part":
		var p uploadPartParams
		if json.Unmarshal(raw, &p) != nil || p.UploadID == "" || p.PartNumber < 1 {
			return nil, &osError{"INVALID_PARAMS", "upload_part requires upload_id, part_number, body"}
		}
		return s.uploadPart(p.UploadID, p.PartNumber, p.Body)
	case "complete_multipart":
		var p completeParams
		if json.Unmarshal(raw, &p) != nil || p.UploadID == "" {
			return nil, &osError{"INVALID_PARAMS", "complete_multipart requires upload_id and parts"}
		}
		return s.completeMultipart(p.UploadID, p.Parts)
	default:
		return nil, &osError{"UNKNOWN_METHOD", fmt.Sprintf("unknown method %q", method)}
	}
}

func main() {
	port := flag.Int("port", 0, "")
	dataDir := flag.String("data-dir", "", "")
	flag.Parse()
	if *dataDir != "" {
		os.MkdirAll(*dataDir, 0o755)
	}
	st := newStore(*dataDir)

	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", *port))
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("listening on %s", ln.Addr())
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Fatal(err)
		}
		go func(c net.Conn) {
			defer c.Close()
			sc := bufio.NewScanner(c)
			sc.Buffer(make([]byte, 0, 1024*1024), 1024*1024)
			enc := json.NewEncoder(c)
			for sc.Scan() {
				var req request
				if json.Unmarshal(sc.Bytes(), &req) != nil {
					continue
				}
				res, err := st.handle(req.Method, req.Params)
				if err != nil {
					oe := err.(*osError)
					enc.Encode(map[string]any{"id": req.ID, "error": map[string]string{"code": oe.code, "message": oe.message}})
				} else {
					enc.Encode(map[string]any{"id": req.ID, "result": res})
				}
			}
		}(conn)
	}
}
