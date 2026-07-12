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

cd ../bff
pip install -r requirements-dev.txt
python3 -m pytest -q
```

## URLs

- UI: `http://localhost:5173`
- BFF health: `http://localhost:8000/health`
- BFF OpenAPI: `http://localhost:8000/docs`
- Gateway health: `http://localhost:8080/health`

## Configuration

`webui/bff/.env`:

```dotenv
ENVIRONMENT=development
GATEWAY_BASE_URL=http://localhost:8080
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

Billing is not implemented. The Models and Billing pages are placeholders.

## Troubleshooting

**Login always fails:** check gateway and BFF health, confirm the admin exists, inspect the BFF response/logs, and clear the `admin_token` cookie.

**BFF will not start in production:** replace the default/weak signing key and set `COOKIE_SECURE=true`.

**CORS error:** ensure `CORS_ORIGINS` is a JSON list containing the exact frontend origin. Development normally uses the Vite proxy and same-origin requests.

**`start-prod.sh` cannot bind port 8080:** this is a known bug because the gateway already owns that port. Use a separately configured reverse proxy/UI port; do not stop the gateway to make the script appear healthy.

**Models or billing show TODO text:** expected in the current implementation.

See [README.md](README.md), [IMPLEMENTATION_SUMMARY.md](IMPLEMENTATION_SUMMARY.md), and [../CODE_REVIEW.md](../CODE_REVIEW.md).
