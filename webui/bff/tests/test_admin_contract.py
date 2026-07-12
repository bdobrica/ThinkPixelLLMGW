"""BFF-to-gateway current-administrator contract tests."""

from unittest.mock import AsyncMock, patch

import pytest

from app.auth import CurrentAdminResponse, me


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
