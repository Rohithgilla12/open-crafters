# Stage 9: The gauntlet

Everything at once, under concurrency.

## Your task

Survive concurrent connections across **two rings**: add, remove, lookup churn.
Protect shared state with a lock (or equivalent).

## What the tester checks

- Multiple connections run mixed operations in parallel.
- Final verification: recorded keys match the reference oracle.

## Notes

- Pure concurrency — no crash restart.
- The gauntlet is the last stage.
