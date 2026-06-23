# Stage 7: Signals in replay

Signals are external events injected into a running workflow. They appear in
history as `WORKFLOW_EXECUTION_SIGNALED`. Your replay engine must react when
that event is present.

## Your task

Implement **`signal_wait`** (see [PROTOCOL.md](../PROTOCOL.md)):

| Last event in history | Commands |
|---|---|
| `WORKFLOW_EXECUTION_STARTED` | `[]` (waiting for signal) |
| `WORKFLOW_EXECUTION_SIGNALED` with `signal_name: "go"` | `COMPLETE_WORKFLOW` with `result` = signal's `input` |

Example:

```
history: [STARTED, SIGNALED{signal_name:"go", input:{value:42}}]
→ COMPLETE_WORKFLOW{result:{value:42}}
```

## Notes

- Signals are just history events from the SDK's perspective — same as
  activity completions and timer fires.
- The workflow doesn't poll for signals; replay sees them in history when the
  worker task arrives.
