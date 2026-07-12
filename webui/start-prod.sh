#!/usr/bin/env bash

# Launch pre-built Web UI artifacts. This script never installs dependencies or
# builds assets; use the documented build commands before deployment.

set -Eeuo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

GATEWAY_BASE_URL="${GATEWAY_BASE_URL:-http://127.0.0.1:8080}"
BFF_HOST="${BFF_HOST:-127.0.0.1}"
BFF_PORT="${BFF_PORT:-8000}"
WEBUI_LISTEN_ADDRESS="${WEBUI_LISTEN_ADDRESS:-127.0.0.1}"
WEBUI_PORT="${WEBUI_PORT:-8081}"
PUBLIC_ORIGIN="${PUBLIC_ORIGIN:-https://localhost:${WEBUI_PORT}}"
FRONTEND_ROOT="${FRONTEND_ROOT:-$SCRIPT_DIR/frontend/dist}"
BFF_VENV="${BFF_VENV:-$SCRIPT_DIR/bff/venv}"
NGINX_BIN="${NGINX_BIN:-nginx}"
NGINX_MIME_TYPES="${NGINX_MIME_TYPES:-/etc/nginx/mime.types}"
STARTUP_TIMEOUT_SECONDS="${STARTUP_TIMEOUT_SECONDS:-30}"

export ENVIRONMENT="${ENVIRONMENT:-production}"
export GATEWAY_BASE_URL
export COOKIE_SECURE="${COOKIE_SECURE:-true}"
export CORS_ORIGINS="${CORS_ORIGINS:-[\"$PUBLIC_ORIGIN\"]}"

die() {
    echo "Error: $*" >&2
    exit 1
}

command -v curl >/dev/null 2>&1 || die "curl is required"
command -v "$NGINX_BIN" >/dev/null 2>&1 || die "nginx is required; no production fallback is provided"
[[ -f "$FRONTEND_ROOT/index.html" ]] || die "pre-built frontend not found at $FRONTEND_ROOT (run pnpm build first)"
[[ -x "$BFF_VENV/bin/uvicorn" ]] || die "BFF virtualenv not found at $BFF_VENV (install requirements before deployment)"

curl --fail --silent --show-error "$GATEWAY_BASE_URL/ready" >/dev/null || \
    die "gateway is not ready at $GATEWAY_BASE_URL/ready"

NGINX_CONF="$(mktemp "${TMPDIR:-/tmp}/llmgw-webui-nginx.XXXXXX.conf")"
NGINX_PID_FILE="$(mktemp -u "${TMPDIR:-/tmp}/llmgw-webui-nginx.XXXXXX.pid")"
PIDS=()

cleanup() {
    local exit_status=$?
    trap - EXIT INT TERM
    if ((${#PIDS[@]})); then
        kill "${PIDS[@]}" 2>/dev/null || true
        wait "${PIDS[@]}" 2>/dev/null || true
    fi
    rm -f "$NGINX_CONF" "$NGINX_PID_FILE"
    exit "$exit_status"
}
trap cleanup EXIT INT TERM

escape_sed() {
    printf '%s' "$1" | sed 's/[&|\\]/\\&/g'
}

sed \
    -e "s|{{FRONTEND_ROOT}}|$(escape_sed "$FRONTEND_ROOT")|g" \
    -e "s|{{WEBUI_LISTEN}}|$(escape_sed "$WEBUI_LISTEN_ADDRESS:$WEBUI_PORT")|g" \
    -e "s|{{BFF_UPSTREAM}}|$(escape_sed "http://$BFF_HOST:$BFF_PORT")|g" \
    -e "s|{{NGINX_MIME_TYPES}}|$(escape_sed "$NGINX_MIME_TYPES")|g" \
    -e "s|{{NGINX_PID_FILE}}|$(escape_sed "$NGINX_PID_FILE")|g" \
    "$SCRIPT_DIR/nginx.conf.template" >"$NGINX_CONF"

"$NGINX_BIN" -t -c "$NGINX_CONF"

UVICORN_ARGS=(app.main:app --app-dir "$SCRIPT_DIR/bff" --host "$BFF_HOST" --port "$BFF_PORT")
if [[ "${TRUST_PROXY_HEADERS:-false}" == "true" ]]; then
    UVICORN_ARGS+=(--proxy-headers --forwarded-allow-ips "${TRUSTED_PROXY_IPS:-127.0.0.1}")
else
    UVICORN_ARGS+=(--no-proxy-headers)
fi

"$BFF_VENV/bin/uvicorn" "${UVICORN_ARGS[@]}" &
PIDS+=("$!")

deadline=$((SECONDS + STARTUP_TIMEOUT_SECONDS))
until curl --fail --silent "http://$BFF_HOST:$BFF_PORT/ready" >/dev/null; do
    kill -0 "${PIDS[0]}" 2>/dev/null || die "BFF exited during startup"
    ((SECONDS < deadline)) || die "BFF readiness timed out"
    sleep 1
done

"$NGINX_BIN" -c "$NGINX_CONF" -g "daemon off;" &
PIDS+=("$!")

echo "Web UI ready at $PUBLIC_ORIGIN"
echo "Gateway: $GATEWAY_BASE_URL | BFF: http://$BFF_HOST:$BFF_PORT | UI: $WEBUI_LISTEN_ADDRESS:$WEBUI_PORT"

set +e
wait -n "${PIDS[@]}"
child_status=$?
set -e
((child_status == 0)) || die "a Web UI child process exited with status $child_status"
die "a Web UI child process exited unexpectedly"
