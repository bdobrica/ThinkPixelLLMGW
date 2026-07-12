"""BFF test configuration."""

import os

import pytest

# Ensure imports never inherit a production environment from the test runner.
os.environ["ENVIRONMENT"] = "development"
os.environ["COOKIE_NAME"] = "custom_session"


@pytest.fixture
def anyio_backend() -> str:
    return "asyncio"
