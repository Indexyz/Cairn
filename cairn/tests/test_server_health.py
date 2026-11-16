from __future__ import annotations

import json

import requests

from cairn.server import health
from cairn.server.models import WorkerModel

from conftest_models import model


class _Resp:
    def __init__(self, status_code: int, text: str = "") -> None:
        self.status_code = status_code
        self.text = text


def _capture_post(monkeypatch, response: _Resp) -> dict:
    captured: dict = {}

    def fake_post(url, headers=None, json=None, timeout=None, proxies=None):
        captured.update(url=url, headers=headers, json=json, timeout=timeout, proxies=proxies)
        return response

    monkeypatch.setattr(requests, "post", fake_post)
    return captured


def test_http_ping_healthy_on_2xx(monkeypatch) -> None:
    _capture_post(monkeypatch, _Resp(200, "ok"))

    result = health.http_ping("http://api/v1/messages", headers={}, json_body={}, timeout=5)

    assert result.ok
    assert result.status == 200


def test_http_ping_unhealthy_on_4xx(monkeypatch) -> None:
    _capture_post(monkeypatch, _Resp(401, "unauthorized token"))

    result = health.http_ping("http://api/v1/messages", headers={}, json_body={}, timeout=5)

    assert not result.ok
    assert result.status == 401
    assert "unauthorized" in result.detail


def test_http_ping_unhealthy_on_connection_error(monkeypatch) -> None:
    def boom(*_args, **_kwargs):
        raise requests.ConnectionError("no route")

    monkeypatch.setattr(requests, "post", boom)

    result = health.http_ping("http://api/v1/messages", headers={}, json_body={}, timeout=5)

    assert not result.ok
    assert result.status is None
    assert "no route" in result.detail


def test_proxies_from_env() -> None:
    assert health.proxies_from_env({"https_proxy": "http://p:7897"}) == {"https": "http://p:7897"}
    assert health.proxies_from_env({"all_proxy": "http://p:1"}) == {"http": "http://p:1", "https": "http://p:1"}
    assert health.proxies_from_env({}) is None


def test_claudecode_check_hits_messages_with_bearer(monkeypatch) -> None:
    captured = _capture_post(monkeypatch, _Resp(200))
    worker = model("claudecode", {"ANTHROPIC_BASE_URL": "http://api", "ANTHROPIC_AUTH_TOKEN": "tok", "ANTHROPIC_MODEL": "m"})

    result = health.check_model(worker, timeout=7)

    assert result.ok
    assert captured["url"] == "http://api/v1/messages"
    assert captured["headers"]["Authorization"] == "Bearer tok"
    assert captured["headers"]["anthropic-version"]
    assert captured["json"]["model"] == "m"
    assert captured["timeout"] == 7


def test_codex_check_hits_responses(monkeypatch) -> None:
    captured = _capture_post(monkeypatch, _Resp(200))
    worker = model("codex", {"CODEX_BASE_URL": "http://api/v1", "OPENAI_API_KEY": "k", "CODEX_MODEL": "m"})

    health.check_model(worker, timeout=5)

    assert captured["url"] == "http://api/v1/responses"
    assert captured["headers"]["Authorization"] == "Bearer k"


def test_pi_check_uses_provider_api_shape(monkeypatch) -> None:
    captured = _capture_post(monkeypatch, _Resp(200))
    completions = model(
        "pi",
        {"PI_BASE_URL": "http://api/v1", "PI_API_KEY": "k", "PI_MODEL": "m", "PI_PROVIDER_API": "openai-completions"},
    )
    health.check_model(completions, timeout=5)
    assert captured["url"] == "http://api/v1/chat/completions"

    anthropic = model(
        "pi",
        {"PI_BASE_URL": "http://api", "PI_API_KEY": "k", "PI_MODEL": "m", "PI_PROVIDER_API": "anthropic-messages"},
    )
    health.check_model(anthropic, timeout=5)
    assert captured["url"] == "http://api/v1/messages"
    assert captured["headers"]["anthropic-version"]


def test_check_uses_worker_proxy(monkeypatch) -> None:
    captured = _capture_post(monkeypatch, _Resp(200))
    worker = model(
        "claudecode",
        {"ANTHROPIC_BASE_URL": "http://api", "ANTHROPIC_AUTH_TOKEN": "t", "ANTHROPIC_MODEL": "m", "https_proxy": "http://127.0.0.1:7897"},
    )

    health.check_model(worker, timeout=5)

    assert captured["proxies"] == {"https": "http://127.0.0.1:7897"}


def test_mock_check_reflects_configured_outcome() -> None:
    ok = model("mock", {"MOCK_HEALTHCHECK": json.dumps({"delay": [0, 0], "outcomes": {"ok": 1.0, "fail": 0.0}})})
    fail = model("mock", {"MOCK_HEALTHCHECK": json.dumps({"delay": [0, 0], "outcomes": {"ok": 0.0, "fail": 1.0}})})

    assert health.check_model(ok, timeout=1).ok
    result = health.check_model(fail, timeout=1)
    assert not result.ok
    assert result.status == 503
    assert result.detail == "mock healthcheck fail"


def test_missing_env_raises_key_error() -> None:
    try:
        health.check_model(model("claudecode", {}), timeout=1)
    except KeyError as exc:
        assert "ANTHROPIC_BASE_URL" in str(exc)
    else:
        raise AssertionError("expected KeyError")


def test_describe_model_names_the_probe() -> None:
    worker = model("codex", {"CODEX_BASE_URL": "http://api", "CODEX_MODEL": "m"})

    assert health.describe_model(worker) == "POST http://api/responses (model=m)"
    assert WorkerModel(name="x", type="mock", task_types=["reason"], max_running=1, priority=0).type == "mock"
