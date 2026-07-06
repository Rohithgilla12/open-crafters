package opencrafters

// DesignBuildStep links a design problem to one build challenge with context.
type DesignBuildStep struct {
	Challenge string
	Blurb     string
}

// DesignBuildSteps maps design problem slug → ordered implementation path.
var DesignBuildSteps = map[string][]DesignBuildStep{
	"design-url-shortener": {
		{Challenge: "build-your-own-id-generator", Blurb: "Mint collision-free short codes from a monotonic counter."},
		{Challenge: "build-your-own-bloom-filter", Blurb: "Fast existence checks before hitting the code database."},
		{Challenge: "build-your-own-object-store", Blurb: "Archive click logs and cold link metadata at scale."},
	},
	"design-distributed-scheduler": {
		{Challenge: "build-your-own-scheduler", Blurb: "Core fire-time queue, leases, and recurring jobs."},
		{Challenge: "build-your-own-queue", Blurb: "At-least-once buffer between scheduler and workers."},
		{Challenge: "build-your-own-distributed-lock", Blurb: "Singleton cron leader or critical-section guards."},
	},
	"design-realtime-feed": {
		{Challenge: "build-your-own-log", Blurb: "Post ingestion stream for fan-out workers."},
		{Challenge: "build-your-own-rate-limiter", Blurb: "Throttle writes and abusive follow churn."},
		{Challenge: "build-your-own-bloom-filter", Blurb: "Skip feed shards that definitely lack a post."},
	},
	"design-workflow-platform": {
		{Challenge: "build-your-own-wal", Blurb: "Append-only event history before ack."},
		{Challenge: "build-your-own-temporal", Blurb: "Durable histories, timers, and task delivery."},
		{Challenge: "build-your-own-workflow-sdk", Blurb: "Deterministic replay from that history."},
	},
	"design-chat-at-scale": {
		{Challenge: "build-your-own-id-generator", Blurb: "Per-conversation sequence or global message IDs."},
		{Challenge: "build-your-own-log", Blurb: "Durable conversation append log."},
		{Challenge: "build-your-own-queue", Blurb: "Fan-out delivery to online devices."},
		{Challenge: "build-your-own-raft", Blurb: "Per-shard ordering and leader election."},
	},
	"design-payment-ledger": {
		{Challenge: "build-your-own-wal", Blurb: "Append entries before acknowledging transfers."},
		{Challenge: "build-your-own-id-generator", Blurb: "Idempotent transaction and idempotency keys."},
		{Challenge: "build-your-own-mvcc", Blurb: "Snapshot balance reads under concurrent writes."},
	},
	"design-notification-system": {
		{Challenge: "build-your-own-queue", Blurb: "Durable per-channel delivery jobs."},
		{Challenge: "build-your-own-scheduler", Blurb: "Digests, quiet hours, and delayed sends."},
		{Challenge: "build-your-own-rate-limiter", Blurb: "Per-user and per-provider caps."},
	},
	"design-distributed-kv": {
		{Challenge: "build-your-own-hash-ring", Blurb: "Key → partition placement and rebalancing."},
		{Challenge: "build-your-own-raft", Blurb: "Leader election and replicated log per shard."},
		{Challenge: "build-your-own-lsm", Blurb: "On-disk engine inside each partition."},
	},
	"design-api-gateway": {
		{Challenge: "build-your-own-rate-limiter", Blurb: "Per-key token buckets and sliding windows."},
		{Challenge: "build-your-own-hash-ring", Blurb: "Shard rate-limit counters or route canaries."},
		{Challenge: "build-your-own-distributed-lock", Blurb: "Config rollout leader election."},
	},
	"design-video-streaming": {
		{Challenge: "build-your-own-object-store", Blurb: "Multipart uploads and segment storage."},
		{Challenge: "build-your-own-queue", Blurb: "Transcode job delivery to workers."},
		{Challenge: "build-your-own-scheduler", Blurb: "Retry failed transcodes and publish windows."},
	},
	"design-search-index": {
		{Challenge: "build-your-own-log", Blurb: "CDC document change stream."},
		{Challenge: "build-your-own-lsm", Blurb: "Immutable inverted-index segments."},
		{Challenge: "build-your-own-bloom-filter", Blurb: "Skip segments that cannot contain a term."},
	},
	"design-multi-tenant-saas": {
		{Challenge: "build-your-own-mvcc", Blurb: "Tenant-scoped transactional rows."},
		{Challenge: "build-your-own-rate-limiter", Blurb: "Per-tenant API and storage quotas."},
		{Challenge: "build-your-own-id-generator", Blurb: "Opaque, non-guessable resource IDs."},
	},
	"design-event-streaming": {
		{Challenge: "build-your-own-wal", Blurb: "Durable append before producer ack."},
		{Challenge: "build-your-own-log", Blurb: "Topics, partitions, and consumer offsets."},
		{Challenge: "build-your-own-queue", Blurb: "Consumer group work distribution."},
	},
	"design-blob-storage": {
		{Challenge: "build-your-own-object-store", Blurb: "The product — buckets, etags, multipart."},
		{Challenge: "build-your-own-wal", Blurb: "Metadata commit before complete."},
		{Challenge: "build-your-own-hash-ring", Blurb: "Shard objects across storage nodes."},
	},
	"design-distributed-cache": {
		{Challenge: "build-your-own-hash-ring", Blurb: "Route keys to cache nodes."},
		{Challenge: "build-your-own-bloom-filter", Blurb: "Negative cache — key definitely absent."},
		{Challenge: "build-your-own-rate-limiter", Blurb: "Stampede protection via single-flight limits."},
	},
	"design-coordination-service": {
		{Challenge: "build-your-own-raft", Blurb: "Linearizable replicated metadata log."},
		{Challenge: "build-your-own-wal", Blurb: "Durability under the consensus log."},
		{Challenge: "build-your-own-distributed-lock", Blurb: "Leases, fencing tokens, ephemeral keys."},
	},
}

// BuildStepsForDesign returns ordered build steps for a design problem.
func BuildStepsForDesign(designSlug string) []DesignBuildStep {
	steps, ok := DesignBuildSteps[designSlug]
	if !ok {
		return nil
	}
	out := make([]DesignBuildStep, len(steps))
	copy(out, steps)
	return out
}

// DesignProblemsForChallenge returns design slugs that reference a build challenge.
func DesignProblemsForChallenge(challengeSlug string) []string {
	var out []string
	for _, slug := range DesignProblemOrder {
		for _, step := range DesignBuildSteps[slug] {
			if step.Challenge == challengeSlug {
				out = append(out, slug)
				break
			}
		}
	}
	return out
}
