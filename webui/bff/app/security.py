"""Cookie signing and verification using itsdangerous."""
from itsdangerous import URLSafeTimedSerializer, BadSignature, SignatureExpired
from .config import settings


def sign_token(token: str) -> str:
    """Sign a JWT token for storage in a cookie."""
    serializer = URLSafeTimedSerializer(settings.secret_key)
    return serializer.dumps(token, salt="admin-cookie")


def verify_token(signed_token: str, max_age: int = None) -> str | None:
    """Verify and extract a JWT token from a signed cookie value.
    
    Args:
        signed_token: The signed token from the cookie
        max_age: Maximum age in seconds (None = no expiration check)
        
    Returns:
        The original JWT token, or None if verification fails
    """
    serializer = URLSafeTimedSerializer(settings.secret_key)
    try:
        token = serializer.loads(
            signed_token,
            salt="admin-cookie",
            max_age=max_age
        )
        return token
    except (BadSignature, SignatureExpired):
        return None
