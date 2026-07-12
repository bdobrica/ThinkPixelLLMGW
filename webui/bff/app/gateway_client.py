"""Application-lifetime HTTP client for the Go LLM Gateway."""

from typing import Any

import httpx
from fastapi import HTTPException, status

from .config import settings


_client: httpx.AsyncClient | None = None


def build_gateway_client() -> httpx.AsyncClient:
    return httpx.AsyncClient(
        base_url=settings.gateway_base_url,
        timeout=httpx.Timeout(
            connect=settings.gateway_connect_timeout,
            read=settings.gateway_read_timeout,
            write=settings.gateway_write_timeout,
            pool=settings.gateway_pool_timeout,
        ),
        limits=httpx.Limits(
            max_connections=settings.gateway_max_connections,
            max_keepalive_connections=settings.gateway_max_keepalive_connections,
        ),
    )


async def start_gateway_client(client: httpx.AsyncClient | None = None) -> httpx.AsyncClient:
    """Install the one client shared by all requests for this application lifetime."""
    global _client
    if _client is not None:
        raise RuntimeError("gateway client already started")
    _client = client or build_gateway_client()
    return _client


async def close_gateway_client() -> None:
    """Close and remove the shared gateway client."""
    global _client
    client, _client = _client, None
    if client is not None:
        await client.aclose()


def get_gateway_client() -> httpx.AsyncClient:
    if _client is None:
        raise RuntimeError("gateway client is not started")
    return _client


def upstream_error_detail(data: dict[str, Any] | None, fallback: str) -> str:
    """Extract only an expected, client-safe error string from an upstream body."""
    if data:
        for key in ("detail", "error", "message"):
            value = data.get(key)
            if isinstance(value, str) and value.strip():
                return value
    return fallback


async def gateway_request(
    method: str,
    path: str,
    jwt_token: str | None = None,
    json_data: dict[str, Any] | None = None,
    params: dict[str, Any] | None = None,
) -> tuple[int, dict[str, Any] | None]:
    """Call the gateway and translate transport/protocol failures consistently."""
    headers = {}
    if jwt_token:
        headers["Authorization"] = f"Bearer {jwt_token}"

    try:
        response = await get_gateway_client().request(
            method=method,
            url=path,
            headers=headers,
            json=json_data,
            params=params,
        )
    except httpx.TimeoutException as exc:
        raise HTTPException(
            status_code=status.HTTP_504_GATEWAY_TIMEOUT,
            detail="Gateway request timed out",
        ) from exc
    except httpx.RequestError as exc:
        raise HTTPException(
            status_code=status.HTTP_502_BAD_GATEWAY,
            detail="Gateway is unavailable",
        ) from exc

    if not response.content:
        return response.status_code, None
    try:
        data = response.json()
    except ValueError as exc:
        raise HTTPException(
            status_code=status.HTTP_502_BAD_GATEWAY,
            detail="Gateway returned an invalid response",
        ) from exc
    if not isinstance(data, dict):
        raise HTTPException(
            status_code=status.HTTP_502_BAD_GATEWAY,
            detail="Gateway returned an invalid response",
        )
    return response.status_code, data


async def gateway_is_ready() -> bool:
    """Return gateway readiness without leaking transport details."""
    try:
        response = await get_gateway_client().get("/ready")
        return response.status_code == status.HTTP_200_OK
    except httpx.RequestError:
        return False
