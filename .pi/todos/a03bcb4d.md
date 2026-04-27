{
  "id": "a03bcb4d",
  "title": "Phase 2: Login rate limiting",
  "tags": [
    "security",
    "should-fix",
    "phase-2"
  ],
  "status": "open",
  "created_at": "2026-04-27T15:10:04.383Z"
}

## Problem
No rate limiting on `/login` POST. Brute force attacks can try passwords at HTTP speed.

## Fix
- Add a simple in-memory rate limiter keyed by IP
- Use a `map[string][]time.Time` with a mutex, or a token bucket
- Config: 5 attempts per 60 seconds per IP
- Return 429 Too Many Requests with `Retry-After` header when exceeded
- Apply as a check inside `handleLoginSubmit`
- Cleanup stale entries periodically (goroutine with ticker, or lazy cleanup)

## Decision
→ IP-only. More in-depth limits will be handled at the reverse proxy layer later.

## Scope
- New file `internal/web/ratelimit.go` (~60 lines)
- Small change in `handlers.go`
