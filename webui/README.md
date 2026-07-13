# LLM Gateway Web UI

The Web UI is a React 19/Vite single-page application backed by a small FastAPI BFF. The browser receives a signed HttpOnly cookie; the BFF extracts the gateway JWT and forwards it in an Authorization header to the Go admin API.

## Current status

| Area | Status |
|---|---|
| Email/password login, logout, protected routes | Implemented |
| API-key list/create/update/revoke UI | Implemented |
| Models BFF list endpoint | Implemented |
| Models page | Implemented (read-only catalog) |
| Billing and usage endpoints/page | Implemented (read-only) |
| Dashboard statistics | Implemented |
| Service-token login | Gateway supports it; Web UI does not expose it |

Authentication cookies, gateway proxying, and the artifact-only production launcher are production-aware. Model mutation stays in the gateway admin API for this release; both viewer and admin roles may use the read-only catalog, billing views, and dashboard, while gateway mutations still require the admin role.

## Development

Prerequisites: a gateway at `http://localhost:8080`, Python 3.10+, Node.js 18+, and pnpm.

Automated (Linux/macOS):

```bash
cd webui
./start-dev.sh
```

Manual:

```bash
# terminal 1
cd webui/bff
python3 -m venv venv
source venv/bin/activate
pip install -r requirements.txt
uvicorn app.main:app --reload --port 8000

# terminal 2
cd webui/frontend
pnpm install
pnpm run dev
```

Open `http://localhost:5173`. Vite proxies `/auth` and `/admin` to the BFF.

## Configuration

Create `webui/bff/.env` as needed:

```dotenv
ENVIRONMENT=development
GATEWAY_BASE_URL=http://localhost:8080
GATEWAY_CONNECT_TIMEOUT=5
GATEWAY_READ_TIMEOUT=30
SECRET_KEY=replace-with-a-long-random-value
COOKIE_NAME=admin_token
COOKIE_PATH=/
COOKIE_SECURE=false
COOKIE_SAMESITE=strict
COOKIE_MAX_AGE=3600
CORS_ORIGINS=["http://localhost:5173"]
```

Copy [bff/.env.example](bff/.env.example) for the full list. In production, set `ENVIRONMENT=production`, a non-default high-entropy `SECRET_KEY` of at least 32 characters, and `COOKIE_SECURE=true`; invalid settings stop startup. Cookie name, path, optional domain, SameSite policy, and lifetime are configurable and used consistently for login, authentication, and logout.

Cross-site cookies are intentionally unsupported: `COOKIE_SAMESITE` accepts only `strict` or `lax`. This keeps cookie-authenticated mutation routes protected by same-site browser policy until a separate CSRF token mechanism is implemented. Proxy headers remain untrusted by default; enable uvicorn's `--proxy-headers --forwarded-allow-ips "$TRUSTED_PROXY_IPS"` only when `TRUST_PROXY_HEADERS=true` and the BFF is behind those known proxies.

The BFF reuses one bounded HTTP connection pool for its lifetime. Connect, read, write, and pool timeouts plus connection limits are configurable in `bff/.env.example`. Transport failures return stable `502` responses, timeouts return `504`, malformed upstream responses return `502`, and safe gateway error messages retain their upstream status. Mutation payloads are typed, reject unknown fields, and are limited by `MAX_REQUEST_BODY_BYTES`.

## BFF endpoints

| Endpoint | Purpose |
|---|---|
| `GET /health` | BFF liveness only |
| `GET /ready` | BFF and gateway readiness |
| `POST /auth/login` | Gateway login and cookie creation |
| `POST /auth/logout` | Delete the cookie |
| `GET /auth/me` | Return current identity via gateway `GET /admin/me` |
| `GET, POST /admin/api-keys` | List/create keys |
| `PUT, DELETE /admin/api-keys/{id}` | Update/revoke a key |
| `GET /admin/models` | Paginated model list; accepts `search` and `provider_id` |
| `GET /admin/usage` | Bounded, paginated request usage |
| `GET /admin/billing/monthly` | Monthly cost/token summaries by API key |
| `GET /admin/dashboard` | Fixed-query aggregate summary; `hours` is limited to 1–168 |

Billing and usage are read-only for both viewer and admin roles. Usage ranges are capped at 90 days and pages at 100 records. The UI labels UTC and USD explicitly; monthly costs include records acknowledged by the asynchronous workers.

## Production deployment

Build and install dependencies before startup:

```bash
cd webui/frontend && pnpm install --frozen-lockfile && pnpm run build
cd ../bff && python3 -m venv venv && venv/bin/pip install -r requirements.txt
cd ..
SECRET_KEY="$(openssl rand -base64 32)" PUBLIC_ORIGIN=https://admin.example.com ./start-prod.sh
```

The launcher defaults to gateway `127.0.0.1:8080`, BFF `127.0.0.1:8000`, and UI `127.0.0.1:8081`, so there is no port collision. Configure `GATEWAY_BASE_URL`, `BFF_HOST`, `BFF_PORT`, `WEBUI_LISTEN_ADDRESS`, `WEBUI_PORT`, `PUBLIC_ORIGIN`, `FRONTEND_ROOT`, or `BFF_VENV` when needed. It renders only [nginx.conf.template](nginx.conf.template), validates it with `nginx -t`, waits for readiness, propagates child failures, and cleans up both processes on termination. There is no non-proxying static-server fallback.

Terminate TLS at nginx or an upstream proxy, retain `COOKIE_SECURE=true`, and use `/ready` for traffic readiness while retaining `/health` for BFF process liveness. The launcher does not build assets or install packages, making deployed artifacts immutable.

## More detail

- [Quick reference](QUICK_REFERENCE.md)
- [Implementation summary](IMPLEMENTATION_SUMMARY.md)
- [BFF documentation](bff/README.md)
- [Frontend documentation](frontend/README.md)
