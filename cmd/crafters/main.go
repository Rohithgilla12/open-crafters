// Command crafters is the open-crafters CLI: a single self-contained binary
// that scaffolds solutions (content is embedded), grades them stage by stage,
// and — run with no arguments — opens an interactive dashboard.
//
// Subcommands:
//
//	crafters                      interactive dashboard (TUI)
//	crafters start <ch> [dir] [--lang python|go|typescript]
//	crafters test  [dir] [--all] [--stage <slug>]
//	crafters status [dir]
//	crafters list
//	crafters grade --challenge <slug> --program <path> [--all|--stage <slug>|--status]
//	crafters hint <challenge> [--stage <slug>]
//	crafters walkthrough <challenge> [--stage <slug>]
//
// Progress is tracked in <solution>/.open-crafters/progress.json.
package main

import (
	"flag"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	opencrafters "github.com/Rohithgilla12/open-crafters"
	"github.com/Rohithgilla12/open-crafters/internal/challenges/logstore"
	"github.com/Rohithgilla12/open-crafters/internal/challenges/lsm"
	"github.com/Rohithgilla12/open-crafters/internal/challenges/mvcc"
	"github.com/Rohithgilla12/open-crafters/internal/challenges/queue"
	"github.com/Rohithgilla12/open-crafters/internal/challenges/raft"
	"github.com/Rohithgilla12/open-crafters/internal/challenges/ratelimiter"
	"github.com/Rohithgilla12/open-crafters/internal/challenges/scheduler"
	"github.com/Rohithgilla12/open-crafters/internal/challenges/temporal"
	"github.com/Rohithgilla12/open-crafters/internal/challenges/wal"
	"github.com/Rohithgilla12/open-crafters/internal/challenges/workflowsdk"
	"github.com/Rohithgilla12/open-crafters/internal/harness"
	"github.com/Rohithgilla12/open-crafters/internal/progress"
)

var challenges = map[string]harness.Challenge{
	"build-your-own-temporal":     temporal.Challenge(),
	"build-your-own-wal":          wal.Challenge(),
	"build-your-own-queue":        queue.Challenge(),
	"build-your-own-mvcc":         mvcc.Challenge(),
	"build-your-own-log":          logstore.Challenge(),
	"build-your-own-lsm":          lsm.Challenge(),
	"build-your-own-workflow-sdk": workflowsdk.Challenge(),
	"build-your-own-raft":         raft.Challenge(),
	"build-your-own-scheduler":    scheduler.Challenge(),
	"build-your-own-rate-limiter": ratelimiter.Challenge(),
}

// challengeOrder re-exports the canonical order from the root module.
var challengeOrder = opencrafters.ChallengeOrder

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		os.Exit(runTUI())
	}
	switch args[0] {
	case "start":
		cmdStart(args[1:])
	case "test":
		cmdTest(args[1:])
	case "status":
		cmdStatus(args[1:])
	case "list":
		cmdList()
	case "grade":
		cmdGrade(args[1:])
	case "walkthrough":
		cmdWalkthrough(args[1:])
	case "hint":
		cmdHint(args[1:])
	case "site":
		cmdSite(args[1:])
	case "submit":
		cmdSubmit(args[1:])
	case "update":
		cmdUpdate()
	case "version", "--version", "-v":
		cmdVersion()
	case "-h", "--help", "help":
		usage(os.Stdout)
	default:
		fmt.Fprintf(os.Stderr, "crafters: unknown command %q\n\n", args[0])
		usage(os.Stderr)
		os.Exit(1)
	}
}

func usage(w *os.File) {
	fmt.Fprint(w, `crafters — build-your-own-X challenges for serious infrastructure

USAGE
  crafters                              open the interactive dashboard
  crafters start <challenge> [dir] [--lang python|go|typescript]
  crafters test  [dir] [--all] [--stage <slug>]
  crafters status [dir]
  crafters list
  crafters grade --challenge <slug> --program <path> [--all|--stage <slug>|--status]
  crafters hint <challenge> [--stage <slug>]            spoiler-free nudge for a stage
  crafters walkthrough <challenge> [--stage <slug>]     how the reference solves it
  crafters submit [dir] [--url <url>] [--token <secret>] [--all|--stage <slug>|--watch]
  crafters site [--out dir]             generate the static showcase site
  crafters update                       self-update to the latest release
  crafters version                      print the version

EXAMPLES
  crafters                       # browse challenges and grade interactively
  crafters start wal             # 'wal' fuzzy-matches build-your-own-wal
  crafters start queue --lang go
  cd my-wal && crafters test     # re-grade, resuming where you left off

<challenge> accepts a fuzzy name (wal, queue, temporal) or a full slug.
`)
}

func die(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "crafters: "+format+"\n", args...)
	os.Exit(1)
}

// resolveChallenge maps a fuzzy name or full slug to a challenge.
func resolveChallenge(query string) (string, harness.Challenge) {
	if ch, ok := challenges[query]; ok {
		return query, ch
	}
	if ch, ok := challenges["build-your-own-"+query]; ok {
		return "build-your-own-" + query, ch
	}
	var matches []string
	for slug := range challenges {
		if strings.Contains(slug, query) {
			matches = append(matches, slug)
		}
	}
	sort.Strings(matches)
	switch len(matches) {
	case 1:
		return matches[0], challenges[matches[0]]
	case 0:
		die("no challenge matches %q. Try: crafters list", query)
	default:
		die("%q is ambiguous — matches: %s", query, strings.Join(matches, ", "))
	}
	return "", harness.Challenge{}
}

// solutionChallenge reads the challenge slug recorded for a solution directory.
func solutionChallenge(dir string) (string, harness.Challenge) {
	program := filepath.Join(dir, "your_program.sh")
	if _, err := os.Stat(program); err != nil {
		die("%q isn't a solution directory (no your_program.sh). Run 'crafters start <challenge>' first, then cd into it.", dir)
	}
	data, err := os.ReadFile(filepath.Join(dir, ".open-crafters", "challenge"))
	if err != nil {
		die("can't tell which challenge %q is for. Re-create it with 'crafters start <challenge>'.", dir)
	}
	slug := strings.TrimSpace(string(data))
	ch, ok := challenges[slug]
	if !ok {
		die("solution %q references unknown challenge %q", dir, slug)
	}
	return slug, ch
}

func cmdStart(args []string) {
	lang := "python"
	var pos []string
	for i := 0; i < len(args); i++ {
		switch a := args[i]; {
		case a == "--lang":
			i++
			if i < len(args) {
				lang = args[i]
			}
		case strings.HasPrefix(a, "--lang="):
			lang = strings.TrimPrefix(a, "--lang=")
		default:
			pos = append(pos, a)
		}
	}
	if len(pos) < 1 {
		die("usage: crafters start <challenge> [dir] [--lang python|go|typescript]")
	}
	slug, ch := resolveChallenge(pos[0])
	dest := "my-" + strings.TrimPrefix(slug, "build-your-own-")
	if len(pos) >= 2 {
		dest = pos[1]
	}
	if err := opencrafters.Scaffold(slug, lang, dest); err != nil {
		die("%v", err)
	}
	fmt.Printf("\x1b[32m✓\x1b[0m created ./%s from the %s starter for %q\n", dest, lang, slug)
	fmt.Printf("  spec: %s\n\n", filepath.Join(dest, "PROTOCOL.md"))
	code := runGrade(ch, filepath.Join(dest, "your_program.sh"), dest, false, "", false)
	fmt.Printf("\nNext: cd %s, edit your code, then run 'crafters test' to grade again.\n", dest)
	os.Exit(code)
}

func cmdTest(args []string) {
	all, stage, dir := false, "", "."
	for i := 0; i < len(args); i++ {
		switch a := args[i]; {
		case a == "--all":
			all = true
		case a == "--stage":
			i++
			if i < len(args) {
				stage = args[i]
			}
		case strings.HasPrefix(a, "--stage="):
			stage = strings.TrimPrefix(a, "--stage=")
		default:
			dir = a
		}
	}
	_, ch := solutionChallenge(dir)
	os.Exit(runGrade(ch, filepath.Join(dir, "your_program.sh"), dir, all, stage, false))
}

func cmdStatus(args []string) {
	dir := "."
	if len(args) > 0 {
		dir = args[0]
	}
	_, ch := solutionChallenge(dir)
	os.Exit(runGrade(ch, filepath.Join(dir, "your_program.sh"), dir, false, "", true))
}

func cmdList() {
	for _, slug := range orderedSlugs() {
		ch := challenges[slug]
		fmt.Printf("%s — %s\n", slug, ch.Name)
		for i, s := range ch.Stages {
			fmt.Printf("  %2d. %-20s %-34s %s\n", i+1, s.Slug, s.Name, difficultyTag(s.Difficulty))
		}
	}
}

// difficultyTag renders a colored difficulty label for terminal output.
func difficultyTag(d string) string {
	switch d {
	case "easy":
		return "\x1b[32measy\x1b[0m"
	case "medium":
		return "\x1b[33mmedium\x1b[0m"
	case "hard":
		return "\x1b[31mhard\x1b[0m"
	}
	return d
}

func cmdGrade(args []string) {
	fs := flag.NewFlagSet("grade", flag.ExitOnError)
	challenge := fs.String("challenge", "", "challenge slug (or fuzzy name)")
	program := fs.String("program", "./your_program.sh", "path to the submission entry point")
	stage := fs.String("stage", "", "run up to and including this slug")
	all := fs.Bool("all", false, "run every stage")
	status := fs.Bool("status", false, "print the progress checklist and exit")
	fs.Parse(args)
	if *challenge == "" {
		die("grade needs --challenge")
	}
	_, ch := resolveChallenge(*challenge)
	programPath, err := filepath.Abs(*program)
	if err != nil {
		die("%v", err)
	}
	if !*status {
		if _, err := os.Stat(programPath); err != nil {
			die("program %q not found: %v", *program, err)
		}
	}
	os.Exit(runGrade(ch, programPath, "", *all, *stage, *status))
}

// parseChallengeStageArgs reads `<challenge> [--stage <slug>] [--all]` where
// the challenge is a leading positional (Go's flag package stops at the first
// non-flag, so these are parsed by hand, like cmdStart).
func parseChallengeStageArgs(args []string) (query, stage string, all bool) {
	for i := 0; i < len(args); i++ {
		switch a := args[i]; {
		case a == "--stage":
			i++
			if i < len(args) {
				stage = args[i]
			}
		case strings.HasPrefix(a, "--stage="):
			stage = strings.TrimPrefix(a, "--stage=")
		case a == "--all":
			all = true
		default:
			if query == "" {
				query = a
			}
		}
	}
	return
}

func shortName(slug string) string { return strings.TrimPrefix(slug, "build-your-own-") }

// cmdWalkthrough prints a challenge's walkthrough — how the reference solves
// it, design-level. With --stage, only that stage's section.
func cmdWalkthrough(args []string) {
	query, stage, _ := parseChallengeStageArgs(args)
	if query == "" {
		die("usage: crafters walkthrough <challenge> [--stage <slug>]")
	}
	slug, ch := resolveChallenge(query)
	if stage != "" {
		section, ok := opencrafters.WalkthroughSection(slug, stage)
		if !ok {
			die("no walkthrough for stage %q in %q (try: crafters list)", stage, ch.Name)
		}
		fmt.Println(section)
		return
	}
	doc, ok := opencrafters.Walkthrough(slug)
	if !ok {
		die("no walkthrough for %q yet", ch.Name)
	}
	fmt.Print(doc)
}

// cmdHint prints a spoiler-free nudge for a stage — the next unpassed one when
// run inside a solution directory, otherwise the first stage (or --stage).
func cmdHint(args []string) {
	query, stage, _ := parseChallengeStageArgs(args)
	if query == "" {
		die("usage: crafters hint <challenge> [--stage <slug>]")
	}
	slug, ch := resolveChallenge(query)
	if !opencrafters.HasWalkthrough(slug) {
		die("no hints for %q yet", ch.Name)
	}
	if stage == "" {
		stage = nextStageSlugForCwd(slug, ch)
	}
	if stage == "" && len(ch.Stages) > 0 {
		stage = ch.Stages[0].Slug
	}
	hint, ok := opencrafters.StageHint(slug, stage)
	if !ok {
		die("no hint for stage %q in %q", stage, ch.Name)
	}
	name := stage
	for _, s := range ch.Stages {
		if s.Slug == stage {
			name = s.Name
			break
		}
	}
	fmt.Printf("\x1b[1m💡 %s — %s\x1b[0m\n%s\n", stage, name, hint)
	fmt.Printf("\nStuck deeper? After you pass it: crafters walkthrough %s --stage %s\n", shortName(slug), stage)
}

// nextStageSlugForCwd returns the next unpassed stage for `ch` if the current
// directory is a solution for it, else "".
func nextStageSlugForCwd(slug string, ch harness.Challenge) string {
	data, err := os.ReadFile(filepath.Join(".open-crafters", "challenge"))
	if err != nil || strings.TrimSpace(string(data)) != slug {
		return ""
	}
	programPath, err := filepath.Abs("your_program.sh")
	if err != nil {
		return ""
	}
	prog, err := progress.Load(progress.PathFor(programPath))
	if err != nil {
		return ""
	}
	if next := firstUnpassed(ch, prog); next != nil {
		return next.Slug
	}
	return ""
}

// stageBefore returns the slug of the stage immediately before slug, or "".
func stageBefore(ch harness.Challenge, slug string) string {
	prev := ""
	for _, s := range ch.Stages {
		if s.Slug == slug {
			return prev
		}
		prev = s.Slug
	}
	return ""
}

// runGrade loads progress, runs the grader (or just prints status), saves
// progress, and prints the next-up pointer. solutionDir is "" for `grade`;
// when set, instruction paths resolve to the spec copy inside the solution.
func runGrade(ch harness.Challenge, programPath, solutionDir string, all bool, stage string, statusOnly bool) int {
	progressPath := progress.PathFor(programPath)
	prog, err := progress.Load(progressPath)
	if err != nil {
		die("reading %s: %v", progressPath, err)
	}

	if statusOnly {
		printStatus(ch, prog, solutionDir)
		return 0
	}

	logf := func(format string, args ...any) { fmt.Printf(format+"\n", args...) }

	target := stage
	if target == "" && !all {
		if next := firstUnpassed(ch, prog); next != nil {
			target = next.Slug
			logf("Resuming: %d/%d stage(s) passed, attempting %q. (Use --all to run everything.)",
				passedCount(ch, prog), len(ch.Stages), next.Slug)
		} else {
			logf("All %d stages already passed — re-running the full suite to verify.", len(ch.Stages))
		}
	}

	logf("open-crafters — %s", ch.Name)
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
	hasWT := opencrafters.HasWalkthrough(ch.Slug)
	short := shortName(ch.Slug)
	if next := firstUnpassed(ch, prog); next == nil {
		logf("\x1b[32;1m🏆 Challenge complete: all %d stages of %q passed.\x1b[0m", len(ch.Stages), ch.Name)
		if hasWT {
			logf("📖 See how each stage works: crafters walkthrough %s", short)
		}
	} else if runErr == nil {
		logf("Next up: \x1b[1m%s — %s\x1b[0m", next.Slug, next.Name)
		logf("Instructions: %s", instructionPath(next, solutionDir))
		if just := stageBefore(ch, next.Slug); hasWT && just != "" {
			if _, ok := opencrafters.WalkthroughSection(ch.Slug, just); ok {
				logf("📖 How %q works: crafters walkthrough %s --stage %s", just, short, just)
			}
		}
	} else {
		logf("Stuck? Re-read the instructions: %s", instructionPath(next, solutionDir))
		if hint, ok := opencrafters.StageHint(ch.Slug, next.Slug); ok {
			logf("\x1b[33m💡 Hint:\x1b[0m %s", hint)
		}
	}
	if runErr != nil {
		return 1
	}
	return 0
}

// instructionPath resolves a stage's instructions to the local spec copy in a
// scaffolded solution, falling back to the repo-relative path for `grade`.
func instructionPath(s *harness.Stage, solutionDir string) string {
	if solutionDir == "" {
		return s.Instructions
	}
	return filepath.Join(solutionDir, "stages", path.Base(s.Instructions))
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

func printStatus(ch harness.Challenge, prog *progress.File, solutionDir string) {
	fmt.Printf("\x1b[1m%s\x1b[0m — %d/%d stages passed\n\n", ch.Name, passedCount(ch, prog), len(ch.Stages))
	nextMarked := false
	for i, s := range ch.Stages {
		switch {
		case prog.HasPassed(ch.Slug, s.Slug):
			fmt.Printf("  \x1b[32m✓\x1b[0m %2d. %-18s %-34s %s\n", i+1, s.Slug, s.Name, difficultyTag(s.Difficulty))
		case !nextMarked:
			fmt.Printf("  \x1b[33m→\x1b[0m %2d. %-18s %-34s %s   \x1b[33m← next\x1b[0m\n", i+1, s.Slug, s.Name, difficultyTag(s.Difficulty))
			fmt.Printf("       instructions: %s\n", instructionPath(&ch.Stages[i], solutionDir))
			nextMarked = true
		default:
			fmt.Printf("    %2d. %-18s %-34s %s\n", i+1, s.Slug, s.Name, difficultyTag(s.Difficulty))
		}
	}
	if !nextMarked {
		fmt.Printf("\n\x1b[32;1m🏆 Challenge complete.\x1b[0m\n")
	}
}

// orderedSlugs returns challenge slugs in canonical order, appending any not
// listed in challengeOrder (so a new challenge still shows up).
func orderedSlugs() []string {
	seen := map[string]bool{}
	var out []string
	for _, slug := range challengeOrder {
		if _, ok := challenges[slug]; ok {
			out = append(out, slug)
			seen[slug] = true
		}
	}
	var rest []string
	for slug := range challenges {
		if !seen[slug] {
			rest = append(rest, slug)
		}
	}
	sort.Strings(rest)
	return append(out, rest...)
}
