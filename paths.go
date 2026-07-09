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
		Description: "Build the Temporal-style server, the deterministic replay SDK, and the worker gateway that wires them together.",
		Challenges: []string{
			"build-your-own-temporal",
			"build-your-own-workflow-sdk",
			"build-your-own-workflow-worker",
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
			"build-your-own-distributed-cache",
			"build-your-own-distributed-kv",
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
			"build-your-own-id-generator",
		},
	},
	{
		Slug:        "integration",
		Name:        "Compose & meta",
		Description: "Wire primitives into real systems — URL shortener, job platform, cache cluster, and chat service gateways, plus building the grader itself. You build one service; the harness spawns the rest. (The workflow worker compose capstone lives on the workflow track; distributed KV on the distributed track.)",
		Challenges: []string{
			"build-your-own-url-shortener",
			"build-your-own-job-platform",
			"build-your-own-cache-cluster",
			"build-your-own-chat-service",
			"build-your-own-harness",
		},
	},
}
