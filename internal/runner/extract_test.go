package runner_test

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/Rohithgilla12/open-crafters/internal/runner"
)

func TestExtractZipFindsProgramAtRoot(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("your_program.sh")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("#!/bin/sh\necho ok\n")); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	dest := t.TempDir()
	program, err := runner.ExtractZip(bytes.NewReader(buf.Bytes()), int64(buf.Len()), dest)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(program) != "your_program.sh" {
		t.Fatalf("got %q", program)
	}
	st, err := os.Stat(program)
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode()&0o111 == 0 {
		t.Fatal("expected executable bit")
	}
}

func TestExtractZipFindsProgramOneLevelDeep(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("my-wal/your_program.sh")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("#!/bin/sh\n")); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	dest := t.TempDir()
	_, err = runner.ExtractZip(bytes.NewReader(buf.Bytes()), int64(buf.Len()), dest)
	if err != nil {
		t.Fatal(err)
	}
}
