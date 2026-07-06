// Reference solution for "Build your own harness" (Go). Passes all 9 stages.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

type harnessError struct{ code, message string }

func (e *harnessError) Error() string { return e.code + ": " + e.message }

type request struct {
	ID     string          `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

type childProc struct {
	cmd     *exec.Cmd
	dataDir string
}

type engine struct {
	mu       sync.Mutex
	children []*childProc
}

func (eng *engine) track(cmd *exec.Cmd, dataDir string) {
	eng.mu.Lock()
	eng.children = append(eng.children, &childProc{cmd: cmd, dataDir: dataDir})
	eng.mu.Unlock()
}

func reservePort() (int, net.Listener, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, nil, err
	}
	return ln.Addr().(*net.TCPAddr).Port, ln, nil
}

func waitTCP(addr string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 250*time.Millisecond)
		if err == nil {
			conn.Close()
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for %s", addr)
}

func (eng *engine) spawn(program string) (string, error) {
	if program == "" {
		return "", &harnessError{"INVALID_PARAMS", "spawn requires program"}
	}
	port, hold, err := reservePort()
	if err != nil {
		return "", &harnessError{"SPAWN_FAILED", err.Error()}
	}
	hold.Close()

	dataDir, err := os.MkdirTemp("", "harness-child-*")
	if err != nil {
		return "", &harnessError{"SPAWN_FAILED", err.Error()}
	}

	cmd := exec.Command(program, "--port", fmt.Sprint(port), "--data-dir", dataDir)
	cmd.Dir = filepath.Dir(program)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		os.RemoveAll(dataDir)
		return "", &harnessError{"SPAWN_FAILED", err.Error()}
	}
	eng.track(cmd, dataDir)

	addr := fmt.Sprintf("127.0.0.1:%d", port)
	if err := waitTCP(addr, 10*time.Second); err != nil {
		syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		cmd.Wait()
		os.RemoveAll(dataDir)
		return "", &harnessError{"SPAWN_FAILED", err.Error()}
	}
	return addr, nil
}

func proxyCall(addr, method string, params json.RawMessage) (map[string]any, error) {
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return nil, &harnessError{"SPAWN_FAILED", fmt.Sprintf("dial %s: %v", addr, err)}
	}
	defer conn.Close()

	var p any
	if len(params) == 0 || string(params) == "null" {
		p = map[string]any{}
	} else {
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, &harnessError{"INVALID_PARAMS", "invalid params"}
		}
	}
	req := map[string]any{"id": "1", "method": method, "params": p}
	line, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	conn.SetDeadline(time.Now().Add(10 * time.Second))
	if _, err := conn.Write(append(line, '\n')); err != nil {
		return nil, err
	}
	reader := bufio.NewReader(conn)
	respLine, err := reader.ReadBytes('\n')
	if err != nil {
		return nil, err
	}
	var resp struct {
		Result map[string]any `json:"result"`
		Error  *struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(respLine, &resp) != nil {
		return nil, fmt.Errorf("invalid child response")
	}
	if resp.Error != nil {
		return nil, &harnessError{resp.Error.Code, resp.Error.Message}
	}
	if resp.Result == nil {
		return map[string]any{}, nil
	}
	return resp.Result, nil
}

func subsetMatch(got, expect map[string]any) bool {
	for k, wv := range expect {
		gv, ok := got[k]
		if !ok {
			return false
		}
		gb, _ := json.Marshal(gv)
		wb, _ := json.Marshal(wv)
		if string(gb) != string(wb) {
			return false
		}
	}
	return true
}

func (eng *engine) handle(method string, raw json.RawMessage) (any, error) {
	switch method {
	case "ping":
		return map[string]string{"message": "pong"}, nil
	case "spawn":
		var p struct {
			Program string `json:"program"`
		}
		if json.Unmarshal(raw, &p) != nil || p.Program == "" {
			return nil, &harnessError{"INVALID_PARAMS", "spawn requires program"}
		}
		addr, err := eng.spawn(p.Program)
		if err != nil {
			return nil, err
		}
		return map[string]string{"addr": addr}, nil
	case "call":
		var p struct {
			Addr   string          `json:"addr"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		if json.Unmarshal(raw, &p) != nil || p.Addr == "" || p.Method == "" {
			return nil, &harnessError{"INVALID_PARAMS", "call requires addr and method"}
		}
		return proxyCall(p.Addr, p.Method, p.Params)
	case "run_case":
		var p struct {
			Program string          `json:"program"`
			Method  string          `json:"method"`
			Params  json.RawMessage `json:"params"`
			Expect  map[string]any  `json:"expect"`
		}
		if json.Unmarshal(raw, &p) != nil || p.Program == "" || p.Method == "" || p.Expect == nil {
			return nil, &harnessError{"INVALID_PARAMS", "run_case requires program, method, expect"}
		}
		addr, err := eng.spawn(p.Program)
		if err != nil {
			return nil, err
		}
		got, err := proxyCall(addr, p.Method, p.Params)
		if err != nil {
			return nil, err
		}
		if !subsetMatch(got, p.Expect) {
			return nil, &harnessError{"CASE_FAILED", fmt.Sprintf("got %#v expect subset %#v", got, p.Expect)}
		}
		return map[string]any{}, nil
	default:
		return nil, &harnessError{"UNKNOWN_METHOD", fmt.Sprintf("unknown method %q", method)}
	}
}

func main() {
	port := flag.Int("port", 0, "")
	flag.String("data-dir", "", "")
	flag.Parse()
	eng := &engine{}
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", *port))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("listening on 127.0.0.1:%d\n", *port)
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Fatal(err)
		}
		go func(c net.Conn) {
			defer c.Close()
			sc := bufio.NewScanner(c)
			sc.Buffer(make([]byte, 0, 1024*1024), 1024*1024)
			w := bufio.NewWriter(c)
			for sc.Scan() {
				var req request
				if json.Unmarshal(sc.Bytes(), &req) != nil {
					continue
				}
				res, err := eng.handle(req.Method, req.Params)
				var resp map[string]any
				if err != nil {
					if he, ok := err.(*harnessError); ok {
						resp = map[string]any{"id": req.ID, "error": map[string]string{"code": he.code, "message": he.message}}
					} else {
						resp = map[string]any{"id": req.ID, "error": map[string]string{"code": "INTERNAL", "message": err.Error()}}
					}
				} else {
					resp = map[string]any{"id": req.ID, "result": res}
				}
				b, _ := json.Marshal(resp)
				w.Write(b)
				w.WriteByte('\n')
				w.Flush()
			}
		}(conn)
	}
}
