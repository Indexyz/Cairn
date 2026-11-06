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
