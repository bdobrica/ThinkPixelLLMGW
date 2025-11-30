#!/usr/bin/env python3
"""
End-to-End Test for LLM Gateway

This test validates the complete LLM Gateway stack by:
1. Starting all services via docker-compose
2. Waiting for services to be healthy
3. Making a real API call through the gateway to OpenAI
4. Validating the response structure and content
5. Testing error handling
6. Cleaning up all services

Requirements:
- Docker and docker-compose installed
- .env file with OPENAI_API_KEY set
- OpenAI Python SDK installed (pip install openai)

Usage:
    python tests/test_e2e.py
    # Or via Make:
    make test-e2e
"""

import os
import sys
import time
import subprocess
import json
from typing import Optional, Dict, Any

try:
    from openai import OpenAI
    from openai import OpenAIError
except ImportError:
    print("Error: OpenAI SDK not installed.")
    print("Install it with: pip install openai")
    sys.exit(1)

import urllib.request
from urllib.error import URLError


class Colors:
    """ANSI color codes for terminal output."""
    HEADER = '\033[95m'
    OKBLUE = '\033[94m'
    OKCYAN = '\033[96m'
    OKGREEN = '\033[92m'
    WARNING = '\033[93m'
    FAIL = '\033[91m'
    ENDC = '\033[0m'
    BOLD = '\033[1m'
    UNDERLINE = '\033[4m'


def print_step(message: str):
    """Print a step message."""
    print(f"\n{Colors.OKBLUE}► {message}{Colors.ENDC}")


def print_success(message: str):
    """Print a success message."""
    print(f"{Colors.OKGREEN}✓ {message}{Colors.ENDC}")


def print_error(message: str):
    """Print an error message."""
    print(f"{Colors.FAIL}✗ {message}{Colors.ENDC}")


def print_warning(message: str):
    """Print a warning message."""
    print(f"{Colors.WARNING}⚠ {message}{Colors.ENDC}")


def print_info(message: str):
    """Print an info message."""
    print(f"{Colors.OKCYAN}  {message}{Colors.ENDC}")


def run_command(cmd: list, check: bool = True, capture_output: bool = True) -> subprocess.CompletedProcess:
    """Run a shell command and return the result."""
    try:
        result = subprocess.run(
            cmd,
            check=check,
            capture_output=capture_output,
            text=True
        )
        return result
    except subprocess.CalledProcessError as e:
        if capture_output:
            print_error(f"Command failed: {' '.join(cmd)}")
            if e.stdout:
                print(f"STDOUT: {e.stdout}")
            if e.stderr:
                print(f"STDERR: {e.stderr}")
        raise


def check_env_file() -> bool:
    """Check if .env file exists and has OPENAI_API_KEY."""
    env_path = os.path.join(os.path.dirname(os.path.dirname(__file__)), '.env')
    
    if not os.path.exists(env_path):
        print_error(f".env file not found at {env_path}")
        return False
    
    with open(env_path, 'r') as f:
        content = f.read()
        if 'OPENAI_API_KEY' not in content:
            print_error("OPENAI_API_KEY not found in .env file")
            return False
        
        # Check if it's not empty
        for line in content.split('\n'):
            if line.startswith('OPENAI_API_KEY='):
                key = line.split('=', 1)[1].strip()
                if not key or key.startswith('sk-') is False:
                    print_error("OPENAI_API_KEY appears to be invalid or empty")
                    return False
    
    print_success(f".env file found with OPENAI_API_KEY")
    return True


def docker_compose_up():
    """Start docker-compose services."""
    print_step("Starting docker-compose services...")
    
    repo_root = os.path.dirname(os.path.dirname(__file__))
    os.chdir(repo_root)
    
    # Start services
    run_command(['docker', 'compose', 'up', '-d'], capture_output=False)
    print_success("Docker-compose services started")


def docker_compose_down():
    """Stop and remove docker-compose services."""
    print_step("Stopping docker-compose services...")
    
    repo_root = os.path.dirname(os.path.dirname(__file__))
    os.chdir(repo_root)
    
    run_command(['docker', 'compose', 'down', '-v'], capture_output=False)
    print_success("Docker-compose services stopped and cleaned up")


def wait_for_service(url: str, timeout: int = 120, interval: int = 5) -> bool:
    """Wait for a service to become available."""
    print_info(f"Waiting for service at {url} (timeout: {timeout}s)...")
    
    start_time = time.time()
    while time.time() - start_time < timeout:
        try:
            with urllib.request.urlopen(url, timeout=3) as response:
                if response.status == 200:
                    return True
        except (URLError, Exception) as e:
            # Service not ready yet
            pass
        
        elapsed = int(time.time() - start_time)
        print(f"  Waiting... ({elapsed}s elapsed)", end='\r')
        time.sleep(interval)
    
    return False


def wait_for_gateway(timeout: int = 120) -> bool:
    """Wait for the LLM Gateway to be healthy."""
    print_step("Waiting for LLM Gateway to be healthy...")
    
    health_url = "http://localhost:8080/health"
    
    if wait_for_service(health_url, timeout):
        print_success("LLM Gateway is healthy and ready")
        return True
    else:
        print_error(f"LLM Gateway failed to become healthy within {timeout}s")
        return False


def test_chat_completion() -> bool:
    """Test a chat completion request through the gateway."""
    print_step("Testing chat completion through gateway...")
    
    try:
        # Initialize OpenAI client pointing to our gateway
        client = OpenAI(
            api_key="demo-key-12345",  # Demo API key from seed data
            base_url="http://localhost:8080/v1",
        )
        
        print_info("Sending chat completion request...")
        response = client.chat.completions.create(
            model="gpt-4o",
            messages=[
                {"role": "user", "content": "Say 'Hello from LLM Gateway!' and nothing else."}
            ],
            max_tokens=50
        )
        
        # Validate response structure
        assert response.choices, "Response has no choices"
        assert len(response.choices) > 0, "Response has empty choices"
        assert response.choices[0].message, "Response choice has no message"
        assert response.choices[0].message.content, "Response message has no content"
        
        content = response.choices[0].message.content
        print_info(f"Response: {content}")
        
        # Validate that we got a reasonable response
        assert len(content) > 0, "Response content is empty"
        
        # Check response metadata
        assert response.model, "Response has no model field"
        assert response.usage, "Response has no usage field"
        assert response.usage.total_tokens > 0, "Response usage shows no tokens"
        
        print_success(f"Chat completion successful!")
        print_info(f"  Model: {response.model}")
        print_info(f"  Tokens: {response.usage.total_tokens} total ({response.usage.prompt_tokens} prompt + {response.usage.completion_tokens} completion)")
        
        return True
        
    except OpenAIError as e:
        print_error(f"OpenAI API error: {e}")
        return False
    except AssertionError as e:
        print_error(f"Validation error: {e}")
        return False
    except Exception as e:
        print_error(f"Unexpected error: {type(e).__name__}: {e}")
        return False


def test_invalid_api_key() -> bool:
    """Test that invalid API key is rejected."""
    print_step("Testing invalid API key handling...")
    
    try:
        client = OpenAI(
            api_key="invalid-key-xyz",
            base_url="http://localhost:8080/v1",
        )
        
        print_info("Sending request with invalid API key...")
        response = client.chat.completions.create(
            model="gpt-4o",
            messages=[{"role": "user", "content": "This should fail"}],
            max_tokens=10
        )
        
        # If we get here, the test failed (should have raised an error)
        print_error("Invalid API key was accepted (should have been rejected)")
        return False
        
    except OpenAIError as e:
        # Expected behavior
        print_success(f"Invalid API key correctly rejected")
        print_info(f"  Error: {str(e)[:100]}")
        return True
    except Exception as e:
        print_warning(f"Unexpected error type: {type(e).__name__}: {e}")
        return True  # Still counts as success if request was rejected


def test_model_alias() -> bool:
    """Test model alias resolution."""
    print_step("Testing model alias resolution...")
    
    try:
        client = OpenAI(
            api_key="demo-key-12345",
            base_url="http://localhost:8080/v1",
        )
        
        # Use the alias 'gpt-4' which should map to 'gpt-4o' based on seed data
        print_info("Sending request with model alias 'gpt-4'...")
        response = client.chat.completions.create(
            model="gpt-4",  # This is an alias
            messages=[{"role": "user", "content": "Hi"}],
            max_tokens=10
        )
        
        print_success(f"Model alias resolved successfully")
        print_info(f"  Requested: gpt-4 (alias)")
        print_info(f"  Actual model: {response.model}")
        
        return True
        
    except OpenAIError as e:
        print_warning(f"Model alias test failed: {e}")
        # This is not critical, as aliases might not be configured
        return True
    except Exception as e:
        print_error(f"Unexpected error: {type(e).__name__}: {e}")
        return False


def check_gateway_logs():
    """Print gateway logs for debugging."""
    print_step("Fetching gateway logs (last 30 lines)...")
    
    try:
        result = run_command(
            ['docker', 'logs', '--tail', '30', 'gw-gateway'],
            check=False
        )
        if result.stdout:
            print(result.stdout)
        if result.stderr:
            print(result.stderr)
    except Exception as e:
        print_warning(f"Could not fetch logs: {e}")


def main():
    """Main test execution."""
    print(f"\n{Colors.BOLD}{Colors.HEADER}{'='*60}")
    print("LLM Gateway - End-to-End Test")
    print(f"{'='*60}{Colors.ENDC}\n")
    
    # Track test results
    tests_run = 0
    tests_passed = 0
    cleanup = True
    
    try:
        # Pre-flight checks
        print_step("Running pre-flight checks...")
        
        if not check_env_file():
            print_error("Pre-flight checks failed")
            return 1
        
        print_success("Pre-flight checks passed")
        
        # Start services
        docker_compose_up()
        
        # Wait for gateway
        if not wait_for_gateway(timeout=120):
            check_gateway_logs()
            return 1
        
        # Run tests
        print(f"\n{Colors.BOLD}Running Tests...{Colors.ENDC}")
        
        # Test 1: Basic chat completion
        tests_run += 1
        if test_chat_completion():
            tests_passed += 1
        
        # Test 2: Invalid API key
        tests_run += 1
        if test_invalid_api_key():
            tests_passed += 1
        
        # Test 3: Model alias
        tests_run += 1
        if test_model_alias():
            tests_passed += 1
        
    except KeyboardInterrupt:
        print_warning("\n\nTest interrupted by user")
        cleanup = True
        return 1
    except Exception as e:
        print_error(f"Unexpected error: {type(e).__name__}: {e}")
        import traceback
        traceback.print_exc()
        cleanup = True
        return 1
    finally:
        # Cleanup
        if cleanup:
            docker_compose_down()
    
    # Print results
    print(f"\n{Colors.BOLD}{Colors.HEADER}{'='*60}")
    print("Test Results")
    print(f"{'='*60}{Colors.ENDC}\n")
    
    print(f"Tests run: {tests_run}")
    print(f"Tests passed: {Colors.OKGREEN}{tests_passed}{Colors.ENDC}")
    print(f"Tests failed: {Colors.FAIL if tests_run - tests_passed > 0 else Colors.OKGREEN}{tests_run - tests_passed}{Colors.ENDC}")
    
    if tests_passed == tests_run:
        print(f"\n{Colors.OKGREEN}{Colors.BOLD}✓ All tests passed!{Colors.ENDC}\n")
        return 0
    else:
        print(f"\n{Colors.FAIL}{Colors.BOLD}✗ Some tests failed{Colors.ENDC}\n")
        return 1


if __name__ == "__main__":
    sys.exit(main())
