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

Create a `.env` file in this directory (optional, defaults work for local dev):

```env
# Go gateway URL
GATEWAY_BASE_URL=http://localhost:8080

# Cookie signing secret (change in production!)
SECRET_KEY=change-this-to-a-secure-random-key-in-production

# Cookie name
COOKIE_NAME=admin_token

# Cookie max age (seconds)
COOKIE_MAX_AGE=3600

# CORS origins (comma-separated)
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
- Set `SECRET_KEY` to a secure random value
- Enable `secure=True` on cookies (requires HTTPS)
- Update `CORS_ORIGINS` to match your production frontend URL
- Consider adding rate limiting
- Use a reverse proxy (nginx/traefik) for SSL termination
