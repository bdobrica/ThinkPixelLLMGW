# Web UI implementation summary

Reviewed against the source on July 12, 2026.

## Design

```text
React/Vite browser
  └─ credentials: include; signed HttpOnly cookie
       └─ FastAPI BFF (:8000)
            └─ Authorization: Bearer <gateway JWT>
                 └─ Go gateway admin API (:8080)
```

The BFF is stateless and has no database. It signs the gateway JWT before storing it in a cookie; the Go gateway remains responsible for JWT signature and role validation.

## Implemented files and behavior

### BFF

- `app/main.py`: FastAPI lifespan, request-size limit, CORS, liveness/readiness, and router registration
- `app/config.py`: environment-backed gateway, validated production signing, explicit cookie, proxy-trust, and CORS settings
- `app/security.py`: timed itsdangerous signing and verification
- `app/dependencies.py`: authentication-cookie dependency
- `app/gateway_client.py`: reusable bounded async connection pool with stable transport/protocol error translation
- `app/auth.py`: login, logout, and current-admin endpoints
- `app/admin.py`: API-key CRUD proxy routes and model listing

### Frontend

- React Router protected/public route structure
- Login and logout flow
- Dashboard with current identity, bounded resource/usage/error/latency statistics, current-month USD cost, and top-five rankings
- API-key management, including the one-time plaintext-key response
- Searchable, paginated read-only model catalog with lifecycle, capability, limit, and pricing hierarchy
- Monthly billing summaries and recent usage view with explicit UTC/USD semantics
- Same-origin fetch client with `credentials: include`
- Vite development proxy for `/auth` and `/admin`
- Production launcher with distinct `8080` gateway, `8000` BFF, and `8081` UI defaults
- One configurable nginx template with SPA fallback, BFF proxying, and immutable static caching
- Pre-built artifact enforcement, nginx preflight validation, readiness gating, child-failure propagation, and signal cleanup

## Actual route mapping

| Browser/BFF route | Gateway route |
|---|---|
| `POST /auth/login` | `POST /admin/auth/login` |
| `GET /auth/me` | `GET /admin/me` |
| `GET, POST /admin/api-keys` | `GET, POST /admin/keys` |
| `PUT, DELETE /admin/api-keys/{id}` | `PUT, DELETE /admin/keys/{id}` |
| `GET /admin/models` | `GET /admin/models` |
| `GET /admin/usage` | `GET /admin/usage` |
| `GET /admin/billing/monthly` | `GET /admin/billing/monthly` |
| `GET /admin/dashboard` | `GET /admin/dashboard` |

## Known gaps

- Model create/edit/delete is intentionally API-only in this release; the UI is read-only for viewers and administrators.
- BFF cookie/configuration and model proxy tests are present, along with model-page component tests.
- Cross-site cookies are deliberately disallowed until a CSRF token mechanism exists; strict/lax same-site cookies are supported.

## Verification

- `pnpm run build`: passed
- `python3 -m compileall -q app`: passed
- `python3 -m pytest -q`: 17 passed (including the current-administrator gateway contract)
- `bash -n webui/start-prod.sh`: passed
- Runtime login/proxy flow: not exercised because the review environment did not run the dependency stack

For prioritized remediation, see [../CODE_REVIEW.md](../CODE_REVIEW.md) and [../TODO.md](../TODO.md).
