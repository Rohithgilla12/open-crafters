# Stage 9: The gauntlet

Nothing new to implement — this stage verifies your cluster survives **write →
crash → restart** churn without losing committed data.

## Your task

Pass the full progression:

1. Commit write `g1`.
2. Kill node `2`.
3. Commit write `g2` on the remaining majority.
4. Restart node `2`.
5. Every key is readable on a live node after the cluster stabilizes.

## Notes

- Combines replication, crash recovery, and catch-up after a slow follower returns.
- If earlier stages were implemented with shortcuts, this stage catches them.
