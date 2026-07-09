package opencrafters

import "testing"

func TestIsComposeChallenge(t *testing.T) {
	compose := []string{
		"build-your-own-url-shortener",
		"build-your-own-job-platform",
		"build-your-own-cache-cluster",
		"build-your-own-workflow-worker",
		"build-your-own-distributed-kv",
		"build-your-own-chat-service",
	}
	for _, slug := range compose {
		if !IsComposeChallenge(slug) {
			t.Errorf("%q should be a compose challenge", slug)
		}
	}
	if IsComposeChallenge("build-your-own-harness") {
		t.Error("harness is meta, not compose")
	}
	if IsComposeChallenge("build-your-own-wal") {
		t.Error("wal is not compose")
	}
}
