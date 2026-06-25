package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func watchAndSubmit(dir, url, token, challenge string, all bool, stage string) {
	fmt.Printf("Watching %s for changes — saving a file will trigger crafters submit (Ctrl+C to stop)\n", dir)
	var last string
	var busy bool
	for {
		hash, err := solutionFingerprint(dir)
		if err != nil {
			die("%v", err)
		}
		if last != "" && hash != last && !busy {
			busy = true
			fmt.Printf("\n\x1b[1m→ change detected, submitting…\x1b[0m\n")
			code := runSubmitOnce(dir, url, token, challenge, all, stage)
			busy = false
			if code != 0 {
				fmt.Printf("\x1b[33m→ fix and save again to re-submit\x1b[0m\n")
			}
		}
		last = hash
		time.Sleep(1 * time.Second)
	}
}

func solutionFingerprint(dir string) (string, error) {
	h := sha256.New()
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if shouldSkipZipEntry(rel, d) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		_, _ = h.Write([]byte(rel))
		_, _ = h.Write([]byte(fmt.Sprint(info.Size(), info.ModTime().UnixNano())))
		return nil
	})
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func runSubmitOnce(dir, url, token, challenge string, all bool, stage string) int {
	zipBytes, err := zipSolutionDir(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "crafters: %v\n", err)
		return 1
	}
	job, err := postGrade(url, token, challenge, stage, all, zipBytes)
	if err != nil {
		fmt.Fprintf(os.Stderr, "crafters: %v\n", err)
		return 1
	}
	fmt.Printf("Submitted job %s for %q\n", job.ID, challenge)
	final, err := pollJob(url, token, job.ID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "crafters: %v\n", err)
		return 1
	}
	fmt.Println()
	fmt.Print(final.Log)
	fmt.Println()
	switch final.Status {
	case "passed":
		fmt.Printf("\x1b[32;1m✓ Remote grading passed (job %s)\x1b[0m\n", final.ID)
		return 0
	case "failed":
		fmt.Printf("\x1b[31m✗ Remote grading failed (exit %d, job %s)\x1b[0m\n", final.ExitCode, final.ID)
		return 1
	default:
		fmt.Printf("\x1b[31m✗ Remote grading error: %s\x1b[0m\n", final.Error)
		return 1
	}
}
