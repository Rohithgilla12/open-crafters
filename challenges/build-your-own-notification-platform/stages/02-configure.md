# Stage 2: Configure limits

Implement `configure_limit` — proxy to rate-limiter `configure` with key `user:<user_id>`, algorithm `fixed_window`, and the given `limit` / `window_ms`.
