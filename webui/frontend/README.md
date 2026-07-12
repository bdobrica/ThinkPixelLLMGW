# LLM Gateway Admin UI

A minimal React + TypeScript admin interface for the LLM Gateway.

## Features

- **Cookie-based authentication**: All auth handled via BFF (no JWT in browser)
- **Clean, minimal UI**: Built with PicoCSS for a professional look
- **Protected routes**: Automatic redirect to login if not authenticated
- **Admin operations**: Manage API keys, models, and view billing

## Tech Stack

- React 18
- TypeScript
- Vite (dev server & build tool)
- React Router v6
- PicoCSS (minimal CSS framework)

## Setup

### 1. Install dependencies

```bash
cd webui/frontend
pnpm install
```

### 2. Run the dev server

```bash
pnpm run dev
```

The app will be available at `http://localhost:5173`.

### 3. Build for production

```bash
pnpm run build
```

The production build will be in the `dist/` directory.

### 4. Preview production build

```bash
pnpm run preview
```

## Architecture

The SPA:
- **Never** talks directly to the Go gateway
- All API calls go through the BFF at `/auth/*` and `/admin/*`
- Vite proxies these paths to `http://localhost:8000` (the BFF) during development
- Authentication is handled via HttpOnly cookies set by the BFF

## Available Routes

- `/login` - Login page (public)
- `/dashboard` - Admin dashboard (protected)
- `/api-keys` - Manage API keys (protected)
- `/models` - View models (protected)
- `/billing` - View billing info (protected)

## Project Structure

```
src/
├── api/
│   └── client.ts         # API client for BFF endpoints
├── components/
│   ├── Layout.tsx        # Main layout with navbar
│   ├── NavBar.tsx        # Navigation bar
│   └── ProtectedRoute.tsx # Auth guard for routes
├── pages/
│   ├── Login.tsx         # Login page
│   ├── Dashboard.tsx     # Dashboard
│   ├── ApiKeys.tsx       # API keys management
│   ├── Models.tsx        # Models (stub)
│   └── Billing.tsx       # Billing (stub)
├── App.tsx               # Root component
├── router.tsx            # Route configuration
└── main.tsx              # Entry point
```

## Development Notes

### Vite Proxy Configuration

The Vite dev server proxies API requests to the BFF:

- `/auth/*` → `http://localhost:8000`
- `/admin/*` → `http://localhost:8000`

This is configured in `vite.config.ts`.

### Authentication Flow

1. User submits login form
2. `POST /auth/login` → BFF validates credentials with Go gateway
3. BFF sets HttpOnly cookie with signed JWT
4. SPA redirects to `/dashboard`
5. Protected routes call `GET /auth/me` to verify auth
6. If auth fails (401), redirect to `/login`

### Adding New Features

To add a new admin page:

1. Create a new component in `src/pages/`
2. Add a route in `src/router.tsx`
3. Add a link in `src/components/NavBar.tsx`
4. Create corresponding endpoint in BFF if needed

## Production Deployment

For production, artifacts must be built before the launcher starts:

1. Build the frontend: `pnpm install --frozen-lockfile && pnpm run build`
2. Serve the `dist/` folder with a web server (nginx, caddy, etc.)
3. Configure the web server to:
   - Serve static files from `dist/`
   - Proxy `/auth/*` and `/admin/*` to the BFF
   - Serve `index.html` for all other routes (SPA mode)

The repository maintains one canonical configuration at `../nginx.conf.template`. `../start-prod.sh` renders it with configurable frontend root, BFF upstream, and UI listen address (default port 8081), validates it, and supervises nginx and the BFF. It requires the existing `dist` and BFF virtualenv artifacts and never installs or builds at production startup.
