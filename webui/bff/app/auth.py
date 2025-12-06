"""Authentication routes for the BFF."""
from fastapi import APIRouter, HTTPException, Response, status, Depends
from pydantic import BaseModel
from typing import Annotated
from .config import settings
from .security import sign_token
from .gateway_client import gateway_request
from .dependencies import get_current_admin_token


router = APIRouter(prefix="/auth", tags=["auth"])


class LoginRequest(BaseModel):
    email: str
    password: str


class LoginResponse(BaseModel):
    success: bool


class LogoutResponse(BaseModel):
    success: bool


@router.post("/login", response_model=LoginResponse)
async def login(request: LoginRequest, response: Response):
    """Authenticate with email/password and set signed cookie with JWT.
    
    Calls the Go gateway's /admin/login endpoint, extracts the JWT,
    signs it, and stores it in an HttpOnly cookie.
    """
    # Call Go gateway login endpoint
    status_code, data = await gateway_request(
        method="POST",
        path="/admin/login",
        json_data={"email": request.email, "password": request.password}
    )
    
    if status_code != 200 or not data or "token" not in data:
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="Invalid credentials"
        )
    
    # Extract JWT from gateway response
    jwt_token = data["token"]
    
    # Sign the token for cookie storage
    signed_token = sign_token(jwt_token)
    
    # Set HttpOnly, Secure, SameSite cookie
    response.set_cookie(
        key=settings.cookie_name,
        value=signed_token,
        max_age=settings.cookie_max_age,
        httponly=True,
        secure=False,  # Set to True in production with HTTPS
        samesite="strict",
    )
    
    return LoginResponse(success=True)


@router.post("/logout", response_model=LogoutResponse)
async def logout(response: Response):
    """Clear the authentication cookie."""
    response.delete_cookie(
        key=settings.cookie_name,
        httponly=True,
        secure=False,  # Set to True in production with HTTPS
        samesite="strict",
    )
    return LogoutResponse(success=True)


@router.get("/me")
async def me(jwt_token: Annotated[str, Depends(get_current_admin_token)]):
    """Get current admin user info by proxying to gateway /admin/me.
    
    This endpoint verifies the cookie and calls the gateway to get user details.
    """
    status_code, data = await gateway_request(
        method="GET",
        path="/admin/me",
        jwt_token=jwt_token
    )
    
    if status_code != 200:
        raise HTTPException(
            status_code=status_code,
            detail=data.get("detail", "Failed to get user info") if data else "Failed to get user info"
        )
    
    return data
