package harness

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// Cluster manages multiple submission processes (e.g. a Raft cluster) with
// optional pairwise network partitions via in-process TCP switches.
type Cluster struct {
	path    string
	size    int
	logf    func(format string, args ...any)
	nodes   []*nodeProc
	switches map[int]map[int]*Switch // switches[from][to]
}

type nodeProc struct {
	id       int
	port     int
	portHold net.Listener
	dataDir  string
	program  *Program
}

// NewCluster creates a cluster of size n (node ids 1..n).
func NewCluster(path string, size int, logf func(string, ...any)) (*Cluster, error) {
	if size < 1 {
		return nil, fmt.Errorf("cluster size must be >= 1, got %d", size)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	c := &Cluster{
		path:     abs,
		size:     size,
		logf:     logf,
		switches: make(map[int]map[int]*Switch),
	}
	for i := 1; i <= size; i++ {
		port, hold, err := reservePort()
		if err != nil {
			c.Cleanup()
			return nil, err
		}
		dataDir, err := os.MkdirTemp("", "open-crafters-raft-*")
		if err != nil {
			c.Cleanup()
			return nil, err
		}
		c.nodes = append(c.nodes, &nodeProc{id: i, port: port, portHold: hold, dataDir: dataDir})
	}
	return c, nil
}

func (c *Cluster) nodeByID(id int) (*nodeProc, error) {
	if id < 1 || id > c.size {
		return nil, fmt.Errorf("unknown node id %d (cluster size %d)", id, c.size)
	}
	return c.nodes[id-1], nil
}

// Addr returns the client address for node id.
func (c *Cluster) Addr(id int) (string, error) {
	n, err := c.nodeByID(id)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("127.0.0.1:%d", n.port), nil
}

// DataDir returns the data directory for node id.
func (c *Cluster) DataDir(id int) (string, error) {
	n, err := c.nodeByID(id)
	if err != nil {
		return "", err
	}
	return n.dataDir, nil
}

// Dial opens a client connection to node id.
func (c *Cluster) Dial(id int) (*Client, error) {
	addr, err := c.Addr(id)
	if err != nil {
		return nil, err
	}
	return Dial(addr)
}

// Size returns the number of nodes in the cluster.
func (c *Cluster) Size() int { return c.size }

func (c *Cluster) peersFor(id int) (string, error) {
	var parts []string
	for i := 1; i <= c.size; i++ {
		var addr string
		if i == id {
			n := c.nodes[i-1]
			addr = fmt.Sprintf("127.0.0.1:%d", n.port)
		} else {
			sw, err := c.switchFor(id, i)
			if err != nil {
				return "", err
			}
			addr = sw.Addr()
		}
		parts = append(parts, fmt.Sprintf("%d=%s", i, addr))
	}
	sort.Strings(parts)
	return strings.Join(parts, ","), nil
}

func (c *Cluster) switchFor(from, to int) (*Switch, error) {
	if from == to {
		return nil, fmt.Errorf("switch from %d to itself", from)
	}
	if c.switches[from] == nil {
		c.switches[from] = make(map[int]*Switch)
	}
	if c.switches[from][to] != nil {
		return c.switches[from][to], nil
	}
	target, err := c.Addr(to)
	if err != nil {
		return nil, err
	}
	sw, err := newSwitch(target)
	if err != nil {
		return nil, err
	}
	c.switches[from][to] = sw
	return sw, nil
}

// StartAll launches every node process.
func (c *Cluster) StartAll() error {
	for _, n := range c.nodes {
		if err := c.startNode(n); err != nil {
			return fmt.Errorf("starting node %d: %w", n.id, err)
		}
	}
	return nil
}

func (c *Cluster) startNode(n *nodeProc) error {
	if n.program != nil && n.program.cmd != nil {
		return fmt.Errorf("node %d already running", n.id)
	}
	releasePortHold(&n.portHold)
	peers, err := c.peersFor(n.id)
	if err != nil {
		return err
	}
	p := &Program{
		path:    c.path,
		port:    n.port,
		dataDir: n.dataDir,
		logf:    c.logf,
	}
	args := []string{
		"--node-id", strconv.Itoa(n.id),
		"--peers", peers,
	}
	if err := p.StartWithArgs(args); err != nil {
		return err
	}
	n.program = p
	return nil
}

// KillNode SIGKILLs a node process.
func (c *Cluster) KillNode(id int) error {
	n, err := c.nodeByID(id)
	if err != nil {
		return err
	}
	if n.program == nil {
		return fmt.Errorf("node %d is not running", id)
	}
	n.program.Kill()
	return nil
}

// RestartNode restarts a node with the same data dir and port.
func (c *Cluster) RestartNode(id int) error {
	n, err := c.nodeByID(id)
	if err != nil {
		return err
	}
	if n.program != nil {
		n.program.Kill()
	}
	n.program = nil
	return c.startNode(n)
}

// Partition blocks traffic between nodes a and b in both directions.
func (c *Cluster) Partition(a, b int) error {
	if a == b {
		return fmt.Errorf("cannot partition node %d from itself", a)
	}
	swAB, err := c.switchFor(a, b)
	if err != nil {
		return err
	}
	swBA, err := c.switchFor(b, a)
	if err != nil {
		return err
	}
	swAB.SetBlocked(true)
	swBA.SetBlocked(true)
	return nil
}

// Heal removes all partitions.
func (c *Cluster) Heal() {
	for _, m := range c.switches {
		for _, sw := range m {
			sw.SetBlocked(false)
		}
	}
}

// IsRunning reports whether node id's process is alive.
func (c *Cluster) IsRunning(id int) (bool, error) {
	n, err := c.nodeByID(id)
	if err != nil {
		return false, err
	}
	return n.program != nil && n.program.cmd != nil, nil
}

// Cleanup stops all nodes, switches, and removes data directories.
func (c *Cluster) Cleanup() {
	for _, n := range c.nodes {
		if n.program != nil {
			n.program.Kill()
		}
		releasePortHold(&n.portHold)
		os.RemoveAll(n.dataDir)
	}
	for _, m := range c.switches {
		for _, sw := range m {
			sw.Close()
		}
	}
}
