from __future__ import annotations

import json

from fastapi.testclient import TestClient
import pytest

from cairn.server import db
from cairn.server.app import app


@pytest.fixture
def client(tmp_path, monkeypatch) -> TestClient:
    monkeypatch.setattr(db, "_db_path", None)
    db.configure(tmp_path / "cairn.db")
    with TestClient(app) as test_client:
        yield test_client


def _model(name: str = "claude-1", **overrides) -> dict:
    payload = {
        "name": name,
        "type": "claudecode",
        "task_types": ["bootstrap", "reason", "explore"],
        "max_running": 2,
        "priority": 0,
        "env": {
            "ANTHROPIC_MODEL": "deepseek-v4-pro",
            "ANTHROPIC_BASE_URL": "https://api.example.com/anthropic",
            "ANTHROPIC_AUTH_TOKEN": "token",
        },
    }
    payload.update(overrides)
    return payload


def test_model_crud_roundtrip(client: TestClient) -> None:
    response = client.post("/models", json=_model())
    assert response.status_code == 201
    assert response.json()["name"] == "claude-1"

    duplicate = client.post("/models", json=_model())
    assert duplicate.status_code == 409

    listed = client.get("/models")
    assert listed.status_code == 200
    assert [item["name"] for item in listed.json()] == ["claude-1"]

    renamed = client.put("/models/claude-1", json=_model("claude-2"))
    assert renamed.status_code == 200
    assert renamed.json()["name"] == "claude-2"
    assert [item["name"] for item in client.get("/models").json()] == ["claude-2"]

    conflict = client.post("/models", json=_model("claude-1"))
    assert conflict.status_code == 201
    assert client.put("/models/claude-2", json=_model("claude-1")).status_code == 409

    assert client.delete("/models/claude-2").status_code == 204
    assert client.delete("/models/claude-2").status_code == 404
    assert [item["name"] for item in client.get("/models").json()] == ["claude-1"]


def test_replace_models_is_a_full_snapshot(client: TestClient) -> None:
    client.post("/models", json=_model("keep"))
    client.post("/models", json=_model("drop"))

    payload = {"models": [_model("keep", priority=3), _model("added")]}
    response = client.put("/models", json=payload)
    assert response.status_code == 200
    # ordered by priority then name
    assert [item["name"] for item in response.json()] == ["added", "keep"]
    assert response.json()[1]["priority"] == 3
    assert [item["name"] for item in client.get("/models").json()] == ["added", "keep"]

    cleared = client.put("/models", json={"models": []})
    assert cleared.status_code == 200
    assert cleared.json() == []


def test_replace_models_rejects_duplicate_names(client: TestClient) -> None:
    response = client.put("/models", json={"models": [_model("dup"), _model("dup")]})
    assert response.status_code == 422


def test_model_validation_rejects_bad_payloads(client: TestClient) -> None:
    assert client.post("/models", json=_model(type="nope")).status_code == 422
    assert client.post("/models", json=_model(task_types=[])).status_code == 422
    assert client.post("/models", json=_model(task_types=["reason", "reason"])).status_code == 422
    assert client.post("/models", json=_model(max_running=0)).status_code == 422

    bad_mock = _model(
        name="mock-1",
        type="mock",
        env={"MOCK_REASON": json.dumps({"delay": [0, 1], "outcomes": {"intent": 0.5}})},
    )
    assert client.post("/models", json=bad_mock).status_code == 422

    good_mock = _model(
        name="mock-2",
        type="mock",
        env={"MOCK_REASON": json.dumps({"delay": [0, 1], "outcomes": {"intent": 1.0}})},
    )
    assert client.post("/models", json=good_mock).status_code == 201


def _mock_model(name: str, behavior: str) -> dict:
    return _model(
        name=name,
        type="mock",
        env={"MOCK_HEALTHCHECK": behavior},
    )


def test_model_test_reports_healthy(client: TestClient) -> None:
    client.post("/models", json=_mock_model("ok", json.dumps({"delay": [0, 0], "outcomes": {"ok": 1.0, "fail": 0.0}})))

    response = client.post("/models/ok/test")

    assert response.status_code == 200
    payload = response.json()
    assert payload["ok"] is True
    assert payload["status"] == 200
    assert payload["endpoint"] == "mock in-process healthcheck"
    assert payload["duration_ms"] >= 0


def test_model_test_reports_unhealthy(client: TestClient) -> None:
    client.post("/models", json=_mock_model("bad", json.dumps({"delay": [0, 0], "outcomes": {"ok": 0.0, "fail": 1.0}})))

    response = client.post("/models/bad/test")

    assert response.status_code == 200
    assert response.json()["ok"] is False
    assert response.json()["status"] == 503
    assert response.json()["detail"] == "mock healthcheck fail"


def test_model_test_returns_404_for_unknown_model(client: TestClient) -> None:
    assert client.post("/models/nope/test").status_code == 404


def test_model_test_reports_missing_env(client: TestClient) -> None:
    client.post("/models", json=_model("empty-claude", env={}))

    response = client.post("/models/empty-claude/test")

    assert response.status_code == 422
    assert "ANTHROPIC_BASE_URL" in response.json()["detail"]


def test_usage_report_is_reflected_in_the_list(client: TestClient) -> None:
    client.post("/models", json=_model("busy"))
    client.post("/models", json=_model("idle"))

    response = client.put("/models/usage", json={"usage": [{"name": "busy", "used": 2}]})

    assert response.status_code == 200
    usage = {item["name"]: item["used"] for item in response.json()}
    assert usage == {"busy": 2, "idle": 0}
    assert {item["name"]: item["used"] for item in client.get("/models").json()} == usage


def test_usage_expires_and_is_dropped_with_the_model(client: TestClient, monkeypatch) -> None:
    from cairn.server.routers import models as models_router

    client.post("/models", json=_model("gone"))
    client.put("/models/usage", json={"usage": [{"name": "gone", "used": 5}]})
    assert models_router._usage_for("gone") == 5

    models_router._USAGE["gone"] = (5, 0.0)
    assert models_router._usage_for("gone") == 0

    client.delete("/models/gone")
    assert "gone" not in models_router._USAGE


def test_usage_report_accepts_an_empty_payload(client: TestClient) -> None:
    client.post("/models", json=_model("quiet"))
    client.put("/models/usage", json={"usage": [{"name": "quiet", "used": 3}]})

    response = client.put("/models/usage", json={"usage": []})

    assert response.status_code == 200
    assert [item["used"] for item in response.json()] == [0]


class _TestClientSession:
    """Bridge the dispatcher client onto TestClient so the HTTP contract is exercised."""

    def __init__(self, test_client) -> None:
        self._client = test_client
        self.calls: list[dict] = []

    def get(self, url, timeout=None, **kwargs):
        return self._client.get(url, **kwargs)

    def put(self, url, json=None, timeout=None, **kwargs):
        self.calls.append({"url": url, "json": json})
        return self._client.put(url, json=json, **kwargs)

    def request(self, method, url, json=None, timeout=None, **kwargs):
        return self._client.request(method, url, json=json, **kwargs)

    def close(self) -> None:
        return None


def _dispatcher_client(client: TestClient):
    from cairn.dispatcher.protocol.client import CairnClient

    session = _TestClientSession(client)
    transport = CairnClient("http://testserver")
    transport._session = lambda: session
    return transport, session


def test_dispatcher_client_parses_model_responses_with_read_only_fields(client: TestClient) -> None:
    from pydantic import ValidationError

    from cairn.dispatcher.config import WorkerConfig
    from cairn.dispatcher.protocol.client import WorkerModelView

    client.post("/models", json=_model("parsed"))
    client.put("/models/usage", json={"usage": [{"name": "parsed", "used": 1}]})

    transport, _ = _dispatcher_client(client)

    models = transport.list_models()

    assert [model.name for model in models] == ["parsed"]
    assert isinstance(models[0], WorkerModelView)
    assert models[0].used == 1
    # The strict write model is unchanged for dispatch.yaml and request bodies.
    with pytest.raises(ValidationError):
        WorkerConfig.model_validate({**_model("strict"), "used": 1})


def test_dispatcher_client_replace_models_round_trips_without_leaking_used(client: TestClient) -> None:
    client.post("/models", json=_model("kept"))
    transport, session = _dispatcher_client(client)

    models = transport.list_models()
    replaced = transport.replace_models(models)

    assert [model.name for model in replaced] == ["kept"]
    sent = session.calls[-1]["json"]["models"]
    assert all("used" not in entry for entry in sent)
