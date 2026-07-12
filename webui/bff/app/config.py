"""Configuration for the BFF service."""

from typing import Literal

from pydantic import Field, model_validator
from pydantic_settings import BaseSettings, SettingsConfigDict


DEFAULT_SECRET_KEY = "change-this-to-a-secure-random-key-in-production"


class Settings(BaseSettings):
    """Application settings loaded from environment variables."""

    model_config = SettingsConfigDict(
        env_file=".env", env_file_encoding="utf-8", extra="ignore"
    )

    environment: Literal["development", "production"] = "development"

    # Gateway settings
    gateway_base_url: str = "http://localhost:8080"
    gateway_connect_timeout: float = Field(default=5.0, gt=0)
    gateway_read_timeout: float = Field(default=30.0, gt=0)
    gateway_write_timeout: float = Field(default=10.0, gt=0)
    gateway_pool_timeout: float = Field(default=5.0, gt=0)
    gateway_max_connections: int = Field(default=100, gt=0)
    gateway_max_keepalive_connections: int = Field(default=20, ge=0)
    max_request_body_bytes: int = Field(default=1_048_576, gt=0)

    # Security and cookie settings
    secret_key: str = DEFAULT_SECRET_KEY
    cookie_name: str = "admin_token"
    cookie_path: str = "/"
    cookie_domain: str | None = None
    cookie_secure: bool = False
    cookie_samesite: Literal["strict", "lax"] = "strict"
    cookie_max_age: int = Field(default=3600, gt=0)

    # Proxy headers are ignored unless both settings are explicitly enabled at
    # process startup (see the documented uvicorn invocation).
    trust_proxy_headers: bool = False
    trusted_proxy_ips: str = "127.0.0.1"

    # CORS settings
    cors_origins: list[str] = ["http://localhost:5173"]

    @model_validator(mode="after")
    def validate_production_security(self) -> "Settings":
        if self.environment != "production":
            return self

        if (
            self.secret_key == DEFAULT_SECRET_KEY
            or len(self.secret_key) < 32
            or len(set(self.secret_key)) < 8
        ):
            raise ValueError(
                "SECRET_KEY must be a non-default, high-entropy value of at least "
                "32 characters in production"
            )
        if not self.cookie_secure:
            raise ValueError("COOKIE_SECURE must be true in production")
        return self


settings = Settings()
