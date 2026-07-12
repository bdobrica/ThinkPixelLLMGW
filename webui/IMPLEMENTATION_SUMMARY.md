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
- Dashboard showing current JWT claims
- API-key management, including the one-time plaintext-key response
- Models and billing placeholder pages
- Same-origin fetch client with `credentials: include`
- Vite development proxy for `/auth` and `/admin`

## Actual route mapping

| Browser/BFF route | Gateway route |
|---|---|
| `POST /auth/login` | `POST /admin/auth/login` |
| `GET /auth/me` | `GET /admin/test` |
| `GET, POST /admin/api-keys` | `GET, POST /admin/keys` |
| `PUT, DELETE /admin/api-keys/{id}` | `PUT, DELETE /admin/keys/{id}` |
| `GET /admin/models` | `GET /admin/models` |

## Known gaps

- Models, billing, and dashboard-statistics UI are unfinished.
- The gateway has no `/admin/billing` endpoint and the BFF route is commented out.
- The frontend client still contains an unused `getBilling()` call that would receive 404.
- BFF cookie/configuration tests are present; frontend tests are not yet present.
- Cross-site cookies are deliberately disallowed until a CSRF token mechanism exists; strict/lax same-site cookies are supported.
- `start-prod.sh` conflicts with the gateway on port 8080; the fallback server also lacks API proxying and SPA routing.

## Verification

- `pnpm run build`: passed
- `python3 -m compileall -q app`: passed
- `python3 -m pytest -q`: 14 passed (cookie/configuration, client lifecycle, failures, readiness, validation, and body limits)
- Runtime login/proxy flow: not exercised because the review environment did not run the dependency stack

For prioritized remediation, see [../CODE_REVIEW.md](../CODE_REVIEW.md) and [../TODO.md](../TODO.md).
