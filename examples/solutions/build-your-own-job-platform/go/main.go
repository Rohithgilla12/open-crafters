// Reference gateway for "Build your own job platform" (Go). Passes all 9 stages.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"sync"
)

const (
	queueName = "jobs"
	lockName  = "dispatcher"
	holderID  = "gateway"
)

type gwError struct{ code, message string }

func (e *gwError) Error() string { return e.code + ": " + e.message }

type request struct {
	ID     string          `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

type peer struct{ addr string }

type engine struct {
	mu        sync.Mutex
	scheduler peer
	queue     peer
	lock      peer
}

type rpcErr struct{ code, message string }

func (e *rpcErr) Error() string { return e.code + ": " + e.message }

func rpc(addr, method string, params map[string]any, out any) error {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return err
	}
	defer conn.Close()
	req, _ := json.Marshal(map[string]any{"id": "1", "method": method, "params": params})
	req = append(req, '\n')
	if _, err := conn.Write(req); err != nil {
		return err
	}
	sc := bufio.NewScanner(conn)
	sc.Buffer(make([]byte, 0, 1024*1024), 1024*1024)
	if !sc.Scan() {
		return fmt.Errorf("no response from %s", addr)
	}
	var resp struct {
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(sc.Bytes(), &resp); err != nil {
		return err
	}
	if resp.Error != nil {
		return &rpcErr{resp.Error.Code, resp.Error.Message}
	}
	if out != nil && len(resp.Result) > 0 {
		return json.Unmarshal(resp.Result, out)
	}
	return nil
}

func (e *engine) dispatchDue() error {
	var tryRes struct {
		Acquired bool   `json:"acquired"`
		Token    string `json:"token"`
	}
	if err := rpc(e.lock.addr, "try_acquire", map[string]any{
		"name": lockName, "holder_id": holderID, "lease_ms": 3000,
	}, &tryRes); err != nil || !tryRes.Acquired {
		return nil
	}
	defer rpc(e.lock.addr, "release", map[string]any{"name": lockName, "token": tryRes.Token}, nil)

	for {
		var pollRes struct {
			Job *struct {
				JobID      string `json:"job_id"`
				Payload    any    `json:"payload"`
				LeaseToken string `json:"lease_token"`
			} `json:"job"`
		}
		if err := rpc(e.scheduler.addr, "poll", nil, &pollRes); err != nil {
			return err
		}
		if pollRes.Job == nil {
			break
		}
		body, _ := json.Marshal(map[string]any{
			"job_id":       pollRes.Job.JobID,
			"payload":      pollRes.Job.Payload,
			"lease_token":  pollRes.Job.LeaseToken,
		})
		if err := rpc(e.queue.addr, "send", map[string]any{
			"queue": queueName,
			"body":  string(body),
		}, nil); err != nil {
			return err
		}
	}
	return nil
}

func (e *engine) handle(method string, raw json.RawMessage) (any, error) {
	switch method {
	case "ping":
		return map[string]string{"message": "pong"}, nil
	case "submit_job":
		var p struct {
			Payload  any `json:"payload"`
			DelayMS  int `json:"delay_ms"`
		}
		if json.Unmarshal(raw, &p) != nil {
			return nil, &gwError{"INVALID_PARAMS", "submit_job requires payload and delay_ms"}
		}
		var res struct {
			JobID string `json:"job_id"`
		}
		if err := rpc(e.scheduler.addr, "schedule", map[string]any{
			"payload": p.Payload, "delay_ms": p.DelayMS,
		}, &res); err != nil {
			return nil, err
		}
		return map[string]string{"job_id": res.JobID}, nil
	case "receive_work":
		if err := e.dispatchDue(); err != nil {
			return nil, err
		}
		var recvRes struct {
			Message *struct {
				Body    string `json:"body"`
				Receipt string `json:"receipt"`
			} `json:"message"`
		}
		if err := rpc(e.queue.addr, "receive", map[string]any{
			"queue": queueName, "visibility_timeout_ms": 30000,
		}, &recvRes); err != nil {
			return nil, err
		}
		if recvRes.Message == nil {
			return map[string]any{"work": nil}, nil
		}
		var body struct {
			JobID      string `json:"job_id"`
			Payload    any    `json:"payload"`
			LeaseToken string `json:"lease_token"`
		}
		if json.Unmarshal([]byte(recvRes.Message.Body), &body) != nil {
			return nil, &gwError{"INTERNAL", "bad queue message body"}
		}
		return map[string]any{
			"work": map[string]any{
				"job_id":       body.JobID,
				"payload":      body.Payload,
				"lease_token":  body.LeaseToken,
				"receipt":      recvRes.Message.Receipt,
			},
		}, nil
	case "complete_work":
		var p struct {
			LeaseToken string `json:"lease_token"`
			Receipt    string `json:"receipt"`
		}
		if json.Unmarshal(raw, &p) != nil || p.LeaseToken == "" || p.Receipt == "" {
			return nil, &gwError{"INVALID_PARAMS", "complete_work requires lease_token and receipt"}
		}
		if err := rpc(e.scheduler.addr, "complete", map[string]any{
			"lease_token": p.LeaseToken,
		}, nil); err != nil {
			return nil, err
		}
		if err := rpc(e.queue.addr, "ack", map[string]any{"receipt": p.Receipt}, nil); err != nil {
			return nil, err
		}
		return map[string]any{}, nil
	case "cancel_job":
		var p struct {
			JobID string `json:"job_id"`
		}
		if json.Unmarshal(raw, &p) != nil || p.JobID == "" {
			return nil, &gwError{"INVALID_PARAMS", "cancel_job requires job_id"}
		}
		var res struct {
			Cancelled bool `json:"cancelled"`
		}
		if err := rpc(e.scheduler.addr, "cancel", map[string]any{"job_id": p.JobID}, &res); err != nil {
			return nil, err
		}
		return map[string]bool{"cancelled": res.Cancelled}, nil
	case "get_job":
		var p struct {
			JobID string `json:"job_id"`
		}
		if json.Unmarshal(raw, &p) != nil || p.JobID == "" {
			return nil, &gwError{"INVALID_PARAMS", "get_job requires job_id"}
		}
		var res struct {
			JobID  string `json:"job_id"`
			Status string `json:"status"`
		}
		if err := rpc(e.scheduler.addr, "get_job", map[string]any{"job_id": p.JobID}, &res); err != nil {
			return nil, err
		}
		return map[string]string{"status": res.Status}, nil
	default:
		return nil, &gwError{"UNKNOWN_METHOD", fmt.Sprintf("unknown method %q", method)}
	}
}

func main() {
	port := flag.Int("port", 0, "")
	flag.String("data-dir", "", "")
	flag.Parse()
	eng := &engine{
		scheduler: peer{addr: os.Getenv("SCHEDULER_ADDR")},
		queue:     peer{addr: os.Getenv("QUEUE_ADDR")},
		lock:      peer{addr: os.Getenv("LOCK_ADDR")},
	}
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
					if ge, ok := err.(*gwError); ok {
						resp = map[string]any{"id": req.ID, "error": map[string]string{"code": ge.code, "message": ge.message}}
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
