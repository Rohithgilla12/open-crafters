# Stage 8: Durable offsets and resume

A consumer's whole crash-recovery story is "ask the log where I was." If
committed offsets evaporate on a broker restart, every consumer silently
reprocesses (or skips) data. And retention state must persist too, or offsets
stop meaning the same thing after a restart.

## Your task

Persist `commit_offset` and `truncate` alongside appends, so that after a
`SIGKILL` and restart:

- every group's committed offset is exactly what it was,
- the retained range (`start_offset`) is preserved,
- a consumer resumes reading correctly from its durable offset.

## Tests

Commit offsets for two groups, truncate a topic, then crash. After restart both
committed offsets are intact, `stats` shows the preserved retention, and a group
reads correctly from its recovered offset.

## Notes

- The simplest approach: append `commit` and `truncate` events to the same
  durable log as your records, and replay them all on boot.
- Replay order matters: apply appends, truncations, and commits in the order
  they happened so the recovered state matches exactly.
