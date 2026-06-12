# Stage 3: Survive a crash

Here's the promise that defines a database: **if the server said OK, the
write exists — no matter what happens next.** In this stage the tester
starts holding you to it: it `SIGKILL`s your process (no shutdown handler,
no flushing on exit) and restarts it with the same `--data-dir`.

## Your task

Make acknowledged writes durable. After a kill + restart:

- every acknowledged `set` is readable,
- overwrites show the *latest* acknowledged value,
- deleted keys stay deleted,
- and writes made *after* a recovery survive the *next* crash too.

The rule that makes this work is **write-before-ack**: persist the operation
to disk *before* sending the response. An ack for data that's only in memory
is a lie waiting for a power cut.

## Tests

The tester writes (including an overwrite and a delete), kills, restarts,
verifies; then writes again, kills again, verifies again.

## Notes

- Any persistence strategy passes *this* stage — even rewriting a whole JSON
  file per write. Do it the simple way if you like; stage 4 will force the
  append-only log and you'll feel *why* it's the right design (O(1) appends
  vs rewriting the world on every set).
- About `fsync`: `SIGKILL` doesn't drop the OS page cache, so the tester
  can't physically catch a missing fsync — only a power cut can. Write your
  code as if the power can fail after any syscall: flush *and* fsync before
  acknowledging. This challenge grades everything it can observe; this part
  is the honor system, and the habit is the lesson.
