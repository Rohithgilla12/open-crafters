# Stage 3: Time-ordered IDs

Snowflake IDs should sort in generation order — time dominates the high bits.

## Your task

Ensure sequential `next_id` calls (with a few milliseconds between them)
return **strictly increasing** decimal strings when compared as integers.

## What the tester checks

- 50 sequential `next_id` calls produce strictly increasing IDs.

## Notes

- You do not need wall-clock sleep inside the server — the tester pauses between
  calls. Focus on never moving the timestamp backwards and incrementing sequence
  within a millisecond.
