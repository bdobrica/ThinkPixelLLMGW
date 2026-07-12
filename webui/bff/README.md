# LLM Gateway BFF (Backend-for-Frontend)

A minimal FastAPI service that sits between the React admin UI and the Go LLM Gateway.

## Features

- **Cookie-based authentication**: Stores admin JWTs in signed, HttpOnly cookies
- **Gateway proxy**: Forwards admin API requests to the Go gateway
- **No database**: Stateless service that relies on the Go gateway for all data

## Setup

### 1. Create a virtual environment

```bash
cd webui/bff
python -m venv venv
source venv/bin/activate  # On Windows: venv\Scripts\activate
```

### 2. Install dependencies

```bash
pip install -r requirements.txt
```

### 3. Configure environment variables

Copy `.env.example` to `.env` (defaults work for local development):

```env
ENVIRONMENT=development
GATEWAY_BASE_URL=http://localhost:8080
SECRET_KEY=change-this-to-a-secure-random-key-in-production
COOKIE_NAME=admin_token
COOKIE_PATH=/
COOKIE_SECURE=false
COOKIE_SAMESITE=strict
COOKIE_MAX_AGE=3600
CORS_ORIGINS=["http://localhost:5173"]
```

### 4. Run the server

```bash
uvicorn app.main:app --reload --host 0.0.0.0 --port 8000
```

The BFF will be available at `http://localhost:8000`.

## API Endpoints

### Authentication

- `POST /auth/login` - Login with email/password, sets cookie
- `POST /auth/logout` - Clears the auth cookie
- `GET /auth/me` - Get current admin user info (requires auth)

### Admin Proxies

All admin endpoints require authentication via cookie:

- `GET /admin/api-keys` - List API keys
- `GET /admin/models` - List models
- `GET /admin/billing` - Get billing information

## Architecture

The BFF:
1. Receives requests from the React SPA
2. Manages authentication via signed cookies
3. Proxies requests to the Go gateway with the JWT in the `Authorization` header
4. Returns responses to the SPA

The SPA never directly talks to the Go gateway and never sees the JWT - it's all handled via HttpOnly cookies.

## Development

For local development, ensure:
- The Go gateway is running on `http://localhost:8080`
- The React dev server is running on `http://localhost:5173` (Vite default)
- CORS is configured to allow the frontend origin

## Security Notes

For production:
- Set `ENVIRONMENT=production`; startup rejects a default, short, or low-entropy `SECRET_KEY`
- Set `COOKIE_SECURE=true` and terminate HTTPS before the browser
- Configure cookie name, path, optional domain, SameSite (`strict` or `lax`), and maximum age as needed
- Update `CORS_ORIGINS` to match your production frontend URL
- Consider adding rate limiting
- Use a reverse proxy (nginx/traefik) for SSL termination

Cross-site cookies (`SameSite=None`) are not accepted because the BFF does not yet issue CSRF tokens. Proxy headers are ignored by default. If they are required, set `TRUST_PROXY_HEADERS=true` and start uvicorn with `--proxy-headers --forwarded-allow-ips "$TRUSTED_PROXY_IPS"`, limiting the IP list to known proxies.

Run the BFF suite with `pip install -r requirements-dev.txt && python -m pytest -q`.
