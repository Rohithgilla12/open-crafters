package opencrafters

// ChallengePath groups challenges into a recommended learning sequence.
type ChallengePath struct {
	Slug        string
	Name        string
	Description string
	Challenges  []string
}

// ChallengePaths are curated tracks through the catalog. Every challenge in
// ChallengeOrder should appear in exactly one path.
var ChallengePaths = []ChallengePath{
	{
		Slug:        "durability",
		Name:        "Durability & storage",
		Description: "From write-ahead logs to object stores — how production systems make data survive crashes.",
		Challenges: []string{
			"build-your-own-wal",
			"build-your-own-queue",
			"build-your-own-log",
			"build-your-own-lsm",
			"build-your-own-mvcc",
			"build-your-own-object-store",
		},
	},
	{
		Slug:        "workflow",
		Name:        "Workflow engines",
		Description: "Build the server and the deterministic replay SDK behind Temporal-style orchestration.",
		Challenges: []string{
			"build-your-own-temporal",
			"build-your-own-workflow-sdk",
		},
	},
	{
		Slug:        "distributed",
		Name:        "Distributed systems",
		Description: "Consensus, placement, and probabilistic primitives for scaled-out infrastructure.",
		Challenges: []string{
			"build-your-own-raft",
			"build-your-own-hash-ring",
			"build-your-own-bloom-filter",
		},
	},
	{
		Slug:        "coordination",
		Name:        "Coordination & control",
		Description: "Schedulers, rate limits, and distributed locks — the glue between services.",
		Challenges: []string{
			"build-your-own-scheduler",
			"build-your-own-rate-limiter",
			"build-your-own-distributed-lock",
		},
	},
}
