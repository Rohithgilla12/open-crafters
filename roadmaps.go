package opencrafters

import "strings"

// RoadmapMilestone is one step on a learning roadmap (maps to a challenge).
type RoadmapMilestone struct {
	Challenge string // challenge slug
	Blurb     string // why this step exists on the journey
}

// Roadmap is a curated learning journey with outcomes and ordered milestones.
// Challenge order comes from the linked path (PathSlug).
type Roadmap struct {
	Slug        string
	Name        string
	Tagline     string
	Description string
	Outcomes    []string
	PathSlug    string
	Milestones  []RoadmapMilestone
}

// Roadmaps are the learner-facing journeys. PathSlug must match ChallengePaths.
var Roadmaps = []Roadmap{
	{
		Slug:        "durability",
		Name:        "Durability & storage",
		Tagline:     "Make data survive crashes — from WALs to object stores.",
		Description: "Production databases, queues, and blob stores all lean on the same idea: acknowledge work only after it is durable. This roadmap walks that stack bottom-up.",
		Outcomes: []string{
			"Understand write-ahead logging and crash recovery",
			"Build at-least-once messaging with visibility timeouts",
			"Implement append-only logs with consumer offsets",
			"Design LSM-trees and MVCC transaction layers",
			"Ship a crash-safe object store with multipart uploads",
		},
		PathSlug: "durability",
		Milestones: []RoadmapMilestone{
			{Challenge: "build-your-own-wal", Blurb: "Start here — the byte-exact durability primitive under every database."},
			{Challenge: "build-your-own-queue", Blurb: "At-least-once delivery: persist before ack, fence with receipts."},
			{Challenge: "build-your-own-log", Blurb: "Partitioned append log — offsets, replay, and retention."},
			{Challenge: "build-your-own-lsm", Blurb: "Memtable + SSTables — how RocksDB-style engines flush and compact."},
			{Challenge: "build-your-own-mvcc", Blurb: "Snapshot isolation and multi-version reads on durable commits."},
			{Challenge: "build-your-own-object-store", Blurb: "Blob storage capstone — etags, listing, multipart, durability."},
		},
	},
	{
		Slug:        "workflow",
		Name:        "Workflow engines",
		Tagline:     "Orchestrate long-running work like Temporal.",
		Description: "Workflow systems split into a durable server (histories, timers, task queues) and a deterministic SDK that replays code against those histories. Build both sides.",
		Outcomes: []string{
			"Model workflow histories and task delivery",
			"Handle activities, timers, signals, and crash recovery",
			"Implement deterministic replay from event histories",
		},
		PathSlug: "workflow",
		Milestones: []RoadmapMilestone{
			{Challenge: "build-your-own-temporal", Blurb: "The server — durable histories, leases, and timer wheels."},
			{Challenge: "build-your-own-workflow-sdk", Blurb: "The worker SDK — same history in, same commands out, every time."},
		},
	},
	{
		Slug:        "distributed",
		Name:        "Distributed systems",
		Tagline:     "Consensus, placement, and probabilistic structures.",
		Description: "Scale-out infrastructure needs agreement on state, smart key placement, and compact membership tests. This track covers the three primitives.",
		Outcomes: []string{
			"Implement Raft leader election and replicated logs",
			"Route keys with consistent hashing and virtual nodes",
			"Build a Bloom filter with zero false negatives",
		},
		PathSlug: "distributed",
		Milestones: []RoadmapMilestone{
			{Challenge: "build-your-own-raft", Blurb: "3-node Raft — election, replication, partitions."},
			{Challenge: "build-your-own-hash-ring", Blurb: "Vnode placement — add/remove nodes without reshuffling everything."},
			{Challenge: "build-your-own-bloom-filter", Blurb: "Probabilistic membership — compact, fast, no false negatives."},
		},
	},
	{
		Slug:        "coordination",
		Name:        "Coordination & control",
		Tagline:     "Schedulers, limits, and locks between services.",
		Description: "Not everything is storage or consensus — production systems need delayed work, admission control, and mutual exclusion. This roadmap covers that glue.",
		Outcomes: []string{
			"Schedule delayed and recurring jobs with leases",
			"Rate-limit with token buckets and sliding windows",
			"Implement distributed locks with lease renewal",
		},
		PathSlug: "coordination",
		Milestones: []RoadmapMilestone{
			{Challenge: "build-your-own-scheduler", Blurb: "Durable delayed work — poll, lease, retry, recurring jobs."},
			{Challenge: "build-your-own-rate-limiter", Blurb: "Admission control — windows, buckets, per-key limits."},
			{Challenge: "build-your-own-distributed-lock", Blurb: "Exclusive locks — acquire, renew, survive crashes."},
		},
	},
	{
		Slug:        "platform",
		Name:        "Full platform roadmap",
		Tagline:     "All four tracks — the complete open-crafters journey.",
		Description: "A suggested order across every path: start with durability fundamentals, add workflow orchestration, then distributed primitives, and finish with coordination services.",
		Outcomes: []string{
			"Complete all 14 challenges across 4 thematic paths",
			"Understand how production infrastructure primitives compose",
		},
		PathSlug: "", // meta-roadmap — milestones are paths
		Milestones: []RoadmapMilestone{
			{Challenge: "__path:durability__", Blurb: "6 challenges — WAL through object store (recommended first)."},
			{Challenge: "__path:workflow__", Blurb: "2 challenges — Temporal server + replay SDK."},
			{Challenge: "__path:distributed__", Blurb: "3 challenges — Raft, hash ring, Bloom filter."},
			{Challenge: "__path:coordination__", Blurb: "3 challenges — scheduler, rate limiter, distributed lock."},
		},
	},
}

// RoadmapBySlug returns a roadmap definition or false.
func RoadmapBySlug(slug string) (Roadmap, bool) {
	for _, r := range Roadmaps {
		if r.Slug == slug {
			return r, true
		}
	}
	return Roadmap{}, false
}

// ChallengesForRoadmap returns ordered challenge slugs for a roadmap.
func ChallengesForRoadmap(r Roadmap) []string {
	if r.PathSlug != "" {
		for _, p := range ChallengePaths {
			if p.Slug == r.PathSlug {
				return append([]string(nil), p.Challenges...)
			}
		}
	}
	var out []string
	for _, m := range r.Milestones {
		if len(m.Challenge) > 7 && m.Challenge[:7] == "__path:" && strings.HasSuffix(m.Challenge, "__") {
			pathSlug := m.Challenge[7 : len(m.Challenge)-2]
			for _, p := range ChallengePaths {
				if p.Slug == pathSlug {
					out = append(out, p.Challenges...)
				}
			}
		}
	}
	return out
}
