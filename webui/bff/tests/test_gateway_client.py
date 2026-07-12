"""Gateway client lifecycle, transport, and BFF proxy behavior tests."""

from unittest.mock import AsyncMock, patch

import httpx
import pytest
from fastapi import HTTPException
from httpx import ASGITransport, AsyncClient, MockTransport, Request, Response

from app.config import settings
from app.dependencies import get_current_admin_token
from app.gateway_client import (
    close_gateway_client,
    gateway_request,
    get_gateway_client,
    start_gateway_client,
)
from app.main import app
from app.admin import CreateAPIKeyRequest, create_api_key


async def installed_client(handler) -> httpx.AsyncClient:
    client = httpx.AsyncClient(
        base_url="http://gateway.test", transport=MockTransport(handler)
    )
    await start_gateway_client(client)
    return client


@pytest.fixture(autouse=True)
async def clean_gateway_client():
    await close_gateway_client()
    yield
    await close_gateway_client()


@pytest.mark.anyio
async def test_client_reuse_jwt_forwarding_and_shutdown() -> None:
    requests: list[Request] = []

    def handler(request: Request) -> Response:
        requests.append(request)
        return Response(200, json={"ok": True})

    client = await installed_client(handler)
    assert await gateway_request("GET", "/one", jwt_token="jwt") == (
        200,
        {"ok": True},
    )
    assert await gateway_request("GET", "/two") == (200, {"ok": True})
    assert requests[0].headers["authorization"] == "Bearer jwt"
    assert len(requests) == 2
    assert get_gateway_client() is client

    await close_gateway_client()
    assert client.is_closed
    with pytest.raises(RuntimeError, match="not started"):
        get_gateway_client()


@pytest.mark.anyio
@pytest.mark.parametrize(
    ("exception", "expected_status", "expected_detail"),
    [
        (httpx.ConnectError("refused"), 502, "Gateway is unavailable"),
        (httpx.ReadTimeout("slow"), 504, "Gateway request timed out"),
    ],
)
async def test_transport_failures_are_stable(
    exception: httpx.RequestError, expected_status: int, expected_detail: str
) -> None:
    def handler(request: Request) -> Response:
        exception.request = request
        raise exception

    await installed_client(handler)
    with pytest.raises(HTTPException) as caught:
        await gateway_request("GET", "/admin/keys")
    assert caught.value.status_code == expected_status
    assert caught.value.detail == expected_detail


@pytest.mark.anyio
async def test_malformed_upstream_json_is_502() -> None:
    await installed_client(lambda _: Response(200, text="not-json"))
    with pytest.raises(HTTPException) as caught:
        await gateway_request("GET", "/admin/keys")
    assert caught.value.status_code == 502
    assert caught.value.detail == "Gateway returned an invalid response"


@pytest.mark.anyio
@pytest.mark.parametrize(
    ("upstream_status", "upstream_body", "expected_detail"),
    [
        (409, {"error": "API key already exists"}, "API key already exists"),
        (503, {"detail": "Gateway temporarily unavailable"}, "Gateway temporarily unavailable"),
    ],
)
async def test_upstream_error_status_and_safe_detail_are_preserved(
    upstream_status: int, upstream_body: dict, expected_detail: str
) -> None:
    with patch(
        "app.admin.gateway_request",
        new=AsyncMock(return_value=(upstream_status, upstream_body)),
    ):
        with pytest.raises(HTTPException) as caught:
            await create_api_key(
                jwt_token="jwt",
                payload=CreateAPIKeyRequest(name="duplicate"),
            )
    assert caught.value.status_code == upstream_status
    assert caught.value.detail == expected_detail


@pytest.mark.anyio
async def test_liveness_is_independent_from_gateway_readiness() -> None:
    with patch("app.main.gateway_is_ready", new=AsyncMock(return_value=False)):
        async with AsyncClient(
            transport=ASGITransport(app=app), base_url="http://test"
        ) as client:
            health = await client.get("/health")
            ready = await client.get("/ready")
    assert health.status_code == 200
    assert health.json() == {"status": "ok"}
    assert ready.status_code == 503
    assert ready.json() == {"status": "unavailable", "gateway": "unavailable"}


@pytest.mark.anyio
async def test_mutation_validation_and_body_limit() -> None:
    async def authenticated() -> str:
        return "jwt"

    app.dependency_overrides[get_current_admin_token] = authenticated
    try:
        async with AsyncClient(
            transport=ASGITransport(app=app), base_url="http://test"
        ) as client:
            invalid = await client.post(
                "/admin/api-keys", json={"unexpected": "field"}
            )
            oversized = await client.post(
                "/admin/api-keys",
                content=b"x" * (settings.max_request_body_bytes + 1),
                headers={"content-type": "application/json"},
            )
        assert invalid.status_code == 422
        assert oversized.status_code == 413
        assert oversized.json() == {"detail": "Request body too large"}
    finally:
        app.dependency_overrides.clear()
