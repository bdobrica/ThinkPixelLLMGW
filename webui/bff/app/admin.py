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
):
    """List models by proxying to the Go gateway."""
    status_code, data = await gateway_request(
        method="GET",
        path="/admin/models",
        jwt_token=jwt_token,
        params={"page": page, "page_size": page_size}
    )
    
    if status_code != 200:
        raise proxy_error(status_code, data, "Failed to list models")
    
    return data


# NOTE: /admin/billing endpoint is not implemented in the Go gateway yet
# @router.get("/billing")
# async def get_billing(
#     jwt_token: Annotated[str, Depends(get_current_admin_token)],
# ):
#     """Get billing information by proxying to the Go gateway."""
#     status_code, data = await gateway_request(
#         method="GET",
#         path="/admin/billing",
#         jwt_token=jwt_token,
#     )
#     
#     if status_code != 200:
#         raise HTTPException(
#             status_code=status_code,
#             detail=data.get("detail", "Failed to get billing info") if data else "Failed to get billing info"
#         )
#     
#     return data
