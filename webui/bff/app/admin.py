"""Admin routes that proxy to the Go gateway."""
from fastapi import APIRouter, HTTPException, Depends, Query
from typing import Annotated, Any
from .gateway_client import gateway_request
from .dependencies import get_current_admin_token


router = APIRouter(prefix="/admin", tags=["admin"])


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
        raise HTTPException(
            status_code=status_code,
            detail=data.get("detail", "Failed to list API keys") if data else "Failed to list API keys"
        )
    
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
        raise HTTPException(
            status_code=status_code,
            detail=data.get("detail", "Failed to list models") if data else "Failed to list models"
        )
    
    return data


@router.get("/billing")
async def get_billing(
    jwt_token: Annotated[str, Depends(get_current_admin_token)],
):
    """Get billing information by proxying to the Go gateway."""
    status_code, data = await gateway_request(
        method="GET",
        path="/admin/billing",
        jwt_token=jwt_token,
    )
    
    if status_code != 200:
        raise HTTPException(
            status_code=status_code,
            detail=data.get("detail", "Failed to get billing info") if data else "Failed to get billing info"
        )
    
    return data
