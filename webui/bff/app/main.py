"""Main FastAPI application for the BFF."""

from contextlib import asynccontextmanager

from fastapi import FastAPI, status
from fastapi.responses import JSONResponse
from fastapi.middleware.cors import CORSMiddleware
from .config import settings
from . import auth, admin
from .gateway_client import close_gateway_client, gateway_is_ready, start_gateway_client


class RequestBodyLimitMiddleware:
    """Reject request bodies once the configured hard limit is crossed."""

    def __init__(self, app, max_bytes: int):
        self.app = app
        self.max_bytes = max_bytes

    async def __call__(self, scope, receive, send):
        if scope["type"] != "http":
            await self.app(scope, receive, send)
            return

        headers = dict(scope.get("headers", []))
        content_length = headers.get(b"content-length")
        if content_length:
            try:
                declared_size = int(content_length)
            except ValueError:
                declared_size = 0
            if declared_size > self.max_bytes:
                response = JSONResponse(
                    status_code=status.HTTP_413_CONTENT_TOO_LARGE,
                    content={"detail": "Request body too large"},
                )
                await response(scope, receive, send)
                return

        total = 0

        async def limited_receive():
            nonlocal total
            message = await receive()
            if message["type"] == "http.request":
                total += len(message.get("body", b""))
                if total > self.max_bytes:
                    raise RequestBodyTooLarge
            return message

        try:
            await self.app(scope, limited_receive, send)
        except RequestBodyTooLarge:
            response = JSONResponse(
                status_code=status.HTTP_413_CONTENT_TOO_LARGE,
                content={"detail": "Request body too large"},
            )
            await response(scope, receive, send)


class RequestBodyTooLarge(Exception):
    pass


@asynccontextmanager
async def lifespan(_: FastAPI):
    await start_gateway_client()
    try:
        yield
    finally:
        await close_gateway_client()


# Create FastAPI app
app = FastAPI(
    title="LLM Gateway BFF",
    description="Backend-for-Frontend service for the LLM Gateway admin UI",
    version="1.0.0",
    lifespan=lifespan,
)
app.add_middleware(RequestBodyLimitMiddleware, max_bytes=settings.max_request_body_bytes)

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
    """BFF process liveness; does not check the gateway."""
    return {"status": "ok"}


@app.get("/ready")
async def ready():
    """BFF readiness, including the downstream gateway."""
    if await gateway_is_ready():
        return {"status": "ready", "gateway": "ready"}
    return JSONResponse(
        status_code=status.HTTP_503_SERVICE_UNAVAILABLE,
        content={"status": "unavailable", "gateway": "unavailable"},
    )
