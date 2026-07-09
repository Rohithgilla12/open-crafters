// Reference gateway for "Build your own chat service" (Go). Passes all 9 stages.
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

const deliveryQueue = "delivery"

type gwError struct{ code, message string }

func (e *gwError) Error() string { return e.code + ": " + e.message }

type request struct {
	ID     string          `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

type engine struct {
	mu    sync.Mutex
	idgen string
	log   string
	queue string
	ready bool
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

func (e *engine) ensureStack() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.ready {
		return nil
	}
	if e.idgen == "" || e.log == "" || e.queue == "" {
		return &gwError{"INTERNAL", "missing IDGEN_ADDR, LOG_ADDR, or QUEUE_ADDR"}
	}
	if err := rpc(e.idgen, "configure", map[string]any{"worker_id": 1}, nil); err != nil {
		return err
	}
	e.ready = true
	return nil
}

func (e *engine) handle(method string, raw json.RawMessage) (any, error) {
	if method == "ping" {
		if err := e.ensureStack(); err != nil {
			return nil, err
		}
		return map[string]string{"message": "pong"}, nil
	}
	if err := e.ensureStack(); err != nil {
		return nil, err
	}
	switch method {
	case "send_message":
		var p struct {
			ConversationID string `json:"conversation_id"`
			Sender         string `json:"sender"`
			Body           string `json:"body"`
		}
		if json.Unmarshal(raw, &p) != nil || p.ConversationID == "" || p.Sender == "" || p.Body == "" {
			return nil, &gwError{"INVALID_PARAMS", "send_message requires conversation_id, sender, body"}
		}
		var idRes struct {
			ID string `json:"id"`
		}
		if err := rpc(e.idgen, "next_id", nil, &idRes); err != nil {
			if re, ok := err.(*rpcErr); ok {
				return nil, &gwError{re.code, re.message}
			}
			return nil, err
		}
		env, _ := json.Marshal(map[string]string{
			"message_id":      idRes.ID,
			"conversation_id": p.ConversationID,
			"sender":          p.Sender,
			"body":            p.Body,
		})
		var logRes struct {
			Offset int `json:"offset"`
		}
		if err := rpc(e.log, "append", map[string]any{
			"topic": p.ConversationID, "value": string(env),
		}, &logRes); err != nil {
			return nil, err
		}
		if err := rpc(e.queue, "send", map[string]any{
			"queue": deliveryQueue, "body": string(env),
		}, nil); err != nil {
			return nil, err
		}
		return map[string]any{"message_id": idRes.ID, "offset": logRes.Offset}, nil
	case "read_messages":
		var p struct {
			ConversationID string `json:"conversation_id"`
			Offset         int    `json:"offset"`
			Max            int    `json:"max"`
		}
		if json.Unmarshal(raw, &p) != nil || p.ConversationID == "" {
			return nil, &gwError{"INVALID_PARAMS", "read_messages requires conversation_id"}
		}
		if p.Max <= 0 {
			p.Max = 100
		}
		var logRes struct {
			Records []struct {
				Offset int    `json:"offset"`
				Value  string `json:"value"`
			} `json:"records"`
		}
		if err := rpc(e.log, "read", map[string]any{
			"topic": p.ConversationID, "offset": p.Offset, "max": p.Max,
		}, &logRes); err != nil {
			return nil, err
		}
		return map[string]any{"records": logRes.Records}, nil
	case "poll_delivery":
		var qRes struct {
			Message *struct {
				ID      string `json:"id"`
				Body    string `json:"body"`
				Receipt string `json:"receipt"`
			} `json:"message"`
		}
		if err := rpc(e.queue, "receive", map[string]any{
			"queue": deliveryQueue, "visibility_timeout_ms": 30000,
		}, &qRes); err != nil {
			return nil, err
		}
		if qRes.Message == nil {
			return map[string]any{"message": nil}, nil
		}
		var env struct {
			MessageID      string `json:"message_id"`
			ConversationID string `json:"conversation_id"`
			Sender         string `json:"sender"`
			Body           string `json:"body"`
		}
		if json.Unmarshal([]byte(qRes.Message.Body), &env) != nil {
			return nil, &gwError{"INTERNAL", "invalid queue message envelope"}
		}
		return map[string]any{
			"message": map[string]any{
				"message_id":      env.MessageID,
				"conversation_id": env.ConversationID,
				"sender":          env.Sender,
				"body":            env.Body,
				"receipt":         qRes.Message.Receipt,
			},
		}, nil
	case "ack_delivery":
		var p struct {
			Receipt string `json:"receipt"`
		}
		if json.Unmarshal(raw, &p) != nil || p.Receipt == "" {
			return nil, &gwError{"INVALID_PARAMS", "ack_delivery requires receipt"}
		}
		if err := rpc(e.queue, "ack", map[string]any{"receipt": p.Receipt}, nil); err != nil {
			return nil, err
		}
		return map[string]any{}, nil
	default:
		return nil, &gwError{"UNKNOWN_METHOD", fmt.Sprintf("unknown method %q", method)}
	}
}

func main() {
	port := flag.Int("port", 0, "TCP port")
	flag.String("data-dir", "", "")
	flag.Parse()
	if *port == 0 {
		log.Fatal("usage: your_program.sh --port PORT")
	}
	eng := &engine{
		idgen: os.Getenv("IDGEN_ADDR"),
		log:   os.Getenv("LOG_ADDR"),
		queue: os.Getenv("QUEUE_ADDR"),
	}
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", *port))
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("chat service gateway listening on %s", ln.Addr())
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Print(err)
			continue
		}
		go func(c net.Conn) {
			defer c.Close()
			sc := bufio.NewScanner(c)
			sc.Buffer(make([]byte, 0, 1024*1024), 1024*1024)
			for sc.Scan() {
				var req request
				if json.Unmarshal(sc.Bytes(), &req) != nil {
					continue
				}
				result, err := eng.handle(req.Method, req.Params)
				var resp map[string]any
				if err != nil {
					if ge, ok := err.(*gwError); ok {
						resp = map[string]any{"id": req.ID, "error": map[string]string{"code": ge.code, "message": ge.message}}
					} else {
						resp = map[string]any{"id": req.ID, "error": map[string]string{"code": "INTERNAL", "message": err.Error()}}
					}
				} else {
					resp = map[string]any{"id": req.ID, "result": result}
				}
				out, _ := json.Marshal(resp)
				out = append(out, '\n')
				c.Write(out)
			}
		}(conn)
	}
}
