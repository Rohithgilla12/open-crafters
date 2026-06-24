package harness

import (
	"io"
	"net"
	"sync/atomic"
)

// Switch forwards TCP connections from a listen address to a target address.
// When blocked, new connections are accepted and immediately closed.
type Switch struct {
	listener net.Listener
	target  string
	blocked atomic.Bool
}

func newSwitch(target string) (*Switch, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	s := &Switch{listener: ln, target: target}
	go s.serve()
	return s, nil
}

func (s *Switch) Addr() string { return s.listener.Addr().String() }

func (s *Switch) SetBlocked(blocked bool) { s.blocked.Store(blocked) }

func (s *Switch) Close() { s.listener.Close() }

func (s *Switch) serve() {
	for {
		in, err := s.listener.Accept()
		if err != nil {
			return
		}
		if s.blocked.Load() {
			in.Close()
			continue
		}
		go s.forward(in)
	}
}

func (s *Switch) forward(in net.Conn) {
	defer in.Close()
	out, err := net.Dial("tcp", s.target)
	if err != nil {
		return
	}
	defer out.Close()
	done := make(chan struct{}, 2)
	go func() { io.Copy(out, in); done <- struct{}{} }()
	go func() { io.Copy(in, out); done <- struct{}{} }()
	<-done
}
