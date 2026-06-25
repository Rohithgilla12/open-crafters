package runner

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type checkRunCreate struct {
	Name    string `json:"name"`
	HeadSHA string `json:"head_sha"`
	Status  string `json:"status"`
}

type checkRunUpdate struct {
	Status     string      `json:"status"`
	Conclusion string      `json:"conclusion,omitempty"`
	Output     checkOutput `json:"output"`
}

type checkOutput struct {
	Title   string `json:"title"`
	Summary string `json:"summary"`
	Text    string `json:"text,omitempty"`
}

type checkRunResponse struct {
	ID int64 `json:"id"`
}

func (g *GitHubClient) createCheck(owner, repo, sha string) (int64, error) {
	body, _ := json.Marshal(checkRunCreate{
		Name:    "open-crafters",
		HeadSHA: sha,
		Status:  "in_progress",
	})
	var resp checkRunResponse
	if err := g.apiJSON(http.MethodPost, fmt.Sprintf("/repos/%s/%s/check-runs", owner, repo), body, &resp); err != nil {
		return 0, err
	}
	return resp.ID, nil
}

func (g *GitHubClient) finishCheck(job *Job, status Status, logText, errMsg string) {
	if g == nil || job.GitHub == nil || job.GitHub.CheckRunID == 0 {
		return
	}
	conclusion := "success"
	title := fmt.Sprintf("%s — passed", job.Challenge)
	summary := "All requested stages passed."
	if status == StatusFailed {
		conclusion = "failure"
		title = fmt.Sprintf("%s — failed", job.Challenge)
		summary = "One or more stages failed."
	} else if status == StatusError {
		conclusion = "failure"
		title = "Grading error"
		summary = errMsg
	}
	text := logText
	if text == "" {
		text = errMsg
	}
	if len(text) > 60000 {
		text = text[:60000] + "\n…(truncated)"
	}
	body, _ := json.Marshal(checkRunUpdate{
		Status:     "completed",
		Conclusion: conclusion,
		Output: checkOutput{
			Title:   title,
			Summary: summary,
			Text:    text,
		},
	})
	url := fmt.Sprintf("/repos/%s/%s/check-runs/%d", job.GitHub.Owner, job.GitHub.Repo, job.GitHub.CheckRunID)
	_ = g.apiJSON(http.MethodPatch, url, body, nil)
}

func (g *GitHubClient) apiJSON(method, path string, body []byte, out any) error {
	req, err := http.NewRequest(method, "https://api.github.com"+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+g.token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := g.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("github api %s %s: %s", method, path, strings.TrimSpace(string(b)))
	}
	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}
