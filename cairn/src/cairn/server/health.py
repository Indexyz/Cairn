from __future__ import annotations

import random
from dataclasses import dataclass

import requests

from cairn.server.mock_behavior import resolve_mock_behavior

DETAIL_LIMIT = 200
ANTHROPIC_VERSION = "2023-06-01"


@dataclass(slots=True)
class HealthResult:
    ok: bool
    status: int | None
    detail: str


def http_ping(
    url: str,
    *,
    headers: dict[str, str],
    json_body: dict,
    timeout: float,
    proxies: dict[str, str] | None = None,
) -> HealthResult:
    """POST a tiny request to an LLM endpoint and judge health by the HTTP status.

    Healthy = a 2xx response (endpoint reachable, key accepted, model callable). Any other
    status, or a connection/timeout error, is unhealthy.
    """
    try:
        response = requests.post(url, headers=headers, json=json_body, timeout=timeout, proxies=proxies)
    except requests.RequestException as exc:
        return HealthResult(ok=False, status=None, detail=_clip(str(exc)))
    ok = 200 <= response.status_code < 300
    return HealthResult(ok=ok, status=response.status_code, detail="" if ok else _clip(response.text))


def proxies_from_env(env: dict[str, str]) -> dict[str, str] | None:
    """Follow the same outbound proxy the worker would use (e.g. a common_env http(s)_proxy)."""
    all_proxy = env.get("all_proxy") or env.get("ALL_PROXY")
    http = env.get("http_proxy") or env.get("HTTP_PROXY") or all_proxy
    https = env.get("https_proxy") or env.get("HTTPS_PROXY") or all_proxy
    proxies: dict[str, str] = {}
    if http:
        proxies["http"] = http
    if https:
        proxies["https"] = https
    return proxies or None


def check_model(model, timeout: float) -> HealthResult:
    """Probe one saved model from the server host."""
    env = model.env
    if model.type == "claudecode":
        return http_ping(
            f"{_required(env, 'ANTHROPIC_BASE_URL')}/v1/messages",
            headers={
                "Authorization": f"Bearer {_required(env, 'ANTHROPIC_AUTH_TOKEN')}",
                "anthropic-version": ANTHROPIC_VERSION,
                "content-type": "application/json",
            },
            json_body={
                "model": _required(env, "ANTHROPIC_MODEL"),
                "max_tokens": 10,
                "messages": [{"role": "user", "content": "ping"}],
            },
            timeout=timeout,
            proxies=proxies_from_env(env),
        )
    if model.type == "codex":
        return http_ping(
            f"{_required(env, 'CODEX_BASE_URL')}/responses",
            headers={
                "Authorization": f"Bearer {_required(env, 'OPENAI_API_KEY')}",
                "content-type": "application/json",
            },
            json_body={
                "model": _required(env, "CODEX_MODEL"),
                "input": [{"role": "user", "content": "ping"}],
                "stream": False,
            },
            timeout=timeout,
            proxies=proxies_from_env(env),
        )
    if model.type == "pi":
        return _check_pi(env, timeout)
    if model.type == "mock":
        return _check_mock(model.name, env)
    raise ValueError(f"unsupported worker type: {model.type}")


def describe_model(model) -> str:
    env = model.env
    if model.type == "claudecode":
        return f"POST {env.get('ANTHROPIC_BASE_URL', '')}/v1/messages (model={env.get('ANTHROPIC_MODEL', '')})"
    if model.type == "codex":
        return f"POST {env.get('CODEX_BASE_URL', '')}/responses (model={env.get('CODEX_MODEL', '')})"
    if model.type == "pi":
        return f"POST {env.get('PI_BASE_URL', '')} (api={env.get('PI_PROVIDER_API', '')}, model={env.get('PI_MODEL', '')})"
    if model.type == "mock":
        return "mock in-process healthcheck"
    return "in-process API ping"


def _check_pi(env: dict[str, str], timeout: float) -> HealthResult:
    base = _required(env, "PI_BASE_URL").rstrip("/")
    api = _required(env, "PI_PROVIDER_API")
    headers = {
        "Authorization": f"Bearer {_required(env, 'PI_API_KEY')}",
        "content-type": "application/json",
    }
    proxies = proxies_from_env(env)
    body = {
        "model": _required(env, "PI_MODEL"),
        "max_tokens": 10,
        "messages": [{"role": "user", "content": "ping"}],
    }
    if "anthropic" in api:
        return http_ping(
            f"{base}/v1/messages",
            headers={**headers, "anthropic-version": ANTHROPIC_VERSION},
            json_body=body,
            timeout=timeout,
            proxies=proxies,
        )
    if "responses" in api:
        return http_ping(
            f"{base}/responses",
            headers=headers,
            json_body={"model": body["model"], "input": [{"role": "user", "content": "ping"}], "stream": False},
            timeout=timeout,
            proxies=proxies,
        )
    # openai-completions and anything else: OpenAI-compatible chat/completions
    return http_ping(f"{base}/chat/completions", headers=headers, json_body=body, timeout=timeout, proxies=proxies)


def _check_mock(worker_name: str, env: dict[str, str]) -> HealthResult:
    outcomes = resolve_mock_behavior(worker_name, env)["healthcheck"]["outcomes"]
    ok = random.random() < outcomes.get("ok", 0.0)
    return HealthResult(ok=ok, status=200 if ok else 503, detail="" if ok else "mock healthcheck fail")


def _required(env: dict[str, str], key: str) -> str:
    value = env.get(key)
    if not value:
        raise KeyError(key)
    return value


def _clip(text: str) -> str:
    compact = " ".join(text.split())
    return compact if len(compact) <= DETAIL_LIMIT else compact[:DETAIL_LIMIT] + "..."
