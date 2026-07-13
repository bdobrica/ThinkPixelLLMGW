"""BFF-to-gateway current-administrator contract tests."""

from unittest.mock import AsyncMock, patch

import pytest

from app.auth import CurrentAdminResponse, me
from app.admin import list_models, list_monthly_billing, list_usage


@pytest.mark.anyio
async def test_current_admin_uses_gateway_admin_me_contract() -> None:
    upstream = {
        "admin_id": "admin-123",
        "auth_type": "user",
        "roles": ["admin", "viewer"],
        "email": "admin@example.test",
    }
    request = AsyncMock(return_value=(200, upstream))
    with patch("app.auth.gateway_request", new=request):
        response = await me(jwt_token="gateway-jwt")

    request.assert_awaited_once_with(
        method="GET", path="/admin/me", jwt_token="gateway-jwt"
    )
    assert CurrentAdminResponse.model_validate(response).model_dump(
        exclude_none=True
    ) == upstream


@pytest.mark.anyio
async def test_model_list_forwards_bounded_filters() -> None:
    upstream = {"items": [], "total_count": 0, "page": 2, "page_size": 12}
    request = AsyncMock(return_value=(200, upstream))
    with patch("app.admin.gateway_request", new=request):
        response = await list_models(
            jwt_token="gateway-jwt",
            page=2,
            page_size=12,
            search="gpt",
            provider_id="provider-123",
        )

    request.assert_awaited_once_with(
        method="GET",
        path="/admin/models",
        jwt_token="gateway-jwt",
        params={
            "page": 2,
            "page_size": 12,
            "search": "gpt",
            "provider_id": "provider-123",
        },
    )
    assert response == upstream


@pytest.mark.anyio
async def test_usage_and_billing_use_real_gateway_contracts() -> None:
    request = AsyncMock(side_effect=[(200, {"items": []}), (200, {"items": []})])
    with patch("app.admin.gateway_request", new=request):
        await list_usage(jwt_token="jwt", page=1, page_size=20)
        await list_monthly_billing(jwt_token="jwt", page=1, page_size=20, year=2026, month=7)
    assert request.await_args_list[0].kwargs["path"] == "/admin/usage"
    assert request.await_args_list[1].kwargs == {
        "method": "GET", "path": "/admin/billing/monthly", "jwt_token": "jwt",
        "params": {"page": 1, "page_size": 20, "year": 2026, "month": 7},
    }
