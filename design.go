package opencrafters

// DesignProblemOrder is the canonical display order for system design problems.
var DesignProblemOrder = []string{
	"design-distributed-scheduler",
	"design-realtime-feed",
	"design-workflow-platform",
	"design-chat-at-scale",
	"design-url-shortener",
	"design-payment-ledger",
	"design-notification-system",
	"design-distributed-kv",
	"design-api-gateway",
	"design-video-streaming",
	"design-search-index",
	"design-multi-tenant-saas",
	"design-event-streaming",
	"design-blob-storage",
	"design-distributed-cache",
	"design-coordination-service",
}

// DesignProblem is metadata for a system design scenario in the learn app.
type DesignProblem struct {
	Slug               string
	Name               string
	Tagline            string
	Difficulty         string
	Category           string
	TimeMinutes        int
	RelatedChallenges  []string
	DiscussionPrompts  []string
}

// DesignProblems are learner-facing system design scenarios (read + discuss, not graded).
var DesignProblems = []DesignProblem{
	{
		Slug:        "design-distributed-scheduler",
		Name:        "Design a distributed job scheduler",
		Tagline:     "Delayed work, leases, retries, and crash recovery at platform scale.",
		Difficulty:  "hard",
		Category:    "coordination",
		TimeMinutes: 45,
		RelatedChallenges: []string{
			"build-your-own-scheduler",
			"build-your-own-queue",
			"build-your-own-distributed-lock",
		},
		DiscussionPrompts: []string{
			"What are the core entities (jobs, workers, schedules) and who owns each?",
			"How do you guarantee at-least-once execution without double side effects?",
			"Where do leases live, and what happens when a worker dies mid-job?",
			"How do recurring jobs survive restarts without firing twice?",
			"Sketch the hot path for schedule → poll → lease → complete.",
		},
	},
	{
		Slug:        "design-realtime-feed",
		Name:        "Design a realtime social feed",
		Tagline:     "Home timelines, fan-out, celebrity posts, and read-heavy traffic.",
		Difficulty:  "hard",
		Category:    "scale",
		TimeMinutes: 50,
		RelatedChallenges: []string{
			"build-your-own-rate-limiter",
			"build-your-own-bloom-filter",
			"build-your-own-log",
		},
		DiscussionPrompts: []string{
			"Write vs read QPS — which path is the bottleneck?",
			"Fan-out on write vs fan-out on read — when do you switch strategies?",
			"How do you rank and paginate a timeline that's always changing?",
			"Where do rate limits and abuse controls sit?",
			"How do you backfill a new follower without melting the database?",
		},
	},
	{
		Slug:        "design-workflow-platform",
		Name:        "Design a workflow orchestration platform",
		Tagline:     "Durable executions, timers, signals, and deterministic replay.",
		Difficulty:  "hard",
		Category:    "workflow",
		TimeMinutes: 55,
		RelatedChallenges: []string{
			"build-your-own-temporal",
			"build-your-own-workflow-sdk",
			"build-your-own-wal",
		},
		DiscussionPrompts: []string{
			"Split responsibilities: what does the server own vs the worker SDK?",
			"What goes in an event history, and what must never be stored there?",
			"How do timers fire reliably across restarts and clock skew?",
			"Why must workflow code be deterministic — what breaks if it isn't?",
			"Walk through crash recovery: worker dies mid-activity, then restarts.",
		},
	},
	{
		Slug:        "design-chat-at-scale",
		Name:        "Design chat at scale",
		Tagline:     "1:1 and group messaging, delivery guarantees, and presence.",
		Difficulty:  "medium",
		Category:    "messaging",
		TimeMinutes: 45,
		RelatedChallenges: []string{
			"build-your-own-queue",
			"build-your-own-id-generator",
			"build-your-own-raft",
		},
		DiscussionPrompts: []string{
			"How do you order messages in a group with concurrent senders?",
			"At-most-once vs at-least-once vs exactly-once — pick per operation and justify.",
			"How do clients sync history after offline periods?",
			"Where do you shard conversations, and how do clients find the right shard?",
			"Presence and typing indicators — hot path vs best-effort?",
		},
	},
	{
		Slug:        "design-url-shortener",
		Name:        "Design a URL shortener",
		Tagline:     "Billions of links, global redirects, and custom aliases at read-heavy scale.",
		Difficulty:  "medium",
		Category:    "storage",
		TimeMinutes: 40,
		RelatedChallenges: []string{
			"build-your-own-id-generator",
			"build-your-own-bloom-filter",
			"build-your-own-object-store",
		},
		DiscussionPrompts: []string{
			"How do you generate short codes — random, counter, or hash — and handle collisions?",
			"What's the read path for a redirect, and how do you hit sub-10ms p99?",
			"How do custom aliases differ from auto-generated codes in storage?",
			"Where do click analytics go without slowing redirects?",
			"How do you expire or recycle short links safely?",
		},
	},
	{
		Slug:        "design-payment-ledger",
		Name:        "Design a payment ledger",
		Tagline:     "Double-entry balances, idempotent transfers, and auditability under failures.",
		Difficulty:  "hard",
		Category:    "durability",
		TimeMinutes: 50,
		RelatedChallenges: []string{
			"build-your-own-wal",
			"build-your-own-mvcc",
			"build-your-own-id-generator",
		},
		DiscussionPrompts: []string{
			"What is your ledger data model — accounts, entries, transactions?",
			"How do you make transfer API idempotent when clients retry?",
			"Where does double-entry bookkeeping show up in the schema?",
			"How do you reconcile balances after a partial crash mid-transfer?",
			"What can you expose for auditors vs what stays internal?",
		},
	},
	{
		Slug:        "design-notification-system",
		Name:        "Design a notification platform",
		Tagline:     "Email, push, and SMS fan-out with preferences, retries, and rate limits.",
		Difficulty:  "medium",
		Category:    "coordination",
		TimeMinutes: 45,
		RelatedChallenges: []string{
			"build-your-own-queue",
			"build-your-own-scheduler",
			"build-your-own-rate-limiter",
		},
		DiscussionPrompts: []string{
			"How do you model user channel preferences (push vs email vs quiet hours)?",
			"What's the path from product event → delivered notification?",
			"How do you dedupe \"same alert sent twice\" across retries?",
			"Where do per-provider rate limits (APNs, SendGrid) get enforced?",
			"How do scheduled digests differ from immediate sends architecturally?",
		},
	},
	{
		Slug:        "design-distributed-kv",
		Name:        "Design a distributed KV store",
		Tagline:     "Partitioning, replication, consistency tiers, and rebalancing under churn.",
		Difficulty:  "hard",
		Category:    "distributed",
		TimeMinutes: 55,
		RelatedChallenges: []string{
			"build-your-own-raft",
			"build-your-own-hash-ring",
			"build-your-own-lsm",
		},
		DiscussionPrompts: []string{
			"How do keys map to partitions, and who is the leader for a key?",
			"What consistency level do you offer — strong, eventual, tunable?",
			"Walk through a write when the primary partition is unavailable.",
			"How do you add a node without moving every key?",
			"Where does an LSM-tree live in your stack vs a simple B-tree?",
		},
	},
	{
		Slug:        "design-api-gateway",
		Name:        "Design a rate-limited API gateway",
		Tagline:     "Auth, routing, quotas, and edge caching in front of microservices.",
		Difficulty:  "medium",
		Category:    "coordination",
		TimeMinutes: 45,
		RelatedChallenges: []string{
			"build-your-own-rate-limiter",
			"build-your-own-distributed-lock",
			"build-your-own-hash-ring",
		},
		DiscussionPrompts: []string{
			"What runs at the edge vs the central gateway tier?",
			"How do you enforce per-API-key and per-tenant rate limits under concurrency?",
			"Where does authentication end and authorization begin?",
			"How do you route to healthy backends during deploys and failures?",
			"What do you cache at the gateway, and what must never be cached?",
		},
	},
	{
		Slug:        "design-video-streaming",
		Name:        "Design video upload and streaming",
		Tagline:     "Multipart uploads, transcoding pipelines, and CDN delivery at global scale.",
		Difficulty:  "hard",
		Category:    "media",
		TimeMinutes: 50,
		RelatedChallenges: []string{
			"build-your-own-object-store",
			"build-your-own-queue",
			"build-your-own-scheduler",
		},
		DiscussionPrompts: []string{
			"How does a client upload a 4 GB file reliably over flaky networks?",
			"What's the pipeline from raw upload → transcoded renditions → playback?",
			"How do you serve adaptive bitrate (HLS/DASH) from a CDN?",
			"Where do you store metadata vs bytes, and how are they linked?",
			"How do view counts and trending work without overloading origin?",
		},
	},
	{
		Slug:        "design-search-index",
		Name:        "Design a search index",
		Tagline:     "Ingestion, inverted indexes, ranking, and query latency at web scale.",
		Difficulty:  "hard",
		Category:    "storage",
		TimeMinutes: 50,
		RelatedChallenges: []string{
			"build-your-own-log",
			"build-your-own-lsm",
			"build-your-own-bloom-filter",
		},
		DiscussionPrompts: []string{
			"How do documents flow from product DB → searchable index?",
			"What does your inverted index look like on disk?",
			"How do you handle deletes and updates without full reindex?",
			"Autocomplete vs full-text search — same system or separate?",
			"How do you shard the index and merge results across shards?",
		},
	},
	{
		Slug:        "design-multi-tenant-saas",
		Name:        "Design multi-tenant SaaS isolation",
		Tagline:     "Tenant boundaries, noisy neighbors, billing meters, and data residency.",
		Difficulty:  "hard",
		Category:    "platform",
		TimeMinutes: 50,
		RelatedChallenges: []string{
			"build-your-own-mvcc",
			"build-your-own-rate-limiter",
			"build-your-own-id-generator",
		},
		DiscussionPrompts: []string{
			"Shared DB vs schema-per-tenant vs DB-per-tenant — when do you pick each?",
			"How does every request get a trusted tenant_id?",
			"How do you prevent tenant A from reading tenant B's rows?",
			"Where do usage meters plug in for billing?",
			"How do you migrate a whale tenant off the shared pool?",
		},
	},
	{
		Slug:        "design-event-streaming",
		Name:        "Design an event streaming platform",
		Tagline:     "Topics, partitions, consumer groups, replay, and retention under write-heavy load.",
		Difficulty:  "hard",
		Category:    "messaging",
		TimeMinutes: 50,
		RelatedChallenges: []string{
			"build-your-own-log",
			"build-your-own-wal",
			"build-your-own-queue",
		},
		DiscussionPrompts: []string{
			"What are the core entities (topic, partition, offset, consumer group) and who owns each?",
			"How do producers pick a partition, and what breaks when one partition goes hot?",
			"At-least-once vs exactly-once delivery — where do you draw the line per use case?",
			"How does retention work without renumbering offsets or breaking replay?",
			"Sketch the path from produce → durable append → fetch → commit offset.",
		},
	},
	{
		Slug:        "design-blob-storage",
		Name:        "Design a blob storage service",
		Tagline:     "Buckets, multipart uploads, etag integrity, and prefix listing at object scale.",
		Difficulty:  "hard",
		Category:    "storage",
		TimeMinutes: 50,
		RelatedChallenges: []string{
			"build-your-own-object-store",
			"build-your-own-wal",
			"build-your-own-hash-ring",
		},
		DiscussionPrompts: []string{
			"How do you separate metadata (bucket, key, etag) from bytes on disk?",
			"Walk through multipart upload: initiate → parts → complete → abort.",
			"What's the read path for GET, and how do you verify integrity with etags?",
			"How do you list by prefix when a bucket holds billions of objects?",
			"Where does crash durability matter — part uploads, commit, or delete?",
		},
	},
	{
		Slug:        "design-distributed-cache",
		Name:        "Design a distributed cache",
		Tagline:     "Sharded hot keys, eviction, stampedes, and coherence without a single choke point.",
		Difficulty:  "medium",
		Category:    "distributed",
		TimeMinutes: 45,
		RelatedChallenges: []string{
			"build-your-own-hash-ring",
			"build-your-own-bloom-filter",
			"build-your-own-rate-limiter",
			"build-your-own-distributed-cache",
		},
		DiscussionPrompts: []string{
			"How do keys map to cache nodes, and what happens when you add or remove a node?",
			"Cache-aside vs read-through vs write-through — pick per operation and justify.",
			"How do you prevent a cache stampede when a hot key expires?",
			"Where do bloom filters help, and what false positives are acceptable?",
			"TTL eviction vs LRU vs explicit invalidation — how do they interact under churn?",
		},
	},
	{
		Slug:        "design-coordination-service",
		Name:        "Design a coordination service",
		Tagline:     "Linearizable metadata, watches, leases, and compare-and-swap under partition.",
		Difficulty:  "hard",
		Category:    "coordination",
		TimeMinutes: 50,
		RelatedChallenges: []string{
			"build-your-own-raft",
			"build-your-own-distributed-lock",
			"build-your-own-wal",
		},
		DiscussionPrompts: []string{
			"What operations do you expose — get/put/delete, CAS, ephemeral keys, watches?",
			"How does Raft map to your request path for a linearizable write?",
			"How do leases and fencing tokens stop a stale leader from corrupting state?",
			"Watches vs long-poll vs push — how do clients learn about key changes?",
			"Walk through leader failover: old leader partitioned, new leader elected, client retries.",
		},
	},
}

// DesignBySlug returns a design problem definition or false.
func DesignBySlug(slug string) (DesignProblem, bool) {
	for _, d := range DesignProblems {
		if d.Slug == slug {
			return d, true
		}
	}
	return DesignProblem{}, false
}
