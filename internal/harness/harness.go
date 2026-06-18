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
	Instructions string // repo-relative path to the stage's instructions
	Test         func(*Context) error
}

// Challenge is a named, ordered list of stages.
type Challenge struct {
	Slug   string
	Name   string
	Stages []Stage
}

// Context is handed to each stage test. Each stage gets a fresh program
// instance (fresh data dir); within a stage the test may kill and restart
// the program to verify durability.
type Context struct {
	program *Program
	Logf    func(format string, args ...any)
}

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
		err := runStage(stage, opts.ProgramPath, logf)
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

func runStage(stage Stage, programPath string, logf func(string, ...any)) error {
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
