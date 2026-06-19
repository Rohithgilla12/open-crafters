package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
)

// version is stamped at release time via -ldflags "-X main.version=vX.Y.Z".
var version = "dev"

const releaseBase = "https://github.com/Rohithgilla12/open-crafters/releases/latest/download"

// currentVersion prefers the ldflags-stamped version, then the module version
// (set when installed via `go install ...@vX.Y.Z`), else "dev".
func currentVersion() string {
	if version != "dev" {
		return version
	}
	if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
		return bi.Main.Version
	}
	return version
}

func cmdVersion() {
	fmt.Printf("crafters %s (%s/%s)\n", currentVersion(), runtime.GOOS, runtime.GOARCH)
}

// cmdUpdate downloads the latest released binary for this platform and replaces
// the running executable in place.
func cmdUpdate() {
	switch runtime.GOOS {
	case "linux", "darwin":
	default:
		die("self-update supports linux and darwin; on %s, rebuild with 'go install %s/cmd/crafters@latest'", runtime.GOOS, "github.com/Rohithgilla12/open-crafters")
	}

	self, err := os.Executable()
	if err != nil {
		die("locating the running binary: %v", err)
	}
	if resolved, err := filepath.EvalSymlinks(self); err == nil {
		self = resolved
	}

	asset := fmt.Sprintf("crafters_%s_%s", runtime.GOOS, runtime.GOARCH)
	url := releaseBase + "/" + asset
	fmt.Printf("→ downloading the latest crafters (%s/%s)…\n", runtime.GOOS, runtime.GOARCH)

	resp, err := http.Get(url) //nolint:gosec // fixed release URL
	if err != nil {
		die("download failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		die("download failed: HTTP %d from %s", resp.StatusCode, url)
	}

	// Write to a sibling temp file, then atomically rename over the running
	// binary (allowed on Unix; the live process keeps its open inode).
	tmp := self + ".new"
	out, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o755)
	if err != nil {
		die("cannot write next to %s: %v (is it on a writable path?)", self, err)
	}
	if _, err := io.Copy(out, resp.Body); err != nil {
		out.Close()
		os.Remove(tmp)
		die("download failed: %v", err)
	}
	out.Close()
	if err := os.Rename(tmp, self); err != nil {
		os.Remove(tmp)
		die("could not replace %s: %v", self, err)
	}
	fmt.Printf("\x1b[32m✓\x1b[0m updated %s\n  run 'crafters version' to confirm.\n", self)
}
