package opencrafters

// ComposeChallenges are meta-compose capstones: the learner implements a gateway
// and the harness spawns reference primitive services.
var ComposeChallenges = []string{
	"build-your-own-url-shortener",
	"build-your-own-job-platform",
	"build-your-own-cache-cluster",
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
