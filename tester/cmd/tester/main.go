// Command tester runs the open-crafters stage tests for a challenge against
// a user's submission, tracking progress in
// <submission dir>/.open-crafters/progress.json.
//
// Usage:
//
//	tester --challenge build-your-own-temporal --program ./your_program.sh
//
// By default the tester *resumes*: it re-verifies the stages you've already
// passed and attempts the next one. Other modes:
//
//	--stage <slug>   run stages up to and including <slug>
//	--all            run every stage
//	--status         print your progress checklist and exit
//	--list           list challenges and stages and exit
//
// Exits 0 on success, 1 on failure.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/open-crafters/open-crafters/tester/internal/challenges/temporal"
	"github.com/open-crafters/open-crafters/tester/internal/challenges/wal"
	"github.com/open-crafters/open-crafters/tester/internal/harness"
	"github.com/open-crafters/open-crafters/tester/internal/progress"
)

var challenges = map[string]harness.Challenge{
	"build-your-own-temporal": temporal.Challenge(),
	"build-your-own-wal":      wal.Challenge(),
}

func main() {
	challengeSlug := flag.String("challenge", "build-your-own-temporal", "challenge slug")
	program := flag.String("program", "./your_program.sh", "path to the submission's entry point")
	stage := flag.String("stage", "", "run stages up to and including this slug")
	all := flag.Bool("all", false, "run every stage")
	status := flag.Bool("status", false, "print the progress checklist and exit")
	list := flag.Bool("list", false, "list challenges and stages, then exit")
	flag.Parse()

	logf := func(format string, args ...any) { fmt.Printf(format+"\n", args...) }

	if *list {
		for slug, ch := range challenges {
			fmt.Printf("%s — %s\n", slug, ch.Name)
			for i, s := range ch.Stages {
				fmt.Printf("  %2d. %-20s %s\n", i+1, s.Slug, s.Name)
			}
		}
		return
	}

	ch, ok := challenges[*challengeSlug]
	if !ok {
		fmt.Fprintf(os.Stderr, "unknown challenge %q (try --list)\n", *challengeSlug)
		os.Exit(1)
	}
	programPath, err := filepath.Abs(*program)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	progressPath := progress.PathFor(programPath)
	prog, err := progress.Load(progressPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "reading %s: %v\n", progressPath, err)
		os.Exit(1)
	}

	if *status {
		printStatus(ch, prog)
		return
	}

	if _, err := os.Stat(programPath); err != nil {
		fmt.Fprintf(os.Stderr, "program %q not found: %v\n", *program, err)
		os.Exit(1)
	}

	// Pick the target stage: explicit --stage, --all, or resume (re-verify
	// passed stages, attempt the first unpassed one).
	target := *stage
	if target == "" && !*all {
		if next := firstUnpassed(ch, prog); next != nil {
			target = next.Slug
			logf("Resuming: %d/%d stage(s) passed, attempting %q. (Use --all to run everything.)",
				passedCount(ch, prog), len(ch.Stages), next.Slug)
		} else {
			logf("All %d stages already passed — re-running the full suite to verify.", len(ch.Stages))
		}
	}

	logf("open-crafters tester — %s", ch.Name)
	runErr := harness.Run(ch, harness.RunOptions{
		TargetSlug:  target,
		ProgramPath: programPath,
		Logf:        logf,
		OnStagePass: func(s harness.Stage) {
			prog.MarkPassed(ch.Slug, s.Slug)
			if err := progress.Save(progressPath, prog); err != nil {
				fmt.Fprintf(os.Stderr, "warning: could not save progress to %s: %v\n", progressPath, err)
			}
		},
	})

	logf("")
	if next := firstUnpassed(ch, prog); next == nil {
		logf("\x1b[32;1m🏆 Challenge complete: all %d stages of %q passed.\x1b[0m", len(ch.Stages), ch.Name)
	} else if runErr == nil {
		logf("Next up: \x1b[1m%s — %s\x1b[0m", next.Slug, next.Name)
		if next.Instructions != "" {
			logf("Instructions: %s", next.Instructions)
		}
	} else if next.Instructions != "" {
		logf("Stuck? Re-read the instructions: %s", next.Instructions)
	}
	if runErr != nil {
		os.Exit(1)
	}
}

func firstUnpassed(ch harness.Challenge, prog *progress.File) *harness.Stage {
	for i := range ch.Stages {
		if !prog.HasPassed(ch.Slug, ch.Stages[i].Slug) {
			return &ch.Stages[i]
		}
	}
	return nil
}

func passedCount(ch harness.Challenge, prog *progress.File) int {
	n := 0
	for _, s := range ch.Stages {
		if prog.HasPassed(ch.Slug, s.Slug) {
			n++
		}
	}
	return n
}

func printStatus(ch harness.Challenge, prog *progress.File) {
	fmt.Printf("\x1b[1m%s\x1b[0m — %d/%d stages passed\n\n", ch.Name, passedCount(ch, prog), len(ch.Stages))
	nextMarked := false
	for i, s := range ch.Stages {
		switch {
		case prog.HasPassed(ch.Slug, s.Slug):
			fmt.Printf("  \x1b[32m✓\x1b[0m %2d. %-18s %s\n", i+1, s.Slug, s.Name)
		case !nextMarked:
			fmt.Printf("  \x1b[33m→\x1b[0m %2d. %-18s %s   \x1b[33m← next\x1b[0m\n", i+1, s.Slug, s.Name)
			if s.Instructions != "" {
				fmt.Printf("       instructions: %s\n", s.Instructions)
			}
			nextMarked = true
		default:
			fmt.Printf("    %2d. %-18s %s\n", i+1, s.Slug, s.Name)
		}
	}
	if !nextMarked {
		fmt.Printf("\n\x1b[32;1m🏆 Challenge complete.\x1b[0m\n")
	}
}
