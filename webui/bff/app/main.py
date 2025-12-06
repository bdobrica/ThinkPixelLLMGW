"""Main FastAPI application for the BFF."""
from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware
from .config import settings
from . import auth, admin


# Create FastAPI app
app = FastAPI(
    title="LLM Gateway BFF",
    description="Backend-for-Frontend service for the LLM Gateway admin UI",
    version="1.0.0",
)

# Add CORS middleware for local development
app.add_middleware(
    CORSMiddleware,
    allow_origins=settings.cors_origins,
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

# Mount routers
app.include_router(auth.router)
app.include_router(admin.router)


@app.get("/")
async def root():
    """Health check endpoint."""
    return {
        "service": "LLM Gateway BFF",
        "status": "ok",
        "gateway": settings.gateway_base_url
    }


@app.get("/health")
async def health():
    """Health check endpoint."""
    return {"status": "ok"}
