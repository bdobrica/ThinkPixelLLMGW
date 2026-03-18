# Web UI Implementation Summary

This document summarizes the new Web UI components added to the LLM Gateway project.

## What Was Added

### Directory Structure

```
webui/
├── README.md                    # Main documentation for Web UI
├── bff/                         # Backend-for-Frontend (Python/FastAPI)
│   ├── .env.example            # Example environment variables
│   ├── .gitignore              # Python gitignore
│   ├── README.md               # BFF documentation
│   ├── requirements.txt        # Python dependencies
│   └── app/
│       ├── __init__.py
│       ├── main.py             # FastAPI app entry point
│       ├── config.py           # Configuration management
│       ├── security.py         # Cookie signing/verification
│       ├── gateway_client.py   # HTTP client for Go gateway
│       ├── dependencies.py     # FastAPI dependencies (auth)
│       ├── auth.py             # Auth endpoints (/auth/login, /auth/logout, /auth/me)
│       └── admin.py            # Admin proxy endpoints (/admin/*)
└── frontend/                    # React SPA (TypeScript/Vite)
    ├── .gitignore              # Node gitignore
    ├── README.md               # Frontend documentation
    ├── package.json            # NPM dependencies
    ├── tsconfig.json           # TypeScript config
    ├── tsconfig.node.json      # TypeScript config for Vite
    ├── vite.config.ts          # Vite config with BFF proxy
    ├── index.html              # HTML entry point (includes PicoCSS CDN)
    └── src/
        ├── main.tsx            # React entry point
        ├── App.tsx             # Root component
        ├── router.tsx          # Route configuration
        ├── api/
        │   └── client.ts       # API client for BFF
        ├── components/
        │   ├── Layout.tsx      # Main layout with navbar
        │   ├── NavBar.tsx      # Navigation bar
        │   └── ProtectedRoute.tsx  # Auth guard
        └── pages/
            ├── Login.tsx       # Login page
            ├── Dashboard.tsx   # Dashboard
            ├── ApiKeys.tsx     # API Keys management
            ├── Models.tsx      # Models page (stub)
            └── Billing.tsx     # Billing page (stub)
```

## Key Features

### BFF (Backend-for-Frontend)

- **Framework**: FastAPI with uvicorn
- **Authentication**: Signed HttpOnly cookies using `itsdangerous`
- **Security**: 
  - JWT never exposed to browser JavaScript
  - HMAC-signed cookies to prevent tampering
  - CORS configured for frontend origin only
  - Configurable cookie settings (secure, samesite, max-age)
- **Endpoints**:
  - `POST /auth/login` - Authenticate with email/password
  - `POST /auth/logout` - Clear auth cookie
  - `GET /auth/me` - Get current user info
  - `GET /admin/api-keys` - List API keys (proxy to gateway)
  - `GET /admin/models` - List models (proxy to gateway)
  - `GET /admin/billing` - Get billing info (proxy to gateway)
- **Configuration**: Environment-based with sensible defaults
- **No Database**: Stateless proxy that relies entirely on the Go gateway

### Frontend (React SPA)

- **Framework**: React 18 + TypeScript + Vite
- **Styling**: PicoCSS via CDN (minimal, semantic CSS)
- **Routing**: React Router v6 with protected routes
- **Authentication**: 
  - Cookie-based (no JWT handling in browser)
  - Automatic redirect to login on 401
  - Auth check on protected routes
- **Pages**:
  - `/login` - Email/password login form
  - `/dashboard` - User info and stats
  - `/api-keys` - List API keys with pagination
  - `/models` - Models page (stub)
  - `/billing` - Billing page (stub)
- **API Client**: Simple fetch wrapper with error handling
- **Dev Server**: Vite with proxy to BFF for `/auth` and `/admin` paths

## Authentication Flow

```
1. User enters email/password on /login page
   ↓
2. Frontend POSTs to /auth/login (proxied to BFF)
   ↓
3. BFF calls Go Gateway POST /admin/login
   ↓
4. Gateway returns JWT
   ↓
5. BFF signs JWT and stores in HttpOnly cookie
   ↓
6. BFF returns success to frontend
   ↓
7. Frontend redirects to /dashboard
   ↓
8. Dashboard calls /auth/me to get user info
   ↓
9. BFF verifies cookie, extracts JWT, calls Gateway /admin/me
   ↓
10. User info displayed on dashboard
```

## Quick Start

### Terminal 1: Start the Go Gateway
```bash
cd llm_gateway
make run
```

### Terminal 2: Start the BFF
```bash
cd webui/bff
python -m venv venv
source venv/bin/activate  # Windows: venv\Scripts\activate
pip install -r requirements.txt
uvicorn app.main:app --reload --port 8000
```

### Terminal 3: Start the Frontend
```bash
cd webui/frontend
npm install
npm run dev
```

### Access
Open http://localhost:5173 in your browser and login with your admin credentials.

## Configuration

### BFF Environment Variables

Create `webui/bff/.env`:

```env
GATEWAY_BASE_URL=http://localhost:8080
SECRET_KEY=change-this-to-a-secure-random-key-in-production
COOKIE_NAME=admin_token
COOKIE_MAX_AGE=3600
CORS_ORIGINS=["http://localhost:5173"]
```

### Frontend Proxy

The Vite dev server automatically proxies API requests:

- `/auth/*` → `http://localhost:8000`
- `/admin/*` → `http://localhost:8000`

This is configured in `webui/frontend/vite.config.ts`.

## Production Deployment

### Build Frontend
```bash
cd webui/frontend
npm run build
# Output: ./dist
```

### Run BFF
```bash
cd webui/bff
pip install -r requirements.txt
uvicorn app.main:app --host 0.0.0.0 --port 8000 --workers 4
```

### Serve with Nginx

```nginx
server {
    listen 443 ssl http2;
    server_name admin.example.com;
    
    root /path/to/webui/frontend/dist;
    
    # Proxy API to BFF
    location ~ ^/(auth|admin) {
        proxy_pass http://localhost:8000;
    }
    
    # SPA fallback
    location / {
        try_files $uri /index.html;
    }
}
```

## Design Decisions

1. **Why a BFF?**
   - Keep JWT out of browser (XSS protection)
   - Single source of truth for auth state
   - Easy to add request transformation/aggregation later
   - Simpler frontend code (no token management)

2. **Why PicoCSS?**
   - Minimal footprint (< 10kb)
   - Semantic HTML (no className bloat)
   - Beautiful defaults
   - Perfect for internal admin UIs

3. **Why no state management library?**
   - Simple app with few shared state needs
   - React hooks are sufficient
   - Reduces bundle size and complexity

4. **Why signed cookies instead of sessions?**
   - Stateless BFF (no session store needed)
   - Works in distributed deployments
   - Still secure with proper signing

## Extending the UI

### Add a New Admin Page

1. Create `webui/frontend/src/pages/NewPage.tsx`
2. Add route in `webui/frontend/src/router.tsx`
3. Add nav link in `webui/frontend/src/components/NavBar.tsx`
4. If needed, add BFF endpoint in `webui/bff/app/admin.py`
5. Update API client in `webui/frontend/src/api/client.ts`

### Add a New BFF Endpoint

```python
# In webui/bff/app/admin.py

@router.get("/your-endpoint")
async def your_endpoint(
    jwt_token: Annotated[str, Depends(get_current_admin_token)],
):
    """Your endpoint description."""
    status_code, data = await gateway_request(
        method="GET",
        path="/admin/your-path",
        jwt_token=jwt_token,
    )
    
    if status_code != 200:
        raise HTTPException(status_code=status_code, detail="Error message")
    
    return data
```

## Testing

### BFF
```bash
cd webui/bff
pytest  # (add tests in app/tests/)
```

### Frontend
```bash
cd webui/frontend
npm run test  # (add tests with Vitest or Jest)
```

## Security Considerations

- **Production**: Set `SECRET_KEY` to a secure random value
- **Production**: Enable `secure=True` on cookies (requires HTTPS)
- **Production**: Update `CORS_ORIGINS` to production domain
- **Production**: Consider adding rate limiting on BFF auth endpoints
- **Production**: Use a reverse proxy (nginx/traefik) for SSL termination
- **Production**: Enable HTTP security headers (CSP, HSTS, etc.)

## What's NOT Included

To keep this minimal, these features are not included but could be added:

- User registration (assumes admins are created via CLI or direct DB)
- Password reset flow
- Multi-factor authentication
- User management UI (assumes admin of admins uses CLI)
- Audit logs viewer (logs go to S3, use external tools)
- Real-time metrics dashboard (use Prometheus/Grafana)
- API key creation UI (listed as TODO in ApiKeys page)
- Advanced filters/search on list pages
- Export/download features

These can be added later as needed.
