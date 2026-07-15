# Stage 1: Boot the stack

The harness starts three **reference** services and your gateway. Implement `ping` on the gateway; the tester also pings queue, scheduler, and rate-limiter directly.

Read `QUEUE_ADDR`, `SCHEDULER_ADDR`, `RATE_LIMITER_ADDR` from the environment.
