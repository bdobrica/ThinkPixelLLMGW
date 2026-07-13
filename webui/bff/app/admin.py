"""Admin routes that proxy to the Go gateway."""
from datetime import datetime
from fastapi import APIRouter, HTTPException, Depends, Query
from pydantic import BaseModel, ConfigDict, Field
from typing import Annotated
from .gateway_client import gateway_request, upstream_error_detail
from .dependencies import get_current_admin_token


router = APIRouter(prefix="/admin", tags=["admin"])


class APIKeyMutation(BaseModel):
    model_config = ConfigDict(extra="forbid")

    name: str | None = Field(default=None, min_length=1, max_length=255)
    allowed_models: list[str] | None = None
    rate_limit_per_minute: int | None = Field(default=None, ge=0)
    monthly_budget_usd: float | None = Field(default=None, ge=0)
    enabled: bool | None = None
    expires_at: datetime | None = None
    tags: dict[str, str] | None = None


class CreateAPIKeyRequest(APIKeyMutation):
    name: str = Field(min_length=1, max_length=255)
    rate_limit_per_minute: int = Field(default=60, ge=0)


class UpdateAPIKeyRequest(APIKeyMutation):
    pass


def payload_dict(payload: BaseModel) -> dict:
    return payload.model_dump(mode="json", exclude_unset=True)


def proxy_error(status_code: int, data: dict | None, fallback: str) -> HTTPException:
    return HTTPException(
        status_code=status_code,
        detail=upstream_error_detail(data, fallback),
    )


@router.get("/api-keys")
async def list_api_keys(
    jwt_token: Annotated[str, Depends(get_current_admin_token)],
    page: int = Query(1, ge=1),
    page_size: int = Query(20, ge=1, le=100),
):
    """List API keys by proxying to the Go gateway."""
    status_code, data = await gateway_request(
        method="GET",
        path="/admin/keys",
        jwt_token=jwt_token,
        params={"page": page, "page_size": page_size}
    )
    
    if status_code != 200:
        raise proxy_error(status_code, data, "Failed to list API keys")
    
    return data


@router.post("/api-keys")
async def create_api_key(
    jwt_token: Annotated[str, Depends(get_current_admin_token)],
    payload: CreateAPIKeyRequest,
):
    """Create API key by proxying to the Go gateway."""
    status_code, data = await gateway_request(
        method="POST",
        path="/admin/keys",
        jwt_token=jwt_token,
        json_data=payload_dict(payload)
    )
    
    if status_code != 201:
        raise proxy_error(status_code, data, "Failed to create API key")
    
    return data


@router.put("/api-keys/{key_id}")
async def update_api_key(
    key_id: str,
    jwt_token: Annotated[str, Depends(get_current_admin_token)],
    payload: UpdateAPIKeyRequest,
):
    """Update API key by proxying to the Go gateway."""
    status_code, data = await gateway_request(
        method="PUT",
        path=f"/admin/keys/{key_id}",
        jwt_token=jwt_token,
        json_data=payload_dict(payload)
    )
    
    if status_code != 200:
        raise proxy_error(status_code, data, "Failed to update API key")
    
    return data


@router.delete("/api-keys/{key_id}")
async def revoke_api_key(
    key_id: str,
    jwt_token: Annotated[str, Depends(get_current_admin_token)],
):
    """Revoke API key by proxying to the Go gateway."""
    status_code, data = await gateway_request(
        method="DELETE",
        path=f"/admin/keys/{key_id}",
        jwt_token=jwt_token,
    )
    
    if status_code != 200:
        raise proxy_error(status_code, data, "Failed to revoke API key")
    
    return data


@router.get("/models")
async def list_models(
    jwt_token: Annotated[str, Depends(get_current_admin_token)],
    page: int = Query(1, ge=1),
    page_size: int = Query(20, ge=1, le=100),
    search: str | None = Query(default=None, max_length=255),
    provider_id: str | None = Query(default=None, max_length=64),
):
    """List models by proxying to the Go gateway."""
    status_code, data = await gateway_request(
        method="GET",
        path="/admin/models",
        jwt_token=jwt_token,
        params={
            "page": page,
            "page_size": page_size,
            **({"search": search} if search else {}),
            **({"provider_id": provider_id} if provider_id else {}),
        }
    )
    
    if status_code != 200:
        raise proxy_error(status_code, data, "Failed to list models")
    
    return data


@router.get("/usage")
async def list_usage(
    jwt_token: Annotated[str, Depends(get_current_admin_token)],
    page: int = Query(1, ge=1), page_size: int = Query(20, ge=1, le=100),
    start: datetime | None = None, end: datetime | None = None,
    api_key_id: str | None = None, model: str | None = Query(None, max_length=255),
    status_code: int | None = Query(None, ge=100, le=599),
):
    params = {"page": page, "page_size": page_size}
    for key, value in {"start": start.isoformat() if start else None, "end": end.isoformat() if end else None,
                       "api_key_id": api_key_id, "model": model, "status_code": status_code}.items():
        if value is not None:
            params[key] = value
    status, data = await gateway_request(method="GET", path="/admin/usage", jwt_token=jwt_token, params=params)
    if status != 200:
        raise proxy_error(status, data, "Failed to list usage")
    return data


@router.get("/billing/monthly")
async def list_monthly_billing(
    jwt_token: Annotated[str, Depends(get_current_admin_token)],
    page: int = Query(1, ge=1), page_size: int = Query(20, ge=1, le=100),
    year: int | None = Query(None, ge=2000, le=9999), month: int | None = Query(None, ge=1, le=12),
    api_key_id: str | None = None,
):
    params = {"page": page, "page_size": page_size}
    for key, value in {"year": year, "month": month, "api_key_id": api_key_id}.items():
        if value is not None:
            params[key] = value
    status, data = await gateway_request(method="GET", path="/admin/billing/monthly", jwt_token=jwt_token, params=params)
    if status != 200:
        raise proxy_error(status, data, "Failed to list monthly billing")
    return data


@router.get("/dashboard")
async def get_dashboard(
    jwt_token: Annotated[str, Depends(get_current_admin_token)],
    hours: int = Query(24, ge=1, le=168),
):
    status, data = await gateway_request(method="GET", path="/admin/dashboard", jwt_token=jwt_token, params={"hours": hours})
    if status != 200:
        raise proxy_error(status, data, "Failed to load dashboard")
    return data
