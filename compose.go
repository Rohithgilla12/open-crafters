package opencrafters

// ComposeChallenges are meta-compose capstones: the learner implements a gateway
// and the harness spawns reference primitive services.
var ComposeChallenges = []string{
	"build-your-own-url-shortener",
	"build-your-own-job-platform",
	"build-your-own-cache-cluster",
	"build-your-own-workflow-worker",
	"build-your-own-distributed-kv",
	"build-your-own-chat-service",
	"build-your-own-notification-platform",
	"build-your-own-payment-ledger",
}

// IsComposeChallenge reports whether slug is a compose capstone.
func IsComposeChallenge(slug string) bool {
	for _, s := range ComposeChallenges {
		if s == slug {
			return true
		}
	}
	return false
}
