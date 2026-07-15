package opencrafters

// DesignStackMilestone is one step on a whiteboard→build journey.
type DesignStackMilestone struct {
	Kind  string // "design" or "build"
	Slug  string
	Blurb string
}

// DesignStack is a curated path from system design to implementation.
type DesignStack struct {
	Slug        string
	Name        string
	Tagline     string
	Description string
	Outcomes    []string
	Milestones  []DesignStackMilestone
}

// DesignStacks link design problems to ordered build challenges.
var DesignStacks = []DesignStack{
	{
		Slug:        "url-shortener",
		Name:        "URL shortener stack",
		Tagline:     "Whiteboard the redirects, then build the three primitives underneath.",
		Description: "Classic interview warm-up — map the read-heavy redirect path on paper, then implement ID generation, fast existence checks, and blob storage for analytics.",
		Outcomes: []string{
			"Complete the URL shortener design problem",
			"Implement snowflake-style codes, Bloom filters, and object storage",
			"See how design boxes map 1:1 to graded challenges",
		},
		Milestones: []DesignStackMilestone{
			{Kind: "design", Slug: "design-url-shortener", Blurb: "Whiteboard — codes, CDN, analytics (~40 min)"},
			{Kind: "build", Slug: "build-your-own-id-generator", Blurb: "Short codes from a monotonic counter"},
			{Kind: "build", Slug: "build-your-own-bloom-filter", Blurb: "Probable code existence before DB lookup"},
			{Kind: "build", Slug: "build-your-own-object-store", Blurb: "Click logs and cold metadata blobs"},
			{Kind: "build", Slug: "build-your-own-url-shortener", Blurb: "Meta-compose — wire the three services in a gateway"},
		},
	},
	{
		Slug:        "job-scheduler",
		Name:        "Job scheduler stack",
		Tagline:     "From platform cron design to scheduler, queue, and lock.",
		Description: "Platform teams need delayed work with leases and retries. Design the scheduler first, then build the three coordination primitives that power it.",
		Outcomes: []string{
			"Design lease-based job execution under worker crashes",
			"Build a durable scheduler and at-least-once queue",
			"Use distributed locks for singleton components",
		},
		Milestones: []DesignStackMilestone{
			{Kind: "design", Slug: "design-distributed-scheduler", Blurb: "Whiteboard — schedule, poll, lease, complete"},
			{Kind: "build", Slug: "build-your-own-scheduler", Blurb: "Fire times, recurring jobs, durability"},
			{Kind: "build", Slug: "build-your-own-queue", Blurb: "Worker delivery with visibility timeout"},
			{Kind: "build", Slug: "build-your-own-distributed-lock", Blurb: "Leader election or exclusive sections"},
			{Kind: "build", Slug: "build-your-own-job-platform", Blurb: "Meta-compose — wire scheduler, queue, and lock in a gateway"},
		},
	},
	{
		Slug:        "workflow-platform",
		Name:        "Workflow platform stack",
		Tagline:     "Histories on disk, orchestration server, deterministic SDK.",
		Description: "Temporal-style systems split into durable server and replay worker. Design the platform, then build WAL → server → SDK in dependency order.",
		Outcomes: []string{
			"Design event histories and timer delivery",
			"Build append-only durability under the server",
			"Implement deterministic replay in the worker SDK",
			"Wire Temporal + SDK into a worker gateway",
		},
		Milestones: []DesignStackMilestone{
			{Kind: "design", Slug: "design-workflow-platform", Blurb: "Whiteboard — server vs SDK, replay, timers"},
			{Kind: "build", Slug: "build-your-own-wal", Blurb: "Append histories before ack"},
			{Kind: "build", Slug: "build-your-own-temporal", Blurb: "Durable workflow server"},
			{Kind: "build", Slug: "build-your-own-workflow-sdk", Blurb: "Same history → same commands, every time"},
			{Kind: "build", Slug: "build-your-own-workflow-worker", Blurb: "Meta-compose — poll Temporal, replay on SDK, complete tasks"},
		},
	},
	{
		Slug:        "distributed-kv",
		Name:        "Distributed KV stack",
		Tagline:     "Placement, consensus, and on-disk segments.",
		Description: "Shard keys across nodes, elect leaders per partition, and store bytes in an LSM. The design problem names the boxes; the builds fill them in.",
		Outcomes: []string{
			"Design partitioning and consistency tiers",
			"Implement Raft replication per shard",
			"Store values in an LSM-tree engine",
		},
		Milestones: []DesignStackMilestone{
			{Kind: "design", Slug: "design-distributed-kv", Blurb: "Whiteboard — shards, leaders, rebalancing"},
			{Kind: "build", Slug: "build-your-own-hash-ring", Blurb: "Key → node routing"},
			{Kind: "build", Slug: "build-your-own-raft", Blurb: "Leader election + replicated log"},
			{Kind: "build", Slug: "build-your-own-lsm", Blurb: "Memtable + SSTables per node"},
			{Kind: "build", Slug: "build-your-own-distributed-kv", Blurb: "Meta-compose — ring, Raft shard, and LSM shard"},
		},
	},
	{
		Slug:        "chat-at-scale",
		Name:        "Chat at scale stack",
		Tagline:     "Conversation design → log, queue, IDs, and consensus.",
		Description: "Messaging needs ordering, durability, and fan-out. Whiteboard the send path, then build the log and delivery primitives — optionally Raft for shard leadership.",
		Outcomes: []string{
			"Design per-conversation ordering and delivery",
			"Build append logs and async fan-out queues",
			"Generate ordered message IDs at scale",
		},
		Milestones: []DesignStackMilestone{
			{Kind: "design", Slug: "design-chat-at-scale", Blurb: "Whiteboard — shard, seq, fan-out"},
			{Kind: "build", Slug: "build-your-own-id-generator", Blurb: "Monotonic per-conversation sequences"},
			{Kind: "build", Slug: "build-your-own-log", Blurb: "Durable conversation log"},
			{Kind: "build", Slug: "build-your-own-queue", Blurb: "Push delivery to online clients"},
			{Kind: "build", Slug: "build-your-own-chat-service", Blurb: "Meta-compose — id gen, log, and delivery queue"},
		},
	},
	{
		Slug:        "distributed-cache",
		Name:        "Distributed cache stack",
		Tagline:     "Whiteboard the cluster, then build routing, negative cache, stampede control, and the node.",
		Description: "Memcached-class systems combine shard routing, optional bloom negative caches, stampede limits, and fast in-memory nodes. Design the cluster, then implement each layer.",
		Outcomes: []string{
			"Complete the distributed cache design problem",
			"Route keys with a hash ring and skip definite misses with a bloom filter",
			"Build the cache node with TTL, CAS, and LRU eviction",
		},
		Milestones: []DesignStackMilestone{
			{Kind: "design", Slug: "design-distributed-cache", Blurb: "Whiteboard — shards, eviction, stampedes (~45 min)"},
			{Kind: "build", Slug: "build-your-own-hash-ring", Blurb: "Key → cache node placement"},
			{Kind: "build", Slug: "build-your-own-bloom-filter", Blurb: "Client-side negative cache"},
			{Kind: "build", Slug: "build-your-own-rate-limiter", Blurb: "Throttle hot-key stampedes"},
			{Kind: "build", Slug: "build-your-own-distributed-cache", Blurb: "In-memory node — GET/SET/TTL/LRU"},
			{Kind: "build", Slug: "build-your-own-cache-cluster", Blurb: "Meta-compose — wire ring, bloom, limiter, and nodes"},
		},
	},
	{
		Slug:        "notification-platform",
		Name:        "Notification platform stack",
		Tagline:     "Whiteboard fan-out, then build queue, scheduler, limits, and the gateway.",
		Description: "Product events become email, push, and SMS. Design preferences and delivery first, then build the queue, delayed digests, rate limits, and the compose gateway that wires them.",
		Outcomes: []string{
			"Design channel preferences, retries, and provider caps",
			"Build durable delivery queues and delayed digests",
			"Wire queue, scheduler, and rate limiter in one gateway",
		},
		Milestones: []DesignStackMilestone{
			{Kind: "design", Slug: "design-notification-system", Blurb: "Whiteboard — preferences, fan-out, quiet hours"},
			{Kind: "build", Slug: "build-your-own-queue", Blurb: "Per-channel delivery jobs"},
			{Kind: "build", Slug: "build-your-own-scheduler", Blurb: "Digests and delayed sends"},
			{Kind: "build", Slug: "build-your-own-rate-limiter", Blurb: "Per-user and per-provider caps"},
			{Kind: "build", Slug: "build-your-own-notification-platform", Blurb: "Meta-compose — wire the three services in a gateway"},
		},
	},
	{
		Slug:        "payment-ledger",
		Name:        "Payment ledger stack",
		Tagline:     "Whiteboard double-entry, then build WAL, IDs, MVCC, and the gateway.",
		Description: "Ledgers need durable appends, unique transfer IDs, and snapshot-isolated balances. Design the money path, then implement the primitives and the compose gateway that wires them.",
		Outcomes: []string{
			"Design idempotent transfers and audit trails",
			"Build WAL durability, snowflake IDs, and MVCC balances",
			"Wire the three primitives into a ledger gateway",
		},
		Milestones: []DesignStackMilestone{
			{Kind: "design", Slug: "design-payment-ledger", Blurb: "Whiteboard — accounts, entries, idempotency"},
			{Kind: "build", Slug: "build-your-own-wal", Blurb: "Append before ack"},
			{Kind: "build", Slug: "build-your-own-id-generator", Blurb: "Transfer and idempotency IDs"},
			{Kind: "build", Slug: "build-your-own-mvcc", Blurb: "Snapshot balance reads"},
			{Kind: "build", Slug: "build-your-own-payment-ledger", Blurb: "Meta-compose — wire WAL, id gen, and MVCC"},
		},
	},
}

// DesignStackBySlug returns a design stack or false.
func DesignStackBySlug(slug string) (DesignStack, bool) {
	for _, s := range DesignStacks {
		if s.Slug == slug {
			return s, true
		}
	}
	return DesignStack{}, false
}

// DesignStacksForChallenge returns stack slugs that include a build challenge.
func DesignStacksForChallenge(challengeSlug string) []string {
	var out []string
	for _, st := range DesignStacks {
		for _, m := range st.Milestones {
			if m.Kind == "build" && m.Slug == challengeSlug {
				out = append(out, st.Slug)
				break
			}
		}
	}
	return out
}

// DesignStacksForDesign returns stack slugs that start with or include a design problem.
func DesignStacksForDesign(designSlug string) []string {
	var out []string
	for _, st := range DesignStacks {
		for _, m := range st.Milestones {
			if m.Kind == "design" && m.Slug == designSlug {
				out = append(out, st.Slug)
				break
			}
		}
	}
	return out
}
