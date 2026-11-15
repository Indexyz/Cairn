from __future__ import annotations

from fastapi.testclient import TestClient
import pytest

from cairn.server import db, notifications
from cairn.server.app import app


@pytest.fixture
def client(tmp_path, monkeypatch) -> TestClient:
    monkeypatch.setattr(db, "_db_path", None)
    db.configure(tmp_path / "cairn.db")
    with TestClient(app) as test_client:
        yield test_client


def _project(client: TestClient, max_steps: int | None = 2, title: str = "budgeted") -> str:
    body: dict = {"title": title, "origin": "start", "goal": "finish", "bootstrap_enabled": False}
    if max_steps is not None:
        body["max_steps"] = max_steps
    response = client.post("/projects", json=body)
    assert response.status_code == 201
    return response.json()["project"]["id"]


def _intent(client: TestClient, project_id: str, description: str = "step"):
    return client.post(
        f"/projects/{project_id}/intents",
        json={"from": ["origin"], "description": description, "creator": "reasoner", "worker": None},
    )


def _conclude(client: TestClient, project_id: str, intent_id: str) -> None:
    response = client.post(
        f"/projects/{project_id}/intents/{intent_id}/conclude",
        json={"worker": "explorer", "description": f"fact {intent_id}"},
    )
    assert response.status_code == 200


def _status(client: TestClient, project_id: str) -> str:
    return client.get(f"/projects/{project_id}").json()["project"]["status"]


def test_max_steps_is_optional_and_round_trips(client: TestClient) -> None:
    unlimited = client.post(
        "/projects", json={"title": "free", "origin": "start", "goal": "finish"}
    ).json()["project"]
    assert unlimited["max_steps"] is None

    project_id = _project(client, max_steps=3)
    detail = client.get(f"/projects/{project_id}").json()["project"]
    assert detail["max_steps"] == 3
    listed = {project["id"]: project for project in client.get("/projects").json()}
    assert listed[project_id]["max_steps"] == 3
    assert "max_steps: 3" in client.get(f"/projects/{project_id}/export?format=yaml").text


@pytest.mark.parametrize("value", [0, -2])
def test_max_steps_rejects_non_positive_values(client: TestClient, value: int) -> None:
    response = client.post(
        "/projects",
        json={"title": "bad", "origin": "start", "goal": "finish", "max_steps": value},
    )
    assert response.status_code == 422


def test_cap_refuses_new_intents_and_blocks_once_the_queue_drains(client: TestClient) -> None:
    project_id = _project(client, max_steps=2)

    assert _intent(client, project_id, "first").status_code == 201
    assert _intent(client, project_id, "second").status_code == 201
    blocked = _intent(client, project_id, "third")
    assert blocked.status_code == 403
    assert "step limit" in blocked.json()["detail"]
    # An open intent is never cut off, so the project keeps running for now.
    assert _status(client, project_id) == "active"

    _conclude(client, project_id, "i001")
    assert _status(client, project_id) == "active"

    _conclude(client, project_id, "i002")
    assert _status(client, project_id) == "blocked"

    assert _intent(client, project_id, "fourth").status_code == 403
    assert client.post(
        f"/projects/{project_id}/hints", json={"content": "note", "creator": "human"}
    ).status_code == 201


def test_blocked_project_still_completes_and_notifies(client: TestClient, monkeypatch) -> None:
    calls: list[tuple[str, str]] = []

    def fake_send(channel, event, project_id, title, text):
        calls.append((event, project_id))
        return {"ok": True, "status": 204, "detail": "", "duration_ms": 1}

    monkeypatch.setattr(notifications, "send", fake_send)
    channel = client.post(
        "/notifications",
        json={
            "name": "blocked",
            "kind": "webhook",
            "enabled": True,
            "events": ["project_blocked"],
            "config": {"url": "https://x.test/hook"},
        },
    )

    assert channel.status_code == 201

    project_id = _project(client, max_steps=1)
    assert _intent(client, project_id, "only").status_code == 201
    _conclude(client, project_id, "i001")
    assert _status(client, project_id) == "blocked"
    assert calls == [("project_blocked", project_id)]

    response = client.post(
        f"/projects/{project_id}/complete",
        json={"from": ["f001"], "description": "solved", "worker": "reasoner"},
    )
    assert response.status_code == 200
    assert _status(client, project_id) == "completed"


def test_resuming_a_spent_project_blocks_again_on_the_next_intent(
    client: TestClient, monkeypatch
) -> None:
    calls: list[str] = []
    monkeypatch.setattr(
        notifications,
        "send",
        lambda channel, event, *args: calls.append(event)
        or {"ok": True, "status": 204, "detail": "", "duration_ms": 1},
    )
    client.post(
        "/notifications",
        json={
            "name": "blocked",
            "kind": "webhook",
            "enabled": True,
            "events": ["project_blocked"],
            "config": {"url": "https://x.test/hook"},
        },
    )

    project_id = _project(client, max_steps=1)
    _intent(client, project_id, "only")
    _conclude(client, project_id, "i001")

    assert client.put(f"/projects/{project_id}/status", json={"status": "active"}).json()[
        "status"
    ] == "active"
    assert _intent(client, project_id, "over budget").status_code == 403
    assert _status(client, project_id) == "blocked"
    # One event for the first block, one for the re-block after the resume.
    assert calls == ["project_blocked", "project_blocked"]

    assert client.put(f"/projects/{project_id}/status", json={"status": "stopped"}).status_code == 200


def test_raising_or_clearing_max_steps_unblocks(client: TestClient) -> None:
    project_id = _project(client, max_steps=1)
    _intent(client, project_id, "only")
    _conclude(client, project_id, "i001")
    assert _status(client, project_id) == "blocked"

    raised = client.put(f"/projects/{project_id}/max_steps", json={"max_steps": 5})
    assert raised.status_code == 200
    assert raised.json()["status"] == "active"
    assert _intent(client, project_id, "again").status_code == 201

    # Lowering the cap back to the declared count blocks the drained project again.
    lowered = client.put(f"/projects/{project_id}/max_steps", json={"max_steps": 2})
    assert lowered.json()["status"] == "active"
    _conclude(client, project_id, "i002")
    assert _status(client, project_id) == "blocked"

    cleared = client.put(f"/projects/{project_id}/max_steps", json={"max_steps": None})
    assert cleared.json()["status"] == "active"
    assert cleared.json()["max_steps"] is None
    assert _intent(client, project_id, "unbounded").status_code == 201


def test_lowering_the_cap_does_not_block_while_intents_are_open(client: TestClient) -> None:
    project_id = _project(client, max_steps=None)
    assert _intent(client, project_id, "first").status_code == 201
    assert _intent(client, project_id, "second").status_code == 201

    response = client.put(f"/projects/{project_id}/max_steps", json={"max_steps": 1})

    assert response.json()["status"] == "active"
    _conclude(client, project_id, "i001")
    _conclude(client, project_id, "i002")
    assert _status(client, project_id) == "blocked"


def test_max_steps_edits_emit_block_and_resume_events(client: TestClient, monkeypatch) -> None:
    calls: list[str] = []
    monkeypatch.setattr(
        notifications,
        "send",
        lambda channel, event, *args: calls.append(event)
        or {"ok": True, "status": 204, "detail": "", "duration_ms": 1},
    )
    channel = client.post(
        "/notifications",
        json={
            "name": "changes",
            "kind": "webhook",
            "enabled": True,
            "events": ["project_blocked", "project_resumed"],
            "config": {"url": "https://x.test/hook"},
        },
    )
    assert channel.status_code == 201

    project_id = _project(client, max_steps=None)
    assert _intent(client, project_id, "first").status_code == 201
    assert _intent(client, project_id, "second").status_code == 201

    client.put(f"/projects/{project_id}/max_steps", json={"max_steps": 1})
    _conclude(client, project_id, "i001")
    _conclude(client, project_id, "i002")
    client.put(f"/projects/{project_id}/max_steps", json={"max_steps": 9})

    assert calls == ["project_blocked", "project_resumed"]


def test_completed_project_rejects_max_steps_changes(client: TestClient) -> None:
    project_id = _project(client, max_steps=None)
    client.post(
        f"/projects/{project_id}/complete",
        json={"from": ["origin"], "description": "solved", "worker": "reasoner"},
    )

    response = client.put(f"/projects/{project_id}/max_steps", json={"max_steps": 4})

    assert response.status_code == 409


def test_fork_copies_max_steps(client: TestClient) -> None:
    project_id = _project(client, max_steps=7)

    forked = client.post(f"/projects/{project_id}/fork", json={})

    assert forked.status_code == 201
    assert forked.json()["project"]["max_steps"] == 7
