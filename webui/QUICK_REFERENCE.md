# Web UI quick reference

## Start development services

First run the Go gateway on `http://localhost:8080`. Then:

```bash
cd webui
./start-dev.sh
```

Or start the services manually:

```bash
# BFF: http://localhost:8000
cd webui/bff
python3 -m venv venv
source venv/bin/activate
pip install -r requirements.txt
uvicorn app.main:app --reload --port 8000

# UI: http://localhost:5173
cd ../frontend
pnpm install
pnpm run dev
```

## Validate changes

```bash
cd webui/frontend
pnpm run build
pnpm test

cd ../bff
pip install -r requirements-dev.txt
python3 -m pytest -q
```

## URLs

- UI: `http://localhost:5173`
- BFF health: `http://localhost:8000/health`
- BFF readiness: `http://localhost:8000/ready`
- BFF OpenAPI: `http://localhost:8000/docs`
- Gateway health: `http://localhost:8080/health`

## Launch production artifacts

Build/install once, then launch:

```bash
cd webui/frontend && pnpm install --frozen-lockfile && pnpm run build
cd ../bff && python3 -m venv venv && venv/bin/pip install -r requirements.txt
cd ..
SECRET_KEY="$(openssl rand -base64 32)" PUBLIC_ORIGIN=https://admin.example.com ./start-prod.sh
```

Defaults: gateway `:8080`, BFF `:8000`, UI `:8081`. Override with `GATEWAY_BASE_URL`, `BFF_HOST`/`BFF_PORT`, `WEBUI_LISTEN_ADDRESS`/`WEBUI_PORT`, `PUBLIC_ORIGIN`, `FRONTEND_ROOT`, and `BFF_VENV`. nginx is required and its configuration is validated before launch.

## Configuration

`webui/bff/.env`:

```dotenv
ENVIRONMENT=development
GATEWAY_BASE_URL=http://localhost:8080
GATEWAY_CONNECT_TIMEOUT=5
GATEWAY_READ_TIMEOUT=30
SECRET_KEY=replace-this-development-default
COOKIE_NAME=admin_token
COOKIE_PATH=/
COOKIE_SECURE=false
COOKIE_SAMESITE=strict
COOKIE_MAX_AGE=3600
CORS_ORIGINS=["http://localhost:5173"]
```

For production use `ENVIRONMENT=production`, `COOKIE_SECURE=true`, HTTPS, and a high-entropy `SECRET_KEY` of at least 32 characters. `COOKIE_NAME`, `COOKIE_PATH`, `COOKIE_DOMAIN`, `COOKIE_SAMESITE`, and `COOKIE_MAX_AGE` are supported. See `bff/.env.example`.

## Implemented routes

- `POST /auth/login`
- `POST /auth/logout`
- `GET /auth/me`
- `GET, POST /admin/api-keys`
- `PUT, DELETE /admin/api-keys/{id}`
- `GET /admin/models`
- `GET /admin/usage`
- `GET /admin/billing/monthly`
- `GET /admin/dashboard`

The Models page is a searchable, paginated, read-only catalog. Billing provides month selection, per-key costs/token totals, and recent usage; displayed costs are USD and request timestamps are UTC.

The dashboard defaults to the last 24 hours (selectable up to seven days), while its cost card explicitly covers the current UTC month. Rankings are capped at five models and five API keys.

`GET /auth/me` proxies the stable gateway `GET /admin/me` identity contract. The former production `/admin/test` route has been removed.

## Troubleshooting

**Login always fails:** check gateway and BFF health, confirm the admin exists, inspect the BFF response/logs, and clear the `admin_token` cookie.

**BFF will not start in production:** replace the default/weak signing key and set `COOKIE_SECURE=true`.

**BFF returns 502:** the gateway connection failed or its response was not valid JSON. Check `GATEWAY_BASE_URL` and gateway health/readiness.

**BFF returns 504:** the gateway exceeded a configured timeout; inspect gateway load and tune the specific timeout only after identifying the slow operation.

**`/health` is 200 but `/ready` is 503:** the BFF process is alive but the gateway is not ready, so the instance should not receive browser traffic.

**CORS error:** ensure `CORS_ORIGINS` is a JSON list containing the exact frontend origin. Development normally uses the Vite proxy and same-origin requests.

**Production launcher says artifacts are missing:** run the build/install commands above. Production startup intentionally never installs packages or builds the SPA.

**nginx preflight fails:** inspect the printed `nginx -t` error and verify `FRONTEND_ROOT`, `NGINX_MIME_TYPES`, listen address, and BFF address. There is intentionally no degraded static-server fallback.

**Models fail to load:** confirm the gateway migrations are current and the signed-in identity has at least the `viewer` role.

Before a release, run `pnpm run lint`, `pnpm run test`, and `pnpm run build`; then complete the repository [release qualification checklist](../docs/release-qualification.md).

**Billing totals lag recent requests:** check billing/usage worker readiness and dead-letter queues; the page reports persisted acknowledged records.

See [README.md](README.md), [IMPLEMENTATION_SUMMARY.md](IMPLEMENTATION_SUMMARY.md), and the [architecture decisions](../docs/adr/README.md).
