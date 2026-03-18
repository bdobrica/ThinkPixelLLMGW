# Web UI Quick Reference

## 🚀 Quick Start

### Development Mode

#### Option 1: Automated (Linux/Mac)
```bash
cd webui
./start-dev.sh
```

#### Option 2: Manual

**Terminal 1 - BFF:**
```bash
cd webui/bff
python -m venv venv
source venv/bin/activate  # Windows: venv\Scripts\activate
pip install -r requirements.txt
uvicorn app.main:app --reload --port 8000
```

**Terminal 2 - Frontend:**
```bash
cd webui/frontend
pnpm install
pnpm run dev
```

**Access:** http://localhost:5173 (Vite dev server with HMR)

### Production Mode

#### With nginx (Recommended)
```bash
cd webui
./start-prod.sh
```

This will:
- Build frontend to static files
- Start BFF on port 8000
- Start nginx on port 8080
- Proxy /auth and /admin to BFF
- Serve static files from dist/

**Access:** http://localhost:8080 (Production build)

#### Manual Production Build
```bash
# Build frontend
cd webui/frontend
pnpm run build

# Serve with your preferred web server
# Static files in: webui/frontend/dist/
```

---

## 📁 File Structure

```
webui/
├── README.md                 # Main docs
├── IMPLEMENTATION_SUMMARY.md # Detailed implementation guide
├── start-dev.sh             # Auto-start script
├── bff/                     # Python FastAPI backend
│   ├── app/
│   │   ├── main.py         # FastAPI app
│   │   ├── auth.py         # Login/logout endpoints
│   │   ├── admin.py        # Admin proxy endpoints
│   │   ├── config.py       # Configuration
│   │   ├── security.py     # Cookie signing
│   │   └── gateway_client.py  # HTTP client
│   └── requirements.txt
└── frontend/               # React TypeScript SPA
    ├── src/
    │   ├── pages/          # Login, Dashboard, ApiKeys, etc.
    │   ├── components/     # NavBar, Layout, ProtectedRoute
    │   └── api/client.ts   # API client
    ├── package.json
    └── vite.config.ts
```

---

## 🔌 API Endpoints

### BFF Endpoints

**Auth:**
- `POST /auth/login` - Login with email/password
- `POST /auth/logout` - Logout
- `GET /auth/me` - Get current user

**Admin (all require auth cookie):**
- `GET /admin/api-keys?page=1&page_size=20` - List API keys
- `GET /admin/models?page=1&page_size=20` - List models
- `GET /admin/billing` - Get billing info

### Frontend Routes

- `/login` - Login page (public)
- `/dashboard` - User dashboard (protected)
- `/api-keys` - API key management (protected)
- `/models` - Models list (protected, stub)
- `/billing` - Billing info (protected, stub)

---

## 🔐 Authentication

**Flow:**
1. User logs in → BFF calls Go gateway
2. BFF receives JWT from gateway
3. BFF signs JWT and stores in HttpOnly cookie
4. All future requests include cookie automatically
5. BFF verifies cookie and proxies to gateway with JWT

**Security:**
- JWT never exposed to browser JavaScript
- Cookies are HttpOnly, Signed, and SameSite=Strict
- CORS restricted to frontend origin only

---

## ⚙️ Configuration

### BFF (`webui/bff/.env`)
```env
GATEWAY_BASE_URL=http://localhost:8080
SECRET_KEY=change-in-production
COOKIE_NAME=admin_token
COOKIE_MAX_AGE=3600
CORS_ORIGINS=["http://localhost:5173"]
```

### Frontend
No configuration needed for dev. Vite automatically proxies `/auth` and `/admin` to BFF.

---

## 🏗️ Production Build

### Build Frontend
```bash
cd webui/frontend
pnpm run build
# Output: ./dist/
```

### Run BFF in Production
```bash
cd webui/bff
pip install -r requirements.txt
uvicorn app.main:app --host 0.0.0.0 --port 8000 --workers 4
```

### Nginx Example
```nginx
server {
    listen 443 ssl;
    server_name admin.example.com;
    root /path/to/webui/frontend/dist;
    
    location ~ ^/(auth|admin) {
        proxy_pass http://localhost:8000;
    }
    
    location / {
        try_files $uri /index.html;
    }
}
```

---

## 🛠️ Development

### Add a New Page

1. Create `frontend/src/pages/NewPage.tsx`
2. Add route in `frontend/src/router.tsx`
3. Add link in `frontend/src/components/NavBar.tsx`
4. Add BFF endpoint if needed in `bff/app/admin.py`

### Add a BFF Endpoint

```python
@router.get("/new-endpoint")
async def new_endpoint(
    jwt_token: Annotated[str, Depends(get_current_admin_token)]
):
    status_code, data = await gateway_request(
        method="GET",
        path="/admin/your-path",
        jwt_token=jwt_token
    )
    if status_code != 200:
        raise HTTPException(status_code=status_code)
    return data
```

---

## 🐛 Troubleshooting

### Frontend won't start
```bash
cd webui/frontend
rm -rf node_modules pnpm-lock.yaml
pnpm install
pnpm run dev
```

### BFF won't start
```bash
cd webui/bff
rm -rf venv
python3 -m venv venv
source venv/bin/activate
pip install -r requirements.txt
uvicorn app.main:app --reload
```

### Can't login
- Check Go gateway is running: `curl http://localhost:8080/health`
- Check BFF can reach gateway: `curl http://localhost:8000/health`
- Check browser console for errors
- Clear browser cookies

### CORS errors
- Verify `CORS_ORIGINS` in BFF includes frontend URL
- Check Vite proxy config in `frontend/vite.config.ts`

---

## 📚 Documentation

- **Main README:** `webui/README.md`
- **BFF Docs:** `webui/bff/README.md`
- **Frontend Docs:** `webui/frontend/README.md`
- **Implementation Details:** `webui/IMPLEMENTATION_SUMMARY.md`

---

## 🔗 URLs (Development)

- **Go Gateway:** http://localhost:8080
- **BFF:** http://localhost:8000
- **Frontend:** http://localhost:5173
- **BFF Health:** http://localhost:8000/health
- **BFF Docs:** http://localhost:8000/docs (FastAPI auto-docs)

---

## 💡 Tips

- Use Chrome DevTools → Application → Cookies to inspect auth cookie
- Use Network tab to see BFF API calls
- BFF auto-reloads on code changes (FastAPI --reload)
- Frontend auto-reloads on code changes (Vite HMR)
- Check `/tmp/bff.log` and `/tmp/frontend.log` if using start-dev.sh
