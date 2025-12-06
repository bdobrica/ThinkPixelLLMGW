#!/bin/bash

# Production startup script for Web UI
# This script builds the frontend and starts nginx + BFF

set -e

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo -e "${BLUE}Starting LLM Gateway Web UI (Production Mode)${NC}"
echo ""

# Check if Go gateway is running
echo -e "${BLUE}Checking if Go gateway is running on localhost:8080...${NC}"
if ! curl -s http://localhost:8080/health > /dev/null 2>&1; then
    echo -e "${RED}Error: Go gateway not running on localhost:8080${NC}"
    echo -e "${RED}Please start it first:${NC}"
    echo -e "  cd llm_gateway && make run"
    exit 1
fi
echo -e "${GREEN}✓ Go gateway is running${NC}"
echo ""

# Build Frontend
echo -e "${BLUE}Building Frontend (React)...${NC}"
cd "$SCRIPT_DIR/frontend"

if [ ! -d "node_modules" ]; then
    echo -e "${BLUE}Installing pnpm dependencies...${NC}"
    pnpm install
fi

echo -e "${BLUE}Running production build...${NC}"
pnpm run build

if [ ! -d "dist" ]; then
    echo -e "${RED}Error: Build failed, dist directory not found${NC}"
    exit 1
fi

echo -e "${GREEN}✓ Frontend built successfully${NC}"
echo ""

# Start BFF
echo -e "${BLUE}Starting BFF (FastAPI) on port 8000...${NC}"
cd "$SCRIPT_DIR/bff"

if [ ! -d "venv" ]; then
    echo -e "${BLUE}Creating Python virtual environment...${NC}"
    python3 -m venv venv
fi

source venv/bin/activate
pip install -q -r requirements.txt

# Start BFF in background
uvicorn app.main:app --host 0.0.0.0 --port 8000 > /tmp/bff.log 2>&1 &
BFF_PID=$!
echo -e "${GREEN}✓ BFF started (PID: $BFF_PID)${NC}"
echo -e "  Logs: tail -f /tmp/bff.log"
echo ""

# Wait for BFF to be ready
echo -e "${BLUE}Waiting for BFF to be ready...${NC}"
for i in {1..30}; do
    if curl -s http://localhost:8000/health > /dev/null 2>&1; then
        break
    fi
    sleep 0.5
done

if ! curl -s http://localhost:8000/health > /dev/null 2>&1; then
    echo -e "${RED}Error: BFF failed to start${NC}"
    kill $BFF_PID 2>/dev/null || true
    exit 1
fi
echo -e "${GREEN}✓ BFF is ready${NC}"
echo ""

# Check if nginx is installed
if ! command -v nginx &> /dev/null; then
    echo -e "${YELLOW}Warning: nginx not found${NC}"
    echo -e "${YELLOW}Please install nginx first:${NC}"
    echo -e "  Ubuntu/Debian: sudo apt-get install nginx"
    echo -e "  macOS: brew install nginx"
    echo -e ""
    echo -e "${YELLOW}Falling back to Python static file server...${NC}"
    cd "$SCRIPT_DIR/frontend/dist"
    python3 -m http.server 8080 > /tmp/static-server.log 2>&1 &
    STATIC_PID=$!
    echo -e "${GREEN}✓ Static server started on port 8080 (PID: $STATIC_PID)${NC}"
    echo -e "${YELLOW}Note: API requests won't be proxied. Use nginx for full functionality.${NC}"
    echo ""
    
    echo -e "${GREEN}════════════════════════════════════════════${NC}"
    echo -e "${GREEN}✓ Services running (fallback mode)${NC}"
    echo -e "${GREEN}════════════════════════════════════════════${NC}"
    echo ""
    echo -e "  ${BLUE}Go Gateway:${NC} http://localhost:8080"
    echo -e "  ${BLUE}BFF:${NC}        http://localhost:8000"
    echo -e "  ${BLUE}Frontend:${NC}   http://localhost:8080"
    echo ""
    echo -e "${YELLOW}Warning: This mode doesn't proxy /auth and /admin to BFF${NC}"
    echo -e "${YELLOW}Install nginx for production use${NC}"
    echo ""
    echo -e "${BLUE}To stop all services:${NC}"
    echo -e "  kill $BFF_PID $STATIC_PID"
    
    # Trap Ctrl+C and cleanup
    cleanup() {
        echo ""
        echo -e "${BLUE}Stopping services...${NC}"
        kill $BFF_PID 2>/dev/null || true
        kill $STATIC_PID 2>/dev/null || true
        echo -e "${GREEN}✓ All services stopped${NC}"
        exit 0
    }
    
    trap cleanup INT TERM
    wait
    exit 0
fi

# Generate nginx config
echo -e "${BLUE}Generating nginx configuration...${NC}"
NGINX_CONF="$SCRIPT_DIR/nginx.conf"

cat > "$NGINX_CONF" << 'EOF'
worker_processes auto;
daemon off;

events {
    worker_connections 1024;
}

http {
    include /etc/nginx/mime.types;
    default_type application/octet-stream;

    sendfile on;
    keepalive_timeout 65;
    gzip on;
    gzip_types text/plain text/css application/json application/javascript text/xml application/xml application/xml+rss text/javascript;

    access_log /tmp/nginx-access.log;
    error_log /tmp/nginx-error.log;

    server {
        listen 8080;
        server_name localhost;

        # Root directory for static files
        root FRONTEND_DIST_PATH;
        index index.html;

        # Proxy API requests to BFF
        location ~ ^/(auth|admin) {
            proxy_pass http://localhost:8000;
            proxy_set_header Host $host;
            proxy_set_header X-Real-IP $remote_addr;
            proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
            proxy_set_header X-Forwarded-Proto $scheme;
            
            # Required for cookies
            proxy_set_header Cookie $http_cookie;
            proxy_pass_header Set-Cookie;
        }

        # SPA fallback - serve index.html for all routes
        location / {
            try_files $uri $uri/ /index.html;
        }

        # Cache static assets
        location ~* \.(js|css|png|jpg|jpeg|gif|ico|svg|woff|woff2|ttf|eot)$ {
            expires 1y;
            add_header Cache-Control "public, immutable";
        }
    }
}
EOF

# Replace placeholder with actual path
sed -i "s|FRONTEND_DIST_PATH|$SCRIPT_DIR/frontend/dist|g" "$NGINX_CONF"

echo -e "${GREEN}✓ nginx configuration created${NC}"
echo ""

# Start nginx
echo -e "${BLUE}Starting nginx on port 8080...${NC}"
nginx -c "$NGINX_CONF" > /tmp/nginx.log 2>&1 &
NGINX_PID=$!

# Wait for nginx to be ready
sleep 1
if ! curl -s http://localhost:8080 > /dev/null 2>&1; then
    echo -e "${RED}Error: nginx failed to start${NC}"
    echo -e "${RED}Check logs: tail -f /tmp/nginx-error.log${NC}"
    kill $BFF_PID 2>/dev/null || true
    exit 1
fi

echo -e "${GREEN}✓ nginx started (PID: $NGINX_PID)${NC}"
echo -e "  Access log: tail -f /tmp/nginx-access.log"
echo -e "  Error log: tail -f /tmp/nginx-error.log"
echo ""

echo -e "${GREEN}════════════════════════════════════════════${NC}"
echo -e "${GREEN}✓ All services are running!${NC}"
echo -e "${GREEN}════════════════════════════════════════════${NC}"
echo ""
echo -e "  ${BLUE}Go Gateway:${NC} http://localhost:8080 (admin API)"
echo -e "  ${BLUE}BFF:${NC}        http://localhost:8000 (internal)"
echo -e "  ${BLUE}Web UI:${NC}     http://localhost:8080"
echo ""
echo -e "${BLUE}Open your browser to:${NC}"
echo -e "  ${GREEN}http://localhost:8080${NC}"
echo ""
echo -e "${BLUE}To stop all services:${NC}"
echo -e "  kill $BFF_PID $NGINX_PID"
echo ""
echo -e "Press Ctrl+C to stop all services..."

# Trap Ctrl+C and cleanup
cleanup() {
    echo ""
    echo -e "${BLUE}Stopping services...${NC}"
    kill $BFF_PID 2>/dev/null || true
    kill $NGINX_PID 2>/dev/null || true
    echo -e "${GREEN}✓ All services stopped${NC}"
    exit 0
}

trap cleanup INT TERM

# Wait for user to press Ctrl+C
wait
