# LLM Gateway Web UI

The Web UI is a React 19/Vite single-page application backed by a small FastAPI BFF. The browser receives a signed HttpOnly cookie; the BFF extracts the gateway JWT and forwards it in an Authorization header to the Go admin API.

## Current status

| Area | Status |
|---|---|
| Email/password login, logout, protected routes | Implemented |
| API-key list/create/update/revoke UI | Implemented |
| Models BFF list endpoint | Implemented |
| Models page | Placeholder |
| Billing endpoint and page | Not implemented / placeholder |
| Dashboard statistics | Placeholder |
| Service-token login | Gateway supports it; Web UI does not expose it |

This UI is suitable for development but needs the production hardening listed in [the repository code review](../CODE_REVIEW.md), especially cookie configuration and the production launcher's port conflict.

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
GATEWAY_BASE_URL=http://localhost:8080
SECRET_KEY=replace-with-a-long-random-value
COOKIE_NAME=admin_token
COOKIE_MAX_AGE=3600
CORS_ORIGINS=["http://localhost:5173"]
```

Important current limitations:

- `SECRET_KEY` has a development default in code; always override it.
- Cookies are currently emitted with `Secure=false`.
- Changing `COOKIE_NAME` currently breaks protected endpoints because the cookie reader is hardcoded to `admin_token`.
- `start-prod.sh` currently tries to bind nginx to the gateway's port 8080 and therefore cannot run beside the expected local gateway. Do not use it unchanged.

## BFF endpoints

| Endpoint | Purpose |
|---|---|
| `GET /health` | BFF liveness only |
| `POST /auth/login` | Gateway login and cookie creation |
| `POST /auth/logout` | Delete the cookie |
| `GET /auth/me` | Return current JWT claims via gateway `/admin/test` |
| `GET, POST /admin/api-keys` | List/create keys |
| `PUT, DELETE /admin/api-keys/{id}` | Update/revoke a key |
| `GET /admin/models` | List models |

There is no BFF `/admin/billing` route today.

## Production deployment direction

Build static assets with `pnpm run build`, serve `frontend/dist` on a port/domain distinct from the gateway, and reverse-proxy `/auth` and `/admin` to a multi-worker BFF. Before deployment, fix/configure secure cookies, require a strong signing key, enable HTTPS, normalize upstream failures, and restrict CORS to the actual UI origin.

## More detail

- [Quick reference](QUICK_REFERENCE.md)
- [Implementation summary](IMPLEMENTATION_SUMMARY.md)
- [BFF documentation](bff/README.md)
- [Frontend documentation](frontend/README.md)
