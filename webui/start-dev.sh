#!/bin/bash

# Development startup script for Web UI
# This script starts both the BFF and Frontend in the background

set -e

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
RED='\033[0;31m'
NC='\033[0m' # No Color

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo -e "${BLUE}Starting LLM Gateway Web UI Development Servers${NC}"
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
uvicorn app.main:app --reload --port 8000 > /tmp/bff.log 2>&1 &
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

# Start Frontend
echo -e "${BLUE}Starting Frontend (Vite) on port 5173...${NC}"
cd "$SCRIPT_DIR/frontend"

if [ ! -d "node_modules" ]; then
    echo -e "${BLUE}Installing pnpm dependencies...${NC}"
    pnpm install
fi

# Start frontend in background
pnpm run dev > /tmp/frontend.log 2>&1 &
FRONTEND_PID=$!
echo -e "${GREEN}✓ Frontend started (PID: $FRONTEND_PID)${NC}"
echo -e "  Logs: tail -f /tmp/frontend.log"
echo ""

# Wait for frontend to be ready
echo -e "${BLUE}Waiting for Frontend to be ready...${NC}"
for i in {1..30}; do
    if curl -s http://localhost:5173 > /dev/null 2>&1; then
        break
    fi
    sleep 0.5
done

echo ""
echo -e "${GREEN}════════════════════════════════════════════${NC}"
echo -e "${GREEN}✓ All services are running!${NC}"
echo -e "${GREEN}════════════════════════════════════════════${NC}"
echo ""
echo -e "  ${BLUE}Go Gateway:${NC} http://localhost:8080"
echo -e "  ${BLUE}BFF:${NC}        http://localhost:8000"
echo -e "  ${BLUE}Frontend:${NC}   http://localhost:5173"
echo ""
echo -e "${BLUE}Open your browser to:${NC}"
echo -e "  ${GREEN}http://localhost:5173${NC}"
echo ""
echo -e "${BLUE}To stop all services:${NC}"
echo -e "  kill $BFF_PID $FRONTEND_PID"
echo ""
echo -e "Press Ctrl+C to stop all services..."

# Trap Ctrl+C and cleanup
cleanup() {
    echo ""
    echo -e "${BLUE}Stopping services...${NC}"
    kill $BFF_PID 2>/dev/null || true
    kill $FRONTEND_PID 2>/dev/null || true
    echo -e "${GREEN}✓ All services stopped${NC}"
    exit 0
}

trap cleanup INT TERM

# Wait for user to press Ctrl+C
wait
