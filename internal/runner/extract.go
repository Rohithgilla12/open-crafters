package runner

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ExtractZip unpacks a submission archive into dest and returns the path to
// your_program.sh (searched at the archive root and one directory deep).
func ExtractZip(r io.ReaderAt, size int64, dest string) (string, error) {
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return "", fmt.Errorf("reading zip: %w", err)
	}
	for _, f := range zr.File {
		if err := extractZipFile(f, dest); err != nil {
			return "", err
		}
	}
	program, err := findProgram(dest)
	if err != nil {
		return "", err
	}
	if err := os.Chmod(program, 0o755); err != nil {
		return "", fmt.Errorf("chmod your_program.sh: %w", err)
	}
	return program, nil
}

func extractZipFile(f *zip.File, dest string) error {
	name := filepath.Clean(f.Name)
	if strings.HasPrefix(name, "..") || filepath.IsAbs(name) {
		return fmt.Errorf("zip entry %q escapes destination", f.Name)
	}
	target := filepath.Join(dest, name)
	if !strings.HasPrefix(target, filepath.Clean(dest)+string(os.PathSeparator)) && target != filepath.Clean(dest) {
		return fmt.Errorf("zip entry %q escapes destination", f.Name)
	}
	if f.FileInfo().IsDir() {
		return os.MkdirAll(target, 0o755)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, f.Mode()|0o111)
	if err != nil {
		return err
	}
	_, err = io.Copy(out, rc)
	closeErr := out.Close()
	if err != nil {
		return err
	}
	return closeErr
}

func findProgram(root string) (string, error) {
	candidates := []string{
		filepath.Join(root, "your_program.sh"),
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return "", err
	}
	for _, e := range entries {
		if e.IsDir() {
			candidates = append(candidates, filepath.Join(root, e.Name(), "your_program.sh"))
		}
	}
	for _, c := range candidates {
		if st, err := os.Stat(c); err == nil && !st.IsDir() {
			return c, nil
		}
	}
	return "", fmt.Errorf("zip must contain your_program.sh at the root or in a single top-level directory")
}
