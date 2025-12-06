"""HTTP client for calling the Go LLM Gateway."""
import httpx
from typing import Any
from .config import settings


async def gateway_request(
    method: str,
    path: str,
    jwt_token: str | None = None,
    json_data: dict[str, Any] | None = None,
    params: dict[str, Any] | None = None,
) -> tuple[int, dict[str, Any] | None]:
    """Make a request to the Go gateway.
    
    Args:
        method: HTTP method (GET, POST, etc.)
        path: Path on the gateway (e.g. "/admin/api-keys")
        jwt_token: Optional JWT token for Authorization header
        json_data: Optional JSON body
        params: Optional query parameters
        
    Returns:
        Tuple of (status_code, response_json)
    """
    url = f"{settings.gateway_base_url}{path}"
    headers = {}
    
    if jwt_token:
        headers["Authorization"] = f"Bearer {jwt_token}"
    
    async with httpx.AsyncClient() as client:
        response = await client.request(
            method=method,
            url=url,
            headers=headers,
            json=json_data,
            params=params,
            timeout=30.0,
        )
        
        # Try to parse JSON response
        try:
            response_data = response.json()
        except Exception:
            response_data = None
        
        return response.status_code, response_data
