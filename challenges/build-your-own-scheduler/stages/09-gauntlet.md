# Stage 9: The gauntlet

Integration under churn: schedule several jobs, cancel one, complete others,
survive a crash mid-flight.

Nothing new — the tester verifies the full lifecycle holds together.
---

<!-- crafters-stage-hint -->
## Stuck?

<details>
<summary><strong>Spoiler-free hint</strong></summary>

> **Hint:** Mix delayed jobs, leases, failures, retries, cancel, recurring, and crashes. Invariants: durable schedule, single-use lease tokens, at-least- once delivery until max attempts, time-based visibility.

Or run: <code>crafters hint scheduler --stage gauntlet</code>
</details>
<!-- /crafters-stage-hint -->
