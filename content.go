// Package opencrafters embeds the challenge content (starter templates and
// stage instructions) into the crafters binary, so a single self-contained
// executable can scaffold a solution with no repository checkout.
package opencrafters

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Dot-prefixed files (e.g. a stray .open-crafters/) are excluded by go:embed
// automatically, so scaffolds never inherit leftover progress.
//
//go:embed challenges
var content embed.FS

func challengeDir(slug string) string { return "challenges/" + slug }

// StarterLangs returns the languages with a starter template for a challenge.
func StarterLangs(slug string) []string {
	entries, err := content.ReadDir(challengeDir(slug) + "/starters")
	if err != nil {
		return nil
	}
	var langs []string
	for _, e := range entries {
		if e.IsDir() {
			langs = append(langs, e.Name())
		}
	}
	sort.Strings(langs)
	return langs
}

// Scaffold writes a fresh solution for (slug, lang) into destDir: the starter
// files at the top level, plus the challenge's PROTOCOL.md and stages/ so the
// spec travels with the solution. destDir must not already exist.
func Scaffold(slug, lang, destDir string) error {
	starterRoot := challengeDir(slug) + "/starters/" + lang
	if _, err := content.ReadDir(starterRoot); err != nil {
		langs := StarterLangs(slug)
		return fmt.Errorf("no %q starter for %s (available: %s)", lang, slug, strings.Join(langs, ", "))
	}
	if _, err := os.Stat(destDir); err == nil {
		return fmt.Errorf("%q already exists — pass a different directory name", destDir)
	}

	if err := copyTree(starterRoot, destDir); err != nil {
		return err
	}
	if err := copyFile(challengeDir(slug)+"/PROTOCOL.md", filepath.Join(destDir, "PROTOCOL.md")); err != nil {
		return err
	}
	if err := copyTree(challengeDir(slug)+"/stages", filepath.Join(destDir, "stages")); err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Join(destDir, ".open-crafters"), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(destDir, ".open-crafters", "challenge"), []byte(slug+"\n"), 0o644); err != nil {
		return err
	}
	// Make the entry point runnable; ignore on platforms without exec bits.
	_ = os.Chmod(filepath.Join(destDir, "your_program.sh"), 0o755)
	return nil
}

func copyTree(srcRoot, dstRoot string) error {
	return fs.WalkDir(content, srcRoot, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel := strings.TrimPrefix(strings.TrimPrefix(p, srcRoot), "/")
		target := filepath.Join(dstRoot, filepath.FromSlash(rel))
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		return copyFile(p, target)
	})
}

func copyFile(src, dst string) error {
	data, err := content.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}
