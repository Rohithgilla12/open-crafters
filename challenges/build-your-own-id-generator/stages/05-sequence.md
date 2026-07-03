# Stage 5: Per-millisecond sequence

Within the same millisecond you can issue thousands of IDs — the low 12 bits
are a counter.

## Your task

When multiple IDs share the same `timestamp_ms`, increment `sequence` from `0`
up to `4095` without collision.

## What the tester checks

- 5000 rapid `next_id` calls — all unique, `sequence` always `0`–`4095`.

## Notes

- This is the heart of the snowflake: same millisecond → bump sequence; new
  millisecond → reset sequence to `0`.
