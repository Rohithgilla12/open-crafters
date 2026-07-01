package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Rohithgilla12/open-crafters/internal/progress"
)

func cmdProgress(args []string) {
	if len(args) < 1 {
		progressUsage(os.Stderr)
		os.Exit(1)
	}
	switch args[0] {
	case "export":
		cmdProgressExport(args[1:])
	case "import":
		cmdProgressImport(args[1:])
	case "show":
		cmdProgressShow(args[1:])
	default:
		die("progress: unknown subcommand %q (try: export, import, show)", args[0])
	}
}

func progressUsage(w *os.File) {
	fmt.Fprint(w, `crafters progress — sync progress.json with the learn app

USAGE
  crafters progress export [dir] [--all] [--out file]
  crafters progress import <file> [dir]
  crafters progress show [dir]

export writes canonical JSON (version 1) to stdout or --out.
  [dir]     solution directory (default: .)
  --all     merge progress from every solution under the current directory

import merges a progress file into a solution's .open-crafters/progress.json.

show prints a human-readable checklist for one solution directory.

EXAMPLES
  cd my-wal && crafters progress export > wal-progress.json
  crafters progress import wal-progress.json my-wal
  crafters progress export --all --out all-progress.json
`)
}

func cmdProgressExport(args []string) {
	fs := flag.NewFlagSet("progress export", flag.ExitOnError)
	all := fs.Bool("all", false, "merge all solutions under cwd")
	out := fs.String("out", "", "write to file instead of stdout")
	fs.Parse(args)

	var merged progress.File
	if *all {
		cwd, err := os.Getwd()
		if err != nil {
			die("%v", err)
		}
		n, err := mergeAllUnder(cwd, &merged)
		if err != nil {
			die("%v", err)
		}
		if n == 0 {
			die("no .open-crafters/progress.json files found under %q", cwd)
		}
	} else {
		dir := "."
		if fs.NArg() > 0 {
			dir = fs.Arg(0)
		}
		prog, err := loadSolutionProgress(dir)
		if err != nil {
			die("%v", err)
		}
		merged = *prog
	}

	data, err := progress.MarshalJSON(&merged)
	if err != nil {
		die("%v", err)
	}
	if *out != "" {
		if err := os.WriteFile(*out, append(data, '\n'), 0o644); err != nil {
			die("writing %s: %v", *out, err)
		}
		fmt.Printf("wrote %s\n", *out)
		return
	}
	os.Stdout.Write(data)
	if len(data) == 0 || data[len(data)-1] != '\n' {
		fmt.Println()
	}
}

func cmdProgressImport(args []string) {
	if len(args) < 1 {
		die("usage: crafters progress import <file> [dir]")
	}
	file := args[0]
	dir := "."
	if len(args) >= 2 {
		dir = args[1]
	}
	data, err := os.ReadFile(file)
	if err != nil {
		die("reading %s: %v", file, err)
	}
	var incoming progress.File
	if err := json.Unmarshal(data, &incoming); err != nil {
		die("parsing %s: %v", file, err)
	}
	programPath, err := filepath.Abs(filepath.Join(dir, "your_program.sh"))
	if err != nil {
		die("%v", err)
	}
	if _, err := os.Stat(programPath); err != nil {
		die("%q isn't a solution directory (no your_program.sh)", dir)
	}
	progressPath := progress.PathFor(programPath)
	existing, err := progress.Load(progressPath)
	if err != nil {
		die("reading %s: %v", progressPath, err)
	}
	progress.Merge(existing, &incoming)
	if err := progress.Save(progressPath, existing); err != nil {
		die("saving %s: %v", progressPath, err)
	}
	fmt.Printf("merged %s into %s\n", file, progressPath)
}

func cmdProgressShow(args []string) {
	dir := "."
	if len(args) > 0 {
		dir = args[0]
	}
	_, ch := solutionChallenge(dir)
	prog, err := loadSolutionProgress(dir)
	if err != nil {
		die("%v", err)
	}
	printStatus(ch, prog, dir)
}

func loadSolutionProgress(dir string) (*progress.File, error) {
	programPath, err := filepath.Abs(filepath.Join(dir, "your_program.sh"))
	if err != nil {
		return nil, err
	}
	return progress.Load(progress.PathFor(programPath))
}

func mergeAllUnder(root string, merged *progress.File) (int, error) {
	count := 0
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Base(path) != "progress.json" {
			return nil
		}
		if filepath.Base(filepath.Dir(path)) != ".open-crafters" {
			return nil
		}
		prog, err := progress.Load(path)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		progress.Merge(merged, prog)
		count++
		return nil
	})
	return count, err
}
