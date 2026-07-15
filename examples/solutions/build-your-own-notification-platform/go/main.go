// Reference gateway for "Build your own notification platform" (Go). Passes all 9 stages.
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
	"sync/atomic"
)

const queueName = "notifications"

type gwError struct{ code, message string }

func (e *gwError) Error() string { return e.code + ": " + e.message }

type request struct {
	ID     string          `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

type peer struct{ addr string }

type engine struct {
	mu          sync.Mutex
	queue       peer
	scheduler   peer
	ratelimiter peer
	seq         atomic.Uint64
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

func (e *engine) nextID() string {
	return fmt.Sprintf("n-%d", e.seq.Add(1))
}

func (e *engine) dispatchDue() error {
	for {
		var pollRes struct {
			Job *struct {
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
		body, err := json.Marshal(pollRes.Job.Payload)
		if err != nil {
			return err
		}
		if err := rpc(e.queue.addr, "send", map[string]any{
			"queue": queueName,
			"body":  string(body),
		}, nil); err != nil {
			return err
		}
		if err := rpc(e.scheduler.addr, "complete", map[string]any{
			"lease_token": pollRes.Job.LeaseToken,
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
	case "configure_limit":
		var p struct {
			UserID   string `json:"user_id"`
			Limit    int    `json:"limit"`
			WindowMS int    `json:"window_ms"`
		}
		if json.Unmarshal(raw, &p) != nil || p.UserID == "" || p.Limit <= 0 || p.WindowMS <= 0 {
			return nil, &gwError{"INVALID_PARAMS", "configure_limit requires user_id, limit, window_ms"}
		}
		if err := rpc(e.ratelimiter.addr, "configure", map[string]any{
			"key": "user:" + p.UserID, "algorithm": "fixed_window",
			"limit": p.Limit, "window_ms": p.WindowMS,
		}, nil); err != nil {
			return nil, err
		}
		return map[string]any{}, nil
	case "notify":
		var p struct {
			UserID  string `json:"user_id"`
			Channel string `json:"channel"`
			Body    string `json:"body"`
		}
		if json.Unmarshal(raw, &p) != nil || p.UserID == "" || p.Channel == "" {
			return nil, &gwError{"INVALID_PARAMS", "notify requires user_id, channel, body"}
		}
		var takeRes struct {
			Allowed bool `json:"allowed"`
		}
		if err := rpc(e.ratelimiter.addr, "take", map[string]any{
			"key": "user:" + p.UserID, "cost": 1,
		}, &takeRes); err != nil {
			if re, ok := err.(*rpcErr); ok && re.code == "KEY_NOT_FOUND" {
				return nil, &gwError{"INVALID_PARAMS", "limit not configured for user"}
			}
			return nil, err
		}
		if !takeRes.Allowed {
			return map[string]any{"notification_id": nil, "queued": false, "rate_limited": true}, nil
		}
		id := e.nextID()
		env := map[string]any{
			"notification_id": id, "user_id": p.UserID, "channel": p.Channel, "body": p.Body,
		}
		body, _ := json.Marshal(env)
		if err := rpc(e.queue.addr, "send", map[string]any{
			"queue": queueName, "body": string(body),
		}, nil); err != nil {
			return nil, err
		}
		return map[string]any{"notification_id": id, "queued": true}, nil
	case "schedule_notify":
		var p struct {
			UserID  string `json:"user_id"`
			Channel string `json:"channel"`
			Body    string `json:"body"`
			DelayMS int    `json:"delay_ms"`
		}
		if json.Unmarshal(raw, &p) != nil || p.UserID == "" || p.Channel == "" {
			return nil, &gwError{"INVALID_PARAMS", "schedule_notify requires user_id, channel, body, delay_ms"}
		}
		id := e.nextID()
		env := map[string]any{
			"notification_id": id, "user_id": p.UserID, "channel": p.Channel, "body": p.Body,
		}
		var res struct {
			JobID string `json:"job_id"`
		}
		if err := rpc(e.scheduler.addr, "schedule", map[string]any{
			"payload": env, "delay_ms": p.DelayMS,
		}, &res); err != nil {
			return nil, err
		}
		return map[string]any{"notification_id": id, "job_id": res.JobID}, nil
	case "receive":
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
			return map[string]any{"notification": nil}, nil
		}
		var env struct {
			NotificationID string `json:"notification_id"`
			UserID         string `json:"user_id"`
			Channel        string `json:"channel"`
			Body           string `json:"body"`
		}
		if json.Unmarshal([]byte(recvRes.Message.Body), &env) != nil {
			return nil, &gwError{"INTERNAL", "bad queue message body"}
		}
		return map[string]any{
			"notification": map[string]any{
				"notification_id": env.NotificationID,
				"user_id":         env.UserID,
				"channel":         env.Channel,
				"body":            env.Body,
				"receipt":         recvRes.Message.Receipt,
			},
		}, nil
	case "ack":
		var p struct {
			Receipt string `json:"receipt"`
		}
		if json.Unmarshal(raw, &p) != nil || p.Receipt == "" {
			return nil, &gwError{"INVALID_PARAMS", "ack requires receipt"}
		}
		if err := rpc(e.queue.addr, "ack", map[string]any{"receipt": p.Receipt}, nil); err != nil {
			return nil, err
		}
		return map[string]any{}, nil
	default:
		return nil, &gwError{"UNKNOWN_METHOD", fmt.Sprintf("unknown method %q", method)}
	}
}

func main() {
	port := flag.Int("port", 0, "")
	flag.String("data-dir", "", "")
	flag.Parse()
	eng := &engine{
		queue:       peer{addr: os.Getenv("QUEUE_ADDR")},
		scheduler:   peer{addr: os.Getenv("SCHEDULER_ADDR")},
		ratelimiter: peer{addr: os.Getenv("RATE_LIMITER_ADDR")},
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
