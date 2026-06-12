// Command tester runs the open-crafters stage tests for a challenge against
// a user's submission.
//
// Usage:
//
//	tester --challenge build-your-own-temporal --program ./your_program.sh [--stage <slug>]
//
// Runs every stage up to and including --stage (all stages when omitted),
// stopping at the first failure. Exits 0 on success, 1 on failure.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/open-crafters/open-crafters/tester/internal/challenges/temporal"
	"github.com/open-crafters/open-crafters/tester/internal/challenges/wal"
	"github.com/open-crafters/open-crafters/tester/internal/harness"
)

var challenges = map[string]harness.Challenge{
	"build-your-own-temporal": temporal.Challenge(),
	"build-your-own-wal":      wal.Challenge(),
}

func main() {
	challengeSlug := flag.String("challenge", "build-your-own-temporal", "challenge slug")
	program := flag.String("program", "./your_program.sh", "path to the submission's entry point")
	stage := flag.String("stage", "", "run stages up to and including this slug (default: all)")
	list := flag.Bool("list", false, "list challenges and stages, then exit")
	flag.Parse()

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
	if _, err := os.Stat(*program); err != nil {
		fmt.Fprintf(os.Stderr, "program %q not found: %v\n", *program, err)
		os.Exit(1)
	}

	logf := func(format string, args ...any) { fmt.Printf(format+"\n", args...) }
	logf("open-crafters tester — %s", ch.Name)
	if err := harness.Run(ch, *stage, *program, logf); err != nil {
		os.Exit(1)
	}
}
