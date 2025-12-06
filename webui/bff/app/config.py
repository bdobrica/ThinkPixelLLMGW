"""Configuration for the BFF service."""
from pydantic_settings import BaseSettings


class Settings(BaseSettings):
    """Application settings loaded from environment variables."""
    
    # Gateway settings
    gateway_base_url: str = "http://localhost:8080"
    
    # Security settings
    secret_key: str = "change-this-to-a-secure-random-key-in-production"
    cookie_name: str = "admin_token"
    cookie_max_age: int = 3600  # 1 hour in seconds
    
    # CORS settings
    cors_origins: list[str] = ["http://localhost:5173"]  # Vite default dev server
    
    class Config:
        env_file = ".env"
        env_file_encoding = "utf-8"


# Global settings instance
settings = Settings()
