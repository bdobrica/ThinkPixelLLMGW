"""Production launcher supervision and template rendering tests."""

import os
from pathlib import Path
import signal
import subprocess
import time


WEBUI_DIR = Path(__file__).resolve().parents[2]


def executable(path: Path, content: str) -> None:
    path.write_text(content)
    path.chmod(0o755)


def launcher_environment(tmp_path: Path) -> tuple[dict[str, str], Path, Path]:
    frontend = tmp_path / "dist"
    frontend.mkdir()
    (frontend / "index.html").write_text("<!doctype html><title>test</title>")

    bin_dir = tmp_path / "bin"
    bin_dir.mkdir()
    bff_venv = tmp_path / "venv"
    (bff_venv / "bin").mkdir(parents=True)
    capture = tmp_path / "rendered-nginx.conf"
    bff_pid = tmp_path / "bff.pid"

    executable(bin_dir / "curl", "#!/bin/sh\nexit 0\n")
    executable(
        bff_venv / "bin" / "uvicorn",
        "#!/bin/sh\n"
        'echo $$ >"$BFF_PID_CAPTURE"\n'
        "trap 'exit 0' TERM INT\n"
        "while :; do sleep 0.1; done\n",
    )
    executable(
        bin_dir / "nginx",
        "#!/bin/sh\n"
        "config=\nprevious=\n"
        "for argument in \"$@\"; do\n"
        "  if [ \"$previous\" = -c ]; then config=$argument; fi\n"
        "  previous=$argument\n"
        "done\n"
        "case \" $* \" in\n"
        "  *' -t '*) cp \"$config\" \"$NGINX_CONFIG_CAPTURE\"; exit 0 ;;\n"
        "esac\n"
        "if [ -n \"${NGINX_RUNTIME_EXIT:-}\" ]; then exit \"$NGINX_RUNTIME_EXIT\"; fi\n"
        "trap 'exit 0' TERM INT\n"
        "while :; do sleep 0.1; done\n",
    )

    environment = os.environ.copy()
    environment.update(
        {
            "PATH": f"{bin_dir}:{environment['PATH']}",
            "NGINX_BIN": str(bin_dir / "nginx"),
            "NGINX_MIME_TYPES": str(tmp_path / "mime.types"),
            "NGINX_CONFIG_CAPTURE": str(capture),
            "BFF_PID_CAPTURE": str(bff_pid),
            "FRONTEND_ROOT": str(frontend),
            "BFF_VENV": str(bff_venv),
            "SECRET_KEY": "0123456789abcdef0123456789abcdef",
            "PUBLIC_ORIGIN": "https://admin.example.test",
            "STARTUP_TIMEOUT_SECONDS": "2",
        }
    )
    return environment, capture, bff_pid


def wait_for(path: Path, timeout: float = 5) -> None:
    deadline = time.monotonic() + timeout
    while not path.exists():
        if time.monotonic() >= deadline:
            raise AssertionError(f"timed out waiting for {path}")
        time.sleep(0.05)


def process_exists(pid: int) -> bool:
    try:
        os.kill(pid, 0)
    except ProcessLookupError:
        return False
    return True


def test_launcher_renders_single_template_and_propagates_child_failure(tmp_path) -> None:
    environment, capture, bff_pid_file = launcher_environment(tmp_path)
    environment["NGINX_RUNTIME_EXIT"] = "7"
    result = subprocess.run(
        ["bash", str(WEBUI_DIR / "start-prod.sh")],
        env=environment,
        text=True,
        capture_output=True,
        timeout=10,
        check=False,
    )
    assert result.returncode != 0
    assert "child process exited with status 7" in result.stderr
    rendered = capture.read_text()
    assert "listen 127.0.0.1:8081;" in rendered
    assert "proxy_pass http://127.0.0.1:8000;" in rendered
    assert f'root "{tmp_path / "dist"}";' in rendered
    assert "{{" not in rendered
    wait_for(bff_pid_file)
    assert not process_exists(int(bff_pid_file.read_text()))


def test_launcher_terminates_children_on_signal(tmp_path) -> None:
    environment, capture, bff_pid_file = launcher_environment(tmp_path)
    process = subprocess.Popen(
        ["bash", str(WEBUI_DIR / "start-prod.sh")],
        env=environment,
        text=True,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
    )
    try:
        wait_for(capture)
        wait_for(bff_pid_file)
        process.send_signal(signal.SIGTERM)
        process.communicate(timeout=10)
        assert not process_exists(int(bff_pid_file.read_text()))
    finally:
        if process.poll() is None:
            process.kill()
            process.wait(timeout=5)
