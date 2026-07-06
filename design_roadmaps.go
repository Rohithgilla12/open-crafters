package opencrafters

// DesignRoadmapMilestone is one step on a system design roadmap.
type DesignRoadmapMilestone struct {
	Problem string // design problem slug
	Blurb   string
}

// DesignRoadmap is a curated whiteboard journey through design problems.
type DesignRoadmap struct {
	Slug        string
	Name        string
	Tagline     string
	Description string
	Outcomes    []string
	Milestones  []DesignRoadmapMilestone
}

// DesignRoadmaps group the 16 design problems into learning journeys.
var DesignRoadmaps = []DesignRoadmap{
	{
		Slug:        "interview-classics",
		Name:        "Interview classics",
		Tagline:     "The problems that show up again and again in senior loops.",
		Description: "Start with the highest-frequency system design prompts — short links, feeds, chat, money, and API gateways — before diving into storage engines or consensus.",
		Outcomes: []string{
			"Structure requirements, scale, and trade-offs under time pressure",
			"Handle read-heavy vs write-heavy bottlenecks",
			"Connect whiteboard boxes to open-crafters build challenges",
		},
		Milestones: []DesignRoadmapMilestone{
			{Problem: "design-url-shortener", Blurb: "Warm up — codes, redirects, and 100:1 read ratio."},
			{Problem: "design-realtime-feed", Blurb: "Fan-out on write vs read; the celebrity problem."},
			{Problem: "design-chat-at-scale", Blurb: "Ordering, delivery, and sharding conversations."},
			{Problem: "design-payment-ledger", Blurb: "Double-entry, idempotency, and audit trails."},
			{Problem: "design-api-gateway", Blurb: "Auth, rate limits, and routing at the edge."},
		},
	},
	{
		Slug:        "storage-and-data",
		Name:        "Storage & data planes",
		Tagline:     "Blobs, indexes, ledgers, logs, and KV at scale.",
		Description: "When the hard part is where bytes live, how they are indexed, and what survives a crash. Pairs naturally with the durability build path.",
		Outcomes: []string{
			"Separate metadata from payload storage",
			"Reason about consistency, retention, and replay",
			"Design ingestion pipelines into searchable indexes",
		},
		Milestones: []DesignRoadmapMilestone{
			{Problem: "design-blob-storage", Blurb: "S3-class multipart uploads and etags."},
			{Problem: "design-event-streaming", Blurb: "Partitioned logs, offsets, and consumer groups."},
			{Problem: "design-search-index", Blurb: "Inverted indexes, segments, and shard merges."},
			{Problem: "design-distributed-kv", Blurb: "Sharding, replication tiers, and rebalancing."},
			{Problem: "design-payment-ledger", Blurb: "Append-only money — WAL instincts in product form."},
		},
	},
	{
		Slug:        "scale-and-traffic",
		Name:        "Scale & traffic",
		Tagline:     "Feeds, video, caches, notifications — when reads dominate.",
		Description: "Products where latency, fan-out, and CDN strategy matter more than consensus algorithms. Good follow-up after coordination builds.",
		Outcomes: []string{
			"Quantify read vs write QPS on every diagram",
			"Layer caches without stale-user surprises",
			"Push work async without losing user trust",
		},
		Milestones: []DesignRoadmapMilestone{
			{Problem: "design-realtime-feed", Blurb: "Timelines and hybrid fan-out."},
			{Problem: "design-distributed-cache", Blurb: "Stampedes, eviction, and sharded hot keys."},
			{Problem: "design-video-streaming", Blurb: "Upload pipelines and CDN segment delivery."},
			{Problem: "design-notification-system", Blurb: "Multi-channel fan-out with preferences."},
			{Problem: "design-api-gateway", Blurb: "Admission control before backends melt."},
		},
	},
	{
		Slug:        "distributed-core",
		Name:        "Distributed core",
		Tagline:     "Consensus, placement, scheduling, and coordination services.",
		Description: "The infrastructure layer under everything else — who is leader, where keys live, when work runs, and how clients watch metadata.",
		Outcomes: []string{
			"Map Raft and leases to concrete APIs",
			"Explain rebalancing without downtime",
			"Separate orchestration from execution",
		},
		Milestones: []DesignRoadmapMilestone{
			{Problem: "design-coordination-service", Blurb: "etcd-style linearizable metadata + watches."},
			{Problem: "design-distributed-kv", Blurb: "Partition leaders and consistency knobs."},
			{Problem: "design-distributed-scheduler", Blurb: "Leases, retries, and recurring jobs."},
			{Problem: "design-workflow-platform", Blurb: "Histories, replay, and long-running work."},
			{Problem: "design-event-streaming", Blurb: "Durable logs as the nervous system."},
		},
	},
	{
		Slug:        "platform-engineering",
		Name:        "Platform engineering",
		Tagline:     "Multi-tenant SaaS, gateways, and glue between teams.",
		Description: "Designing the layer other engineers build on — tenant isolation, quotas, notifications, and workflow platforms.",
		Outcomes: []string{
			"Enforce tenant boundaries at every hop",
			"Meter usage without slowing the hot path",
			"Split control plane from data plane",
		},
		Milestones: []DesignRoadmapMilestone{
			{Problem: "design-multi-tenant-saas", Blurb: "Isolation models and noisy neighbors."},
			{Problem: "design-api-gateway", Blurb: "The public contract + rate limits."},
			{Problem: "design-notification-system", Blurb: "Event → channel delivery pipelines."},
			{Problem: "design-workflow-platform", Blurb: "Durable orchestration for product teams."},
			{Problem: "design-distributed-scheduler", Blurb: "Internal cron and job infrastructure."},
		},
	},
	{
		Slug:        "full-curriculum",
		Name:        "Full whiteboard curriculum",
		Tagline:     "All 16 problems in a suggested order — the complete tour.",
		Description: "A long arc from familiar interview prompts through storage, scale, distributed primitives, and platform design. Budget several weeks.",
		Outcomes: []string{
			"Complete every open-crafters design scenario",
			"See how primitives recur across different products",
			"Pair each whiteboard with related build challenges",
		},
		Milestones: []DesignRoadmapMilestone{
			{Problem: "design-url-shortener", Blurb: "Start approachable — redirects and caching."},
			{Problem: "design-distributed-scheduler", Blurb: "Leases and delayed work."},
			{Problem: "design-notification-system", Blurb: "Async fan-out in practice."},
			{Problem: "design-realtime-feed", Blurb: "Read-heavy social scale."},
			{Problem: "design-chat-at-scale", Blurb: "Messaging ordering and delivery."},
			{Problem: "design-payment-ledger", Blurb: "Correctness under retries."},
			{Problem: "design-blob-storage", Blurb: "Object metadata + bytes."},
			{Problem: "design-event-streaming", Blurb: "Logs as a product."},
			{Problem: "design-search-index", Blurb: "Inverted indexes at scale."},
			{Problem: "design-distributed-cache", Blurb: "Ephemeral sharded cache."},
			{Problem: "design-distributed-kv", Blurb: "Durable partitioned KV."},
			{Problem: "design-coordination-service", Blurb: "Consensus as a service."},
			{Problem: "design-workflow-platform", Blurb: "Replay and histories."},
			{Problem: "design-api-gateway", Blurb: "Edge policy and routing."},
			{Problem: "design-video-streaming", Blurb: "Media pipelines + CDN."},
			{Problem: "design-multi-tenant-saas", Blurb: "Capstone — tenants, quotas, migration."},
		},
	},
}

// DesignRoadmapBySlug returns a design roadmap or false.
func DesignRoadmapBySlug(slug string) (DesignRoadmap, bool) {
	for _, r := range DesignRoadmaps {
		if r.Slug == slug {
			return r, true
		}
	}
	return DesignRoadmap{}, false
}

// DesignProblemsForRoadmap returns ordered design slugs for a roadmap.
func DesignProblemsForRoadmap(r DesignRoadmap) []string {
	out := make([]string, 0, len(r.Milestones))
	for _, m := range r.Milestones {
		out = append(out, m.Problem)
	}
	return out
}
