package harness

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
)

// Program manages the lifecycle of the user's submission process.
// It survives restarts within a stage (same data dir, new port allowed but we
// keep the same port for simplicity).
type Program struct {
	path    string
	port    int
	dataDir string
	logf    func(format string, args ...any)

	cmd *exec.Cmd
}

func NewProgram(path string, logf func(string, ...any)) (*Program, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	port, err := freePort()
	if err != nil {
		return nil, fmt.Errorf("allocating port: %w", err)
	}
	dataDir, err := os.MkdirTemp("", "open-crafters-data-*")
	if err != nil {
		return nil, fmt.Errorf("creating data dir: %w", err)
	}
	return &Program{path: abs, port: port, dataDir: dataDir, logf: logf}, nil
}

func (p *Program) Addr() string { return fmt.Sprintf("127.0.0.1:%d", p.port) }

// Start launches the process and waits until it accepts TCP connections.
func (p *Program) Start() error {
	if p.cmd != nil {
		return fmt.Errorf("program already running")
	}
	cmd := exec.Command(p.path, "--port", fmt.Sprint(p.port), "--data-dir", p.dataDir)
	cmd.Dir = filepath.Dir(p.path)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	cmd.Stderr = cmd.Stdout
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("starting %s: %w", p.path, err)
	}
	p.cmd = cmd
	go func() {
		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			p.logf("[your_program] %s", scanner.Text())
		}
	}()

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", p.Addr(), 250*time.Millisecond)
		if err == nil {
			conn.Close()
			return nil
		}
		if cmd.ProcessState != nil {
			return fmt.Errorf("program exited before accepting connections")
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("program did not accept connections on %s within 10s", p.Addr())
}

// Kill sends SIGKILL to the program's process group, simulating a crash.
func (p *Program) Kill() {
	if p.cmd == nil {
		return
	}
	syscall.Kill(-p.cmd.Process.Pid, syscall.SIGKILL)
	p.cmd.Wait()
	p.cmd = nil
	// Give the OS a moment to release the listening socket.
	time.Sleep(100 * time.Millisecond)
}

// Cleanup terminates the process and removes the data directory.
func (p *Program) Cleanup() {
	p.Kill()
	os.RemoveAll(p.dataDir)
}

func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}
