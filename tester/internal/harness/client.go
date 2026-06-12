package harness

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"sync/atomic"
	"time"
)

// RPCError is a protocol-level error returned by the server.
type RPCError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *RPCError) Error() string { return fmt.Sprintf("%s: %s", e.Code, e.Message) }

// Client is a newline-delimited-JSON client for one TCP connection.
type Client struct {
	conn   net.Conn
	reader *bufio.Reader
	nextID atomic.Int64
}

func Dial(addr string) (*Client, error) {
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("connecting to %s: %w", addr, err)
	}
	return &Client{conn: conn, reader: bufio.NewReaderSize(conn, 1024*1024)}, nil
}

func (c *Client) Close() { c.conn.Close() }

type request struct {
	ID     string `json:"id"`
	Method string `json:"method"`
	Params any    `json:"params"`
}

type response struct {
	ID     string          `json:"id"`
	Result json.RawMessage `json:"result"`
	Error  *RPCError       `json:"error"`
}

// Call sends a request and decodes the result into out (if out is non-nil).
// A server-sent protocol error is returned as *RPCError; transport or
// malformed-response problems are returned as ordinary errors.
func (c *Client) Call(method string, params any, out any) error {
	id := fmt.Sprint(c.nextID.Add(1))
	if params == nil {
		params = map[string]any{}
	}
	payload, err := json.Marshal(request{ID: id, Method: method, Params: params})
	if err != nil {
		return err
	}
	c.conn.SetDeadline(time.Now().Add(10 * time.Second))
	if _, err := c.conn.Write(append(payload, '\n')); err != nil {
		return fmt.Errorf("sending %s request: %w", method, err)
	}
	line, err := c.reader.ReadBytes('\n')
	if err != nil {
		return fmt.Errorf("reading %s response: %w", method, err)
	}
	var resp response
	if err := json.Unmarshal(line, &resp); err != nil {
		return fmt.Errorf("%s response is not valid JSON: %w (got: %.200s)", method, err, line)
	}
	if resp.ID != id {
		return fmt.Errorf("%s response has id %q, expected %q", method, resp.ID, id)
	}
	if resp.Error != nil {
		if len(resp.Result) > 0 && string(resp.Result) != "null" {
			return fmt.Errorf("%s response contains both result and error", method)
		}
		return resp.Error
	}
	if resp.Result == nil {
		return fmt.Errorf("%s response contains neither result nor error (got: %.200s)", method, line)
	}
	if out != nil {
		if err := json.Unmarshal(resp.Result, out); err != nil {
			return fmt.Errorf("decoding %s result: %w (got: %.200s)", method, err, resp.Result)
		}
	}
	return nil
}
