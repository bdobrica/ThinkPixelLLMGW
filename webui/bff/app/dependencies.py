"""FastAPI dependencies for auth and request handling."""
from fastapi import Cookie, HTTPException, status
from typing import Annotated
from .config import settings
from .security import verify_token


async def get_current_admin_token(
    admin_token: Annotated[str | None, Cookie()] = None
) -> str:
    """Extract and verify the admin JWT from the signed cookie.
    
    Raises:
        HTTPException: 401 if cookie is missing or invalid
        
    Returns:
        The verified JWT token
    """
    if not admin_token:
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="Not authenticated"
        )
    
    # Verify the signed cookie and extract the JWT
    jwt_token = verify_token(admin_token, max_age=settings.cookie_max_age)
    
    if not jwt_token:
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="Invalid or expired token"
        )
    
    return jwt_token
