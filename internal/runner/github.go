package runner

import (
	"archive/tar"
	"compress/gzip"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type pushEvent struct {
	Ref        string `json:"ref"`
	Before     string `json:"before"`
	After      string `json:"after"`
	Deleted    bool   `json:"deleted"`
	Repository struct {
		FullName      string `json:"full_name"`
		DefaultBranch string `json:"default_branch"`
	} `json:"repository"`
}

// GitHubClient posts check runs for push-triggered grades.
type GitHubClient struct {
	token  string
	client *http.Client
}

func NewGitHubClient(token string) *GitHubClient {
	if token == "" {
		return nil
	}
	return &GitHubClient{token: token, client: http.DefaultClient}
}

func VerifyGitHubSignature(secret string, body []byte, sigHeader string) bool {
	if secret == "" {
		return false
	}
	const prefix = "sha256="
	if !strings.HasPrefix(sigHeader, prefix) {
		return false
	}
	got, err := hex.DecodeString(strings.TrimPrefix(sigHeader, prefix))
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(body)
	want := mac.Sum(nil)
	return hmac.Equal(got, want)
}

func parsePushEvent(body []byte) (*pushEvent, error) {
	var ev pushEvent
	if err := json.Unmarshal(body, &ev); err != nil {
		return nil, err
	}
	if ev.Repository.FullName == "" || ev.After == "" {
		return nil, fmt.Errorf("not a push event")
	}
	return &ev, nil
}

func branchFromRef(ref string) string {
	return strings.TrimPrefix(ref, "refs/heads/")
}

func (g *GitHubClient) downloadAndPrepare(owner, repo, sha string) (challenge string, zipBytes []byte, err error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/tarball/%s", owner, repo, sha)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+g.token)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := g.client.Do(req)
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", nil, fmt.Errorf("github tarball %s: %s", resp.Status, strings.TrimSpace(string(b)))
	}

	tmp, err := os.MkdirTemp("", "gh-tar-*")
	if err != nil {
		return "", nil, err
	}
	defer os.RemoveAll(tmp)

	if err := extractTarGz(resp.Body, tmp); err != nil {
		return "", nil, err
	}
	solutionDir, err := FindSolutionRoot(tmp)
	if err != nil {
		return "", nil, err
	}
	challenge, err = readChallengeSlug(solutionDir)
	if err != nil {
		return "", nil, err
	}
	zipBytes, err = ZipDir(solutionDir)
	return challenge, zipBytes, err
}

func extractTarGz(r io.Reader, dest string) error {
	gr, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("gzip: %w", err)
	}
	defer gr.Close()
	tr := tar.NewReader(gr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		name := filepath.Clean(hdr.Name)
		if strings.HasPrefix(name, "..") || filepath.IsAbs(name) {
			return fmt.Errorf("tar entry %q escapes destination", hdr.Name)
		}
		target := filepath.Join(dest, name)
		if !strings.HasPrefix(target, filepath.Clean(dest)+string(os.PathSeparator)) {
			return fmt.Errorf("tar entry %q escapes destination", hdr.Name)
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode)|0o644)
			if err != nil {
				return err
			}
			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return err
			}
			if err := f.Close(); err != nil {
				return err
			}
		}
	}
}

func FindSolutionRoot(searchRoot string) (string, error) {
	var found string
	err := filepath.WalkDir(searchRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Base(path) != "your_program.sh" {
			return nil
		}
		dir := filepath.Dir(path)
		if _, err := os.Stat(filepath.Join(dir, ".open-crafters", "challenge")); err != nil {
			return nil
		}
		if found != "" {
			return fmt.Errorf("multiple open-crafters solutions in repository")
		}
		found = dir
		return nil
	})
	if err != nil {
		return "", err
	}
	if found == "" {
		return "", fmt.Errorf("no open-crafters solution found (need your_program.sh and .open-crafters/challenge)")
	}
	return found, nil
}

func readChallengeSlug(solutionDir string) (string, error) {
	data, err := os.ReadFile(filepath.Join(solutionDir, ".open-crafters", "challenge"))
	if err != nil {
		return "", fmt.Errorf("reading .open-crafters/challenge: %w", err)
	}
	slug := strings.TrimSpace(string(data))
	if slug == "" {
		return "", fmt.Errorf(".open-crafters/challenge is empty")
	}
	return slug, nil
}

func splitRepo(full string) (owner, repo string, err error) {
	parts := strings.Split(full, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid repository %q", full)
	}
	return parts[0], parts[1], nil
}
