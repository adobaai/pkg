# Rate Limiting Middleware

## Rate Limit Headers

When rate limiting occurs, the middleware sets standard HTTP headers:

- `Retry-After`: Seconds to wait before retrying
- `X-RateLimit-Limit`: Configured burst for the resolved strategy (auth/cookie/ip)
- `X-RateLimit-Remaining`: Approximate remaining requests (estimated)

`X-RateLimit-Reset` is not emitted.

## Design Decisions

### Capacity Management

Uses **otter** (`github.com/maypok86/otter/v2`) with bounded size and expiry:

- **Memory bounded**: `MaximumSize` enforces upper bound.
- **Auto-expiry**: `ExpiryAccessing` removes inactive entries after `StaleAfter`.

Anonymous IP identity is resolved from `X-Real-IP` first, then `RemoteAddr`.

### Store Behavior

- `Len()` is exact but O(n), intended for tests/debugging.
- `EstimateMemoryBytes()` uses cache estimated size (O(1)).
- `Close()` invalidates entries and stops background goroutines.

### Middleware Requirements

- A non-nil `UserIDExtractor` must be provided.
- If extractor is nil, middleware returns an explicit internal error instead of panicking.
