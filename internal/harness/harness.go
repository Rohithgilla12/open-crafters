// Package harness contains the challenge-agnostic core of the open-crafters
// tester: process lifecycle management, the wire-protocol client, and the
// stage runner. Challenge-specific test logic lives under
// internal/challenges/<challenge>.
package harness

import (
	"fmt"
	"time"
)

// Stage is one gradeable step of a challenge.
type Stage struct {
	Slug         string
	Name         string
	Difficulty   string // "easy" | "medium" | "hard"
	Instructions string // repo-relative path to the stage's instructions
	Test         func(*Context) error
	TestCluster  func(*ClusterContext) error // used when Challenge.ClusterSize > 0
}

// Challenge is a named, ordered list of stages.
type Challenge struct {
	Slug        string
	Name        string
	Stages      []Stage
	ClusterSize int // 0 = single process; >0 = multi-node cluster (e.g. Raft)
}

// Context is handed to each single-process stage test.
type Context struct {
	program *Program
	Logf    func(format string, args ...any)
}

// ClusterContext is handed to multi-node stage tests.
type ClusterContext struct {
	cluster *Cluster
	Logf    func(format string, args ...any)
}

// Dial opens a client connection to node id (1-based).
func (c *ClusterContext) Dial(nodeID int) (*Client, error) { return c.cluster.Dial(nodeID) }

// Addr returns the client address for node id.
func (c *ClusterContext) Addr(nodeID int) (string, error) { return c.cluster.Addr(nodeID) }

// DataDir returns the data directory for node id.
func (c *ClusterContext) DataDir(nodeID int) (string, error) { return c.cluster.DataDir(nodeID) }

// KillNode SIGKILLs node id.
func (c *ClusterContext) KillNode(nodeID int) error { return c.cluster.KillNode(nodeID) }

// RestartNode restarts node id with the same data dir.
func (c *ClusterContext) RestartNode(nodeID int) error { return c.cluster.RestartNode(nodeID) }

// Partition blocks traffic between nodes a and b (both directions).
func (c *ClusterContext) Partition(a, b int) error { return c.cluster.Partition(a, b) }

// Heal removes all network partitions.
func (c *ClusterContext) Heal() { c.cluster.Heal() }

func (c *ClusterContext) IsRunning(nodeID int) (bool, error) {
	return c.cluster.IsRunning(nodeID)
}

// ClusterSize returns the number of nodes in the cluster.
func (c *ClusterContext) ClusterSize() int { return c.cluster.Size() }

// Addr returns the address the user's server listens on.
func (c *Context) Addr() string { return c.program.Addr() }

// DataDir returns the --data-dir passed to the user's program. Stage tests
// may inspect and manipulate its contents (e.g. to craft or corrupt durable
// state) while the program is stopped.
func (c *Context) DataDir() string { return c.program.DataDir() }

// Dial opens a new client connection to the user's server.
func (c *Context) Dial() (*Client, error) { return Dial(c.program.Addr()) }

// KillProgram SIGKILLs the user's process, simulating a crash.
func (c *Context) KillProgram() { c.program.Kill() }

// StartProgram starts (or restarts) the user's process with the same
// port and data dir, waiting for it to accept connections.
func (c *Context) StartProgram() error { return c.program.Start() }

// RunOptions configures a Run.
type RunOptions struct {
	// TargetSlug: run stages up to and including this slug (all stages when
	// empty), stopping at the first failure.
	TargetSlug  string
	ProgramPath string
	Logf        func(format string, args ...any)
	// OnStagePass, if set, is called after each stage passes.
	OnStagePass func(Stage)
	// OnStageStart/OnStageEnd, if set, bracket each stage attempt so a caller
	// (e.g. the TUI) can render live progress without parsing Logf output.
	// OnStageEnd's error is nil on success.
	OnStageStart func(Stage)
	OnStageEnd   func(Stage, error, time.Duration)
}

// Run executes a challenge's stages per opts.
func Run(ch Challenge, opts RunOptions) error {
	stages := ch.Stages
	if opts.TargetSlug != "" {
		idx := -1
		for i, s := range stages {
			if s.Slug == opts.TargetSlug {
				idx = i
				break
			}
		}
		if idx == -1 {
			return fmt.Errorf("unknown stage %q in challenge %q", opts.TargetSlug, ch.Slug)
		}
		stages = stages[:idx+1]
	}
	logf := opts.Logf

	for i, stage := range stages {
		logf("")
		logf("\x1b[1m[stage %d/%d] %s — %s\x1b[0m", i+1, len(ch.Stages), stage.Slug, stage.Name)
		if opts.OnStageStart != nil {
			opts.OnStageStart(stage)
		}
		start := time.Now()
		err := runStage(stage, opts.ProgramPath, ch.ClusterSize, logf)
		elapsed := time.Since(start)
		if opts.OnStageEnd != nil {
			opts.OnStageEnd(stage, err, elapsed)
		}
		if err != nil {
			logf("\x1b[31m✗ stage %q failed: %v\x1b[0m", stage.Slug, err)
			return fmt.Errorf("stage %q failed", stage.Slug)
		}
		logf("\x1b[32m✓ %s passed (%.2fs)\x1b[0m", stage.Slug, elapsed.Seconds())
		if opts.OnStagePass != nil {
			opts.OnStagePass(stage)
		}
	}
	logf("")
	logf("\x1b[32;1mAll %d stage(s) passed.\x1b[0m", len(stages))
	return nil
}

func runStage(stage Stage, programPath string, clusterSize int, logf func(string, ...any)) error {
	if clusterSize > 0 {
		if stage.TestCluster == nil {
			return fmt.Errorf("stage %q requires TestCluster (cluster size %d)", stage.Slug, clusterSize)
		}
		cluster, err := NewCluster(programPath, clusterSize, logf)
		if err != nil {
			return err
		}
		defer cluster.Cleanup()
		if err := cluster.StartAll(); err != nil {
			return err
		}
		return stage.TestCluster(&ClusterContext{cluster: cluster, Logf: logf})
	}
	if stage.Test == nil {
		return fmt.Errorf("stage %q requires Test", stage.Slug)
	}
	program, err := NewProgram(programPath, logf)
	if err != nil {
		return err
	}
	defer program.Cleanup()
	if err := program.Start(); err != nil {
		return err
	}
	return stage.Test(&Context{program: program, Logf: logf})
}
