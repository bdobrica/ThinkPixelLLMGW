"""Authentication cookie and signing configuration tests."""

from unittest.mock import AsyncMock, patch

import pytest
from httpx import ASGITransport, AsyncClient
from itsdangerous import URLSafeTimedSerializer
from pydantic import ValidationError

from app.config import DEFAULT_SECRET_KEY, Settings, settings
from app.main import app
from app.security import sign_token


def test_production_rejects_default_and_weak_secrets() -> None:
    with pytest.raises(ValidationError):
        Settings(environment="production", cookie_secure=True, _env_file=None)
    with pytest.raises(ValidationError):
        Settings(
            environment="production",
            cookie_secure=True,
            secret_key="too-short",
            _env_file=None,
        )
    with pytest.raises(ValidationError):
        Settings(
            environment="production",
            cookie_secure=True,
            secret_key="a" * 32,
            _env_file=None,
        )


def test_production_requires_secure_cookie() -> None:
    with pytest.raises(ValidationError):
        Settings(
            environment="production",
            cookie_secure=False,
            secret_key="0123456789abcdef" * 2,
            _env_file=None,
        )


def test_development_defaults_support_http() -> None:
    config = Settings(_env_file=None)
    assert config.environment == "development"
    assert config.secret_key == DEFAULT_SECRET_KEY
    assert config.cookie_secure is False


@pytest.mark.anyio
async def test_custom_cookie_name_round_trip_and_logout_attributes(monkeypatch) -> None:
    monkeypatch.setattr(settings, "cookie_path", "/console")
    monkeypatch.setattr(settings, "cookie_secure", True)

    with patch(
        "app.auth.gateway_request",
        new=AsyncMock(return_value=(200, {"token": "gateway-jwt"})),
    ):
        async with AsyncClient(
            transport=ASGITransport(app=app), base_url="https://example.test"
        ) as client:
            login = await client.post(
                "/auth/login",
                json={"email": "admin@example.test", "password": "secret"},
            )
            assert login.status_code == 200
            cookie = login.headers["set-cookie"]
            assert "custom_session=" in cookie
            assert "Path=/console" in cookie
            assert "HttpOnly" in cookie
            assert "Secure" in cookie
            assert "SameSite=strict" in cookie

            # The configured path controls when the browser sends the cookie.
            client.cookies.set(
                "custom_session",
                sign_token("gateway-jwt"),
                path="/",
            )
            with patch(
                "app.auth.gateway_request",
                new=AsyncMock(
                    return_value=(
                        200,
                        {
                            "admin_id": "admin-123",
                            "auth_type": "user",
                            "roles": ["admin"],
                            "email": "admin@example.test",
                        },
                    )
                ),
            ):
                current = await client.get("/auth/me")
                assert current.status_code == 200
                assert current.json()["admin_id"] == "admin-123"

            logout = await client.post("/auth/logout")
            deleted = logout.headers["set-cookie"]
            assert "custom_session=" in deleted
            assert "Max-Age=0" in deleted
            assert "Path=/console" in deleted
            assert "HttpOnly" in deleted
            assert "Secure" in deleted
            assert "SameSite=strict" in deleted


@pytest.mark.anyio
async def test_tampered_cookie_is_rejected() -> None:
    async with AsyncClient(
        transport=ASGITransport(app=app), base_url="http://test"
    ) as client:
        client.cookies.set(settings.cookie_name, sign_token("jwt") + "tampered")
        assert (await client.get("/auth/me")).status_code == 401


@pytest.mark.anyio
async def test_expired_cookie_is_rejected(monkeypatch) -> None:
    monkeypatch.setattr(settings, "cookie_max_age", -1)
    serializer = URLSafeTimedSerializer(settings.secret_key)
    signed = serializer.dumps("jwt", salt="admin-cookie")
    async with AsyncClient(
        transport=ASGITransport(app=app), base_url="http://test"
    ) as client:
        client.cookies.set(settings.cookie_name, signed)
        assert (await client.get("/auth/me")).status_code == 401
