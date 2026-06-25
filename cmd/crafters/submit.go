package main

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func cmdSubmit(args []string) {
	url := os.Getenv("CRAFTERS_RUNNER_URL")
	token := os.Getenv("CRAFTERS_RUNNER_TOKEN")
	dir := "."
	all := false
	stage := ""
	challenge := ""
	wait := true

	for i := 0; i < len(args); i++ {
		switch a := args[i]; {
		case a == "--url":
			i++
			if i < len(args) {
				url = args[i]
			}
		case strings.HasPrefix(a, "--url="):
			url = strings.TrimPrefix(a, "--url=")
		case a == "--token":
			i++
			if i < len(args) {
				token = args[i]
			}
		case strings.HasPrefix(a, "--token="):
			token = strings.TrimPrefix(a, "--token=")
		case a == "--all":
			all = true
		case a == "--stage":
			i++
			if i < len(args) {
				stage = args[i]
			}
		case strings.HasPrefix(a, "--stage="):
			stage = strings.TrimPrefix(a, "--stage=")
		case a == "--challenge":
			i++
			if i < len(args) {
				challenge = args[i]
			}
		case strings.HasPrefix(a, "--challenge="):
			challenge = strings.TrimPrefix(a, "--challenge=")
		case a == "--no-wait":
			wait = false
		case a == "-h", a == "--help":
			printSubmitUsage(os.Stdout)
			return
		default:
			dir = a
		}
	}

	if url == "" {
		die("submit needs a runner URL — set CRAFTERS_RUNNER_URL or pass --url https://runner.gilla.fun")
	}
	if token == "" {
		die("submit needs a runner token — set CRAFTERS_RUNNER_TOKEN or pass --token <secret>")
	}
	if challenge == "" {
		slug, _ := solutionChallenge(dir)
		challenge = slug
	} else {
		challenge, _ = resolveChallenge(challenge)
	}
	if stage != "" && all {
		die("use either --stage or --all, not both")
	}

	program := filepath.Join(dir, "your_program.sh")
	if _, err := os.Stat(program); err != nil {
		die("%q isn't a solution directory (no your_program.sh)", dir)
	}

	zipBytes, err := zipSolutionDir(dir)
	if err != nil {
		die("%v", err)
	}

	job, err := postGrade(url, token, challenge, stage, all, zipBytes)
	if err != nil {
		die("%v", err)
	}
	fmt.Printf("Submitted job %s for %q\n", job.ID, challenge)
	if !wait {
		fmt.Printf("Poll: %s/v1/jobs/%s\n", strings.TrimRight(url, "/"), job.ID)
		return
	}

	final, err := pollJob(url, token, job.ID)
	if err != nil {
		die("%v", err)
	}
	fmt.Println()
	fmt.Print(final.Log)
	fmt.Println()
	switch final.Status {
	case "passed":
		fmt.Printf("\x1b[32;1m✓ Remote grading passed (job %s)\x1b[0m\n", final.ID)
		os.Exit(0)
	case "failed":
		fmt.Printf("\x1b[31m✗ Remote grading failed (exit %d, job %s)\x1b[0m\n", final.ExitCode, final.ID)
		os.Exit(1)
	default:
		fmt.Printf("\x1b[31m✗ Remote grading error: %s\x1b[0m\n", final.Error)
		os.Exit(1)
	}
}

func printSubmitUsage(w *os.File) {
	fmt.Fprint(w, `Usage: crafters submit [dir] [options]

Upload a solution directory to a hosted runner for sandboxed grading.

Environment:
  CRAFTERS_RUNNER_URL    base URL of the runner API (e.g. https://runner.gilla.fun)
  CRAFTERS_RUNNER_TOKEN  bearer token for the runner API
  CRAFTERS_RUNNER_RESOLVE_IP  optional: dial this IP for TLS (Tailscale MagicDNS workaround)

Options:
  --url <url>            runner base URL (overrides CRAFTERS_RUNNER_URL)
  --token <secret>       API token (overrides CRAFTERS_RUNNER_TOKEN)
  --challenge <slug>     challenge slug or fuzzy name (default: read from solution)
  --stage <slug>         grade up to this stage (default: resume next stage)
  --all                  grade every stage
  --no-wait              return immediately with the job id

Examples:
  export CRAFTERS_RUNNER_URL=https://runner.gilla.fun
  export CRAFTERS_RUNNER_TOKEN=...
  cd my-wal && crafters submit
  crafters submit --all --challenge wal ./my-wal
`)
}

type remoteJob struct {
	ID        string `json:"id"`
	Status    string `json:"status"`
	Challenge string `json:"challenge"`
	ExitCode  int    `json:"exit_code"`
	Log       string `json:"log"`
	Error     string `json:"error"`
}

func postGrade(baseURL, token, challenge, stage string, all bool, zipBytes []byte) (*remoteJob, error) {
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	_ = mw.WriteField("challenge", challenge)
	if all {
		_ = mw.WriteField("all", "true")
	}
	if stage != "" {
		_ = mw.WriteField("stage", stage)
	}
	fw, err := mw.CreateFormFile("file", "solution.zip")
	if err != nil {
		return nil, err
	}
	if _, err := io.Copy(fw, bytes.NewReader(zipBytes)); err != nil {
		return nil, err
	}
	if err := mw.Close(); err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, strings.TrimRight(baseURL, "/")+"/v1/grade", &body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := runnerHTTPClient(baseURL).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("runner returned %s: %s", resp.Status, strings.TrimSpace(string(b)))
	}
	var job remoteJob
	if err := json.NewDecoder(resp.Body).Decode(&job); err != nil {
		return nil, err
	}
	return &job, nil
}

func pollJob(baseURL, token, id string) (*remoteJob, error) {
	url := strings.TrimRight(baseURL, "/") + "/v1/jobs/" + id
	for {
		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := runnerHTTPClient(baseURL).Do(req)
		if err != nil {
			return nil, err
		}
		var job remoteJob
		err = json.NewDecoder(resp.Body).Decode(&job)
		resp.Body.Close()
		if err != nil {
			return nil, err
		}
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("runner returned %s", resp.Status)
		}
		switch job.Status {
		case "queued", "running":
			time.Sleep(2 * time.Second)
			continue
		default:
			return &job, nil
		}
	}
}

func runnerHTTPClient(baseURL string) *http.Client {
	resolveIP := strings.TrimSpace(os.Getenv("CRAFTERS_RUNNER_RESOLVE_IP"))
	if resolveIP == "" {
		return http.DefaultClient
	}
	u, err := url.Parse(baseURL)
	if err != nil {
		return http.DefaultClient
	}
	host := u.Hostname()
	dialer := &net.Dialer{Timeout: 30 * time.Second}
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{ServerName: host},
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			dialHost, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, err
			}
			if dialHost == host {
				addr = net.JoinHostPort(resolveIP, port)
			}
			return dialer.DialContext(ctx, network, addr)
		},
	}
	return &http.Client{Transport: transport}
}

func zipSolutionDir(dir string) ([]byte, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
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
		w, err := zw.Create(rel)
		if err != nil {
			return err
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(w, f)
		closeErr := f.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	})
	if err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func shouldSkipZipEntry(rel string, _ os.DirEntry) bool {
	if rel == ".git" || strings.HasPrefix(rel, ".git/") {
		return true
	}
	if rel == "node_modules" || strings.HasPrefix(rel, "node_modules/") {
		return true
	}
	if strings.Contains(rel, "__pycache__") {
		return true
	}
	if filepath.Base(rel) == ".DS_Store" {
		return true
	}
	if strings.HasPrefix(rel, ".") && !strings.HasPrefix(rel, ".open-crafters") {
		return true
	}
	return false
}
