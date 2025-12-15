# LLM Gateway Web UI

A minimal web-based admin interface for the LLM Gateway, consisting of a React frontend and a Python BFF (Backend-for-Frontend).

## Components

### 1. Frontend (`./frontend`)

A minimal React + TypeScript SPA with:
- Cookie-based authentication (no JWT handling in browser)
- Clean UI using PicoCSS
- Protected routes for admin operations
- Pages for API keys, models, and billing

**Quick start:**
```bash
cd frontend
npm install
npm run dev
```

See [frontend/README.md](./frontend/README.md) for details.

### 2. BFF (`./bff`)

A FastAPI service that:
- Manages authentication via signed HttpOnly cookies
- Proxies admin API requests to the Go gateway
- Provides a clean REST API for the frontend
- No database - stateless service

**Quick start:**
```bash
cd bff
python -m venv venv
source venv/bin/activate  # Windows: venv\Scripts\activate
pip install -r requirements.txt
uvicorn app.main:app --reload
```

See [bff/README.md](./bff/README.md) for details.

## Architecture

```
┌─────────────┐
│   Browser   │
│  (React UI) │
└──────┬──────┘
       │ HTTP (cookies)
       ▼
┌─────────────┐
│     BFF     │
│  (FastAPI)  │
└──────┬──────┘
       │ HTTP + JWT
       ▼
┌─────────────┐
│ Go Gateway  │
│ (Admin API) │
└─────────────┘
```

**Authentication Flow:**
1. User logs in with email/password on the React UI
2. BFF calls Go gateway `/admin/login` and receives JWT
3. BFF signs the JWT and stores it in an HttpOnly cookie
4. All subsequent requests from the UI include the cookie
5. BFF verifies the cookie and forwards requests to the gateway with the JWT

**Security:**
- JWT is never exposed to browser JavaScript (HttpOnly cookies)
- Cookies are signed to prevent tampering
- CORS is configured to only allow the frontend origin
- In production, use HTTPS and set `secure=True` on cookies

## Development Setup

### Prerequisites

- Node.js 18+ (for frontend)
- Python 3.9+ (for BFF)
- The Go LLM Gateway running on `http://localhost:8080`

### Running Locally

#### Option 1: Development Mode (Vite Dev Server)

1. **Start the Go gateway** (from repo root):
   ```bash
   cd llm_gateway
   make run
   ```

2. **Start the BFF** (terminal 1):
   ```bash
   cd webui/bff
   python -m venv venv
   source venv/bin/activate
   pip install -r requirements.txt
   uvicorn app.main:app --reload --port 8000
   ```

3. **Start the frontend** (terminal 2):
   ```bash
   cd webui/frontend
   pnpm install
   pnpm run dev
   ```

4. **Access the UI**:
   - Open `http://localhost:5173` in your browser
   - Hot module reloading enabled
   - Login with your admin credentials

#### Option 2: Production Mode (nginx + Static Files)

1. **Start the Go gateway** (from repo root):
   ```bash
   cd llm_gateway
   make run
   ```

2. **Run the production startup script**:
   ```bash
   cd webui
   ./start-prod.sh
   ```

   This will:
   - Build the frontend to static files (`pnpm run build`)
   - Start the BFF on port 8000
   - Start nginx on port 8080 serving static files and proxying API requests
   - If nginx is not installed, falls back to Python's http.server

3. **Access the UI**:
   - Open `http://localhost:8080` in your browser
   - Production-optimized build
   - Login with your admin credentials

#### Automated Start (Development)

For development mode:
```bash
cd webui
./start-dev.sh
```

This automatically starts both BFF and Vite dev server.

## Production Deployment

### Build the Frontend

```bash
cd frontend
pnpm run build
```

The production build will be in `./frontend/dist`.

### Deploy

For production, you'll need:

1. **Serve the frontend** with a web server (nginx, caddy, etc.)
2. **Run the BFF** with a production WSGI server:
   ```bash
   pip install uvicorn[standard]
   uvicorn app.main:app --host 0.0.0.0 --port 8000 --workers 4
   ```
3. **Configure the web server** to:
   - Serve static files from `./frontend/dist`
   - Proxy `/auth/*` and `/admin/*` to the BFF
   - Use HTTPS for secure cookies

Example nginx config:

```nginx
server {
    listen 443 ssl http2;
    server_name admin.example.com;
    
    ssl_certificate /path/to/cert.pem;
    ssl_certificate_key /path/to/key.pem;
    
    root /path/to/webui/frontend/dist;
    index index.html;
    
    # Proxy API requests to BFF
    location ~ ^/(auth|admin) {
        proxy_pass http://localhost:8000;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
    
    # SPA fallback
    location / {
        try_files $uri $uri/ /index.html;
    }
}
```

### Environment Variables

For production, update these environment variables in the BFF:

```env
GATEWAY_BASE_URL=http://your-gateway-url:8080
SECRET_KEY=your-very-secure-random-secret-key-here
COOKIE_NAME=admin_token
COOKIE_MAX_AGE=3600
CORS_ORIGINS=["https://admin.example.com"]
```

See `./bff/.env.example` for a complete list.

## Development Notes

- The frontend dev server (Vite) proxies `/auth` and `/admin` requests to the BFF
- CORS is enabled on the BFF for local development
- API key signing uses `itsdangerous` library (production-ready)
- All authentication state is server-side (no localStorage/sessionStorage)

## Adding New Features

### Add a New Admin Page

1. Create component in `./frontend/src/pages/YourPage.tsx`
2. Add route in `./frontend/src/router.tsx`
3. Add navigation link in `./frontend/src/components/NavBar.tsx`
4. If needed, add BFF endpoint in `./bff/app/admin.py`

### Add a New BFF Endpoint

1. Add route handler in `./bff/app/admin.py` (or create new module)
2. Use `get_current_admin_token` dependency for auth
3. Call gateway using `gateway_request` helper
4. Update frontend API client in `./frontend/src/api/client.ts`

## Troubleshooting

### Frontend can't connect to BFF
- Check that BFF is running on `http://localhost:8000`
- Check Vite proxy config in `./frontend/vite.config.ts`
- Check browser console for CORS errors

### BFF can't connect to Gateway
- Check `GATEWAY_BASE_URL` in BFF config
- Verify the Go gateway is running
- Check BFF logs for connection errors

### Authentication not working
- Clear browser cookies and try again
- Check that `SECRET_KEY` is set in BFF
- Verify admin credentials are correct in the gateway database

## License

Same as the main LLM Gateway project.
