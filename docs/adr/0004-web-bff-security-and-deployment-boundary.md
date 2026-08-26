# ADR-0004: Web BFF security and deployment boundary

- Status: Accepted
- Date: 2026-07-12
- Owners: project maintainers
- Supersedes: none
- Superseded by: none

## Context

The browser UI needs cookie-based authentication while the gateway uses bearer credentials. Earlier production defaults allowed a known signing key, insecure cookies, inconsistent configurable cookie names, per-request upstream clients, unnormalized network failures, and conflicting gateway/UI ports.

## Decision

Retain a FastAPI backend-for-frontend (BFF) as the browser trust boundary:

- production mode requires a strong non-default signing secret and Secure cookies;
- cookie name, path, domain, SameSite, maximum age, and deletion attributes are configured consistently;
- trusted proxy headers are opt-in and source-scoped;
- one lifespan-owned bounded HTTP client proxies to the gateway and maps connection/timeout failures to stable `502`/`504` responses;
- BFF liveness is separate from gateway-backed readiness;
- mutation requests use typed schemas and a hard body limit.

The production launcher uses immutable pre-built assets, distinct gateway/BFF/UI ports (`8080`/`8000`/`8081` by default), one nginx template, configuration preflight, dependency readiness, signal-safe cleanup, and no production Python static-server fallback. The authenticated identity contract is `GET /admin/me`; the test-named legacy route was removed.

## Alternatives considered

- Store gateway bearer tokens directly in browser storage: rejected because it increases exposure to browser script compromise.
- Put cookie authentication directly in the Go gateway: rejected to keep the public API authentication contract independent of the browser UI.
- Start ad-hoc development servers in production: rejected because proxying, SPA routing, supervision, and immutable artifact behavior differ.

## Consequences

- Browser-specific cookie and proxy concerns do not leak into the gateway API.
- Cross-site cookie deployments remain constrained to Strict/Lax SameSite until a separate CSRF-token design is implemented.
- Deployment must terminate TLS at a trusted proxy and keep the BFF/private upstream boundary correctly configured.

## Security impact

The BFF holds the signed HttpOnly cookie boundary and forwards the gateway token server-side. Production requires TLS/Secure cookies and a strong signing secret. SameSite provides the current CSRF boundary; cross-site cookie support requires a separate CSRF decision.

## Operational impact

The BFF client pool and timeout controls bound upstream resource use. Deployments must route public UI traffic to nginx/BFF while keeping the gateway-facing BFF path and trusted proxy configuration private and correct.

## References and evidence

Implemented in commits `a5da4de`, `f8f4f45`, `cddcef4`, and `6561c9e`. See the Web UI README and quick reference for current configuration.
