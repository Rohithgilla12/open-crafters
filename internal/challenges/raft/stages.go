package raft

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/Rohithgilla12/open-crafters/internal/harness"
)

const (
	clusterSize   = 3
	pollInterval  = 100 * time.Millisecond
	electionWait  = 5 * time.Second
	commitWait    = 3 * time.Second
	partitionWait = 2 * time.Second
)

type Status struct {
	NodeID      string `json:"node_id"`
	Role        string `json:"role"`
	Term        int    `json:"term"`
	LeaderID    string `json:"leader_id"`
	CommitIndex int    `json:"commit_index"`
	LastApplied int    `json:"last_applied"`
}

func Challenge() harness.Challenge {
	docs := "challenges/build-your-own-raft/stages/"
	return harness.Challenge{
		Slug:        "build-your-own-raft",
		Name:        "Build your own Raft",
		ClusterSize: clusterSize,
		Stages: []harness.Stage{
			{Slug: "bind", Name: "Boot the cluster", Difficulty: "easy", Instructions: docs + "01-bind.md", TestCluster: testBind},
			{Slug: "leader", Name: "Leader election", Difficulty: "medium", Instructions: docs + "02-leader.md", TestCluster: testLeader},
			{Slug: "replicate", Name: "Replicate a write", Difficulty: "medium", Instructions: docs + "03-replicate.md", TestCluster: testReplicate},
			{Slug: "read", Name: "Read your writes", Difficulty: "easy", Instructions: docs + "04-read.md", TestCluster: testRead},
			{Slug: "follower-crash", Name: "Survive a follower crash", Difficulty: "medium", Instructions: docs + "05-follower-crash.md", TestCluster: testFollowerCrash},
			{Slug: "leader-crash", Name: "Survive a leader crash", Difficulty: "hard", Instructions: docs + "06-leader-crash.md", TestCluster: testLeaderCrash},
			{Slug: "durability", Name: "Survive a full crash", Difficulty: "hard", Instructions: docs + "07-durability.md", TestCluster: testDurability},
			{Slug: "partition", Name: "Partition safety", Difficulty: "hard", Instructions: docs + "08-partition.md", TestCluster: testPartition},
			{Slug: "gauntlet", Name: "The gauntlet", Difficulty: "hard", Instructions: docs + "09-gauntlet.md", TestCluster: testGauntlet},
		},
	}
}

func ping(c *harness.Client, wantNodeID string) error {
	var res struct {
		Message string `json:"message"`
		NodeID  string `json:"node_id"`
	}
	if err := c.Call("ping", nil, &res); err != nil {
		return err
	}
	if res.Message != "pong" {
		return fmt.Errorf(`ping: expected message "pong", got %q`, res.Message)
	}
	if res.NodeID != wantNodeID {
		return fmt.Errorf(`ping: expected node_id %q, got %q`, wantNodeID, res.NodeID)
	}
	return nil
}

func getStatus(c *harness.Client) (*Status, error) {
	var res Status
	if err := c.Call("get_status", nil, &res); err != nil {
		return nil, err
	}
	return &res, nil
}

func expectRPCError(err error, code string, context string) error {
	if err == nil {
		return fmt.Errorf("%s: expected error code %q, but call succeeded", context, code)
	}
	var rpcErr *harness.RPCError
	if !errors.As(err, &rpcErr) {
		return fmt.Errorf("%s: expected protocol error %q, got: %v", context, code, err)
	}
	if rpcErr.Code != code {
		return fmt.Errorf("%s: expected error code %q, got %q (%s)", context, code, rpcErr.Code, rpcErr.Message)
	}
	return nil
}

func waitForStableLeader(ctx *harness.ClusterContext) (int, error) {
	deadline := time.Now().Add(electionWait)
	for time.Now().Before(deadline) {
		leaderID, ok, err := currentLeader(ctx)
		if err != nil {
			return 0, err
		}
		if ok {
			return leaderID, nil
		}
		time.Sleep(pollInterval)
	}
	return 0, fmt.Errorf("no stable leader elected within %s — check election timeouts (need ≥ 300ms)", electionWait)
}

func currentLeader(ctx *harness.ClusterContext) (leaderID int, ok bool, err error) {
	var leaderNode int
	leaderCount := 0
	consensus := ""
	for id := 1; id <= ctx.ClusterSize(); id++ {
		running, err := ctx.IsRunning(id)
		if err != nil {
			return 0, false, err
		}
		if !running {
			continue
		}
		c, err := ctx.Dial(id)
		if err != nil {
			return 0, false, err
		}
		st, err := getStatus(c)
		c.Close()
		if err != nil {
			return 0, false, fmt.Errorf("node %d get_status: %w", id, err)
		}
		if st.Role == "leader" {
			leaderCount++
			leaderNode = id
		}
		if consensus == "" {
			consensus = st.LeaderID
		} else if st.LeaderID != consensus && st.LeaderID != "0" && consensus != "0" {
			return 0, false, nil
		}
		if st.LeaderID != "0" {
			consensus = st.LeaderID
		}
	}
	if leaderCount != 1 {
		return 0, false, nil
	}
	if consensus == "" || consensus == "0" {
		return 0, false, nil
	}
	want, _ := strconv.Atoi(consensus)
	if want != leaderNode {
		return 0, false, nil
	}
	for id := 1; id <= ctx.ClusterSize(); id++ {
		running, err := ctx.IsRunning(id)
		if err != nil {
			return 0, false, err
		}
		if !running {
			continue
		}
		c, err := ctx.Dial(id)
		if err != nil {
			return 0, false, err
		}
		st, err := getStatus(c)
		c.Close()
		if err != nil {
			return 0, false, err
		}
		if st.LeaderID != consensus && st.LeaderID != "0" {
			return 0, false, nil
		}
	}
	return leaderNode, true, nil
}

func setOnLeader(ctx *harness.ClusterContext, key string, value any) (int, error) {
	leaderID, err := waitForStableLeader(ctx)
	if err != nil {
		return 0, err
	}
	c, err := ctx.Dial(leaderID)
	if err != nil {
		return 0, err
	}
	defer c.Close()
	var res struct {
		Index int `json:"index"`
	}
	if err := c.Call("set", map[string]any{"key": key, "value": value}, &res); err != nil {
		return 0, fmt.Errorf("set on leader node %d: %w", leaderID, err)
	}
	if res.Index < 1 {
		return 0, fmt.Errorf("set on leader node %d: expected index >= 1, got %d", leaderID, res.Index)
	}
	return leaderID, nil
}

func getKey(c *harness.Client, key string) (found bool, value any, err error) {
	var res struct {
		Found bool `json:"found"`
		Value any  `json:"value"`
	}
	if err := c.Call("get", map[string]any{"key": key}, &res); err != nil {
		return false, nil, err
	}
	return res.Found, res.Value, nil
}

func waitForCommitIndex(ctx *harness.ClusterContext, want int) error {
	deadline := time.Now().Add(commitWait)
	for time.Now().Before(deadline) {
		ok := true
		for id := 1; id <= ctx.ClusterSize(); id++ {
			running, err := ctx.IsRunning(id)
			if err != nil {
				return err
			}
			if !running {
				continue
			}
			c, err := ctx.Dial(id)
			if err != nil {
				return err
			}
			st, err := getStatus(c)
			c.Close()
			if err != nil {
				return err
			}
			if st.CommitIndex < want {
				ok = false
				break
			}
		}
		if ok {
			return nil
		}
		time.Sleep(pollInterval)
	}
	return fmt.Errorf("commit_index did not reach %d on all nodes within %s", want, commitWait)
}

func jsonEq(a, b any) bool {
	ja, errA := json.Marshal(a)
	jb, errB := json.Marshal(b)
	return errA == nil && errB == nil && string(ja) == string(jb)
}

func testBind(ctx *harness.ClusterContext) error {
	ctx.Logf("pinging all %d nodes", ctx.ClusterSize())
	for id := 1; id <= ctx.ClusterSize(); id++ {
		running, err := ctx.IsRunning(id)
		if err != nil {
			return err
		}
		if !running {
			continue
		}
		c, err := ctx.Dial(id)
		if err != nil {
			return err
		}
		if err := ping(c, fmt.Sprint(id)); err != nil {
			c.Close()
			return fmt.Errorf("node %d: %w", id, err)
		}
		c.Close()
	}
	ctx.Logf("all nodes answered ping with correct node_id")
	return nil
}

func testLeader(ctx *harness.ClusterContext) error {
	ctx.Logf("waiting for a stable leader election")
	leaderID, err := waitForStableLeader(ctx)
	if err != nil {
		return err
	}
	ctx.Logf("node %d is the sole leader", leaderID)
	return nil
}

func testReplicate(ctx *harness.ClusterContext) error {
	ctx.Logf("writing key=replicate via Raft")
	if _, err := setOnLeader(ctx, "replicate", "ok"); err != nil {
		return err
	}
	if err := waitForCommitIndex(ctx, 1); err != nil {
		return err
	}
	ctx.Logf("all nodes reached commit_index >= 1")
	return nil
}

func testRead(ctx *harness.ClusterContext) error {
	if _, err := setOnLeader(ctx, "foo", "bar"); err != nil {
		return err
	}
	ctx.Logf("reading foo from every node")
	for id := 1; id <= ctx.ClusterSize(); id++ {
		c, err := ctx.Dial(id)
		if err != nil {
			return err
		}
		found, val, err := getKey(c, "foo")
		c.Close()
		if err != nil {
			return fmt.Errorf("node %d get: %w", id, err)
		}
		if !found {
			return fmt.Errorf("node %d: expected key foo to exist after committed set", id)
		}
		if val != "bar" {
			return fmt.Errorf("node %d: expected foo=bar, got %v", id, val)
		}
	}
	ctx.Logf("all nodes return the committed value")
	return nil
}

func testFollowerCrash(ctx *harness.ClusterContext) error {
	leaderID, err := waitForStableLeader(ctx)
	if err != nil {
		return err
	}
	follower := 1
	for id := 1; id <= ctx.ClusterSize(); id++ {
		if id != leaderID {
			follower = id
			break
		}
	}
	ctx.Logf("killing follower node %d", follower)
	if err := ctx.KillNode(follower); err != nil {
		return err
	}
	if _, err := setOnLeader(ctx, "survive", 42); err != nil {
		return err
	}
	ctx.Logf("cluster still committed a write with one follower down")
	return nil
}

func testLeaderCrash(ctx *harness.ClusterContext) error {
	if _, err := setOnLeader(ctx, "before", "crash"); err != nil {
		return err
	}
	leaderID, err := waitForStableLeader(ctx)
	if err != nil {
		return err
	}
	ctx.Logf("killing leader node %d", leaderID)
	if err := ctx.KillNode(leaderID); err != nil {
		return err
	}
	newLeader, err := waitForStableLeader(ctx)
	if err != nil {
		return err
	}
	ctx.Logf("node %d elected as new leader", newLeader)
	if _, err := setOnLeader(ctx, "after", "recovery"); err != nil {
		return err
	}
	c, err := ctx.Dial(newLeader)
	if err != nil {
		return err
	}
	found, val, err := getKey(c, "before")
	c.Close()
	if err != nil {
		return err
	}
	if !found || val != "crash" {
		return fmt.Errorf("after leader crash: expected before=crash to survive, got found=%v value=%v", found, val)
	}
	ctx.Logf("new leader serves prior committed data and accepts new writes")
	return nil
}

func testDurability(ctx *harness.ClusterContext) error {
	if _, err := setOnLeader(ctx, "durable", map[string]any{"x": 1}); err != nil {
		return err
	}
	ctx.Logf("killing all nodes")
	for id := 1; id <= ctx.ClusterSize(); id++ {
		if err := ctx.KillNode(id); err != nil {
			return err
		}
	}
	ctx.Logf("restarting all nodes with the same data dirs")
	for id := 1; id <= ctx.ClusterSize(); id++ {
		if err := ctx.RestartNode(id); err != nil {
			return fmt.Errorf("restart node %d: %w", id, err)
		}
	}
	if _, err := waitForStableLeader(ctx); err != nil {
		return err
	}
	c, err := ctx.Dial(1)
	if err != nil {
		return err
	}
	found, val, err := getKey(c, "durable")
	c.Close()
	if err != nil {
		return err
	}
	if !found || !jsonEq(val, map[string]any{"x": float64(1)}) {
		return fmt.Errorf("after full restart: expected durable={x:1}, got found=%v value=%v", found, val)
	}
	ctx.Logf("committed state survived total cluster crash")
	return nil
}

func testPartition(ctx *harness.ClusterContext) error {
	leaderID, err := waitForStableLeader(ctx)
	if err != nil {
		return err
	}
	isolated := leaderID
	ctx.Logf("partitioning leader node %d from the rest of the cluster", isolated)
	for id := 1; id <= ctx.ClusterSize(); id++ {
		if id == isolated {
			continue
		}
		if err := ctx.Partition(isolated, id); err != nil {
			return err
		}
	}
	defer ctx.Heal()

	c, err := ctx.Dial(isolated)
	if err != nil {
		return err
	}
	setErr := c.Call("set", map[string]any{"key": "split", "value": "brain"}, nil)
	c.Close()
	if expectRPCError(setErr, "NOT_COMMITTED", "set on isolated leader") != nil {
		if expectRPCError(setErr, "NOT_LEADER", "set on isolated leader") != nil {
			return fmt.Errorf("isolated leader set should fail with NOT_COMMITTED or NOT_LEADER, got: %v", setErr)
		}
	}
	ctx.Logf("isolated leader rejected or failed the write")

	time.Sleep(partitionWait)
	if _, err := setOnLeader(ctx, "majority", "wins"); err != nil {
		return fmt.Errorf("majority partition should commit a write: %w", err)
	}
	ctx.Logf("majority partition elected a leader and committed")
	return nil
}

func testGauntlet(ctx *harness.ClusterContext) error {
	ctx.Logf("gauntlet: writes, crash, restart")
	if _, err := setOnLeader(ctx, "g1", 1); err != nil {
		return err
	}
	if err := ctx.KillNode(2); err != nil {
		return err
	}
	if _, err := setOnLeader(ctx, "g2", 2); err != nil {
		return err
	}
	if err := ctx.RestartNode(2); err != nil {
		return err
	}
	if _, err := waitForStableLeader(ctx); err != nil {
		return err
	}
	for _, key := range []string{"g1", "g2"} {
		c, err := ctx.Dial(3)
		if err != nil {
			return err
		}
		found, _, err := getKey(c, key)
		c.Close()
		if err != nil {
			return err
		}
		if !found {
			return fmt.Errorf("gauntlet: key %q missing after churn", key)
		}
	}
	ctx.Logf("gauntlet passed")
	return nil
}
