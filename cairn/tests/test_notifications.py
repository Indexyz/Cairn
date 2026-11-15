from __future__ import annotations

from fastapi.testclient import TestClient
import pytest

from cairn.server import db, notifications
from cairn.server.app import app

DISCORD = {
    "name": "team discord",
    "kind": "discord",
    "enabled": True,
    "events": ["project_completed"],
    "config": {"webhook_url": "https://discord.com/api/webhooks/1/abc"},
}


@pytest.fixture
def client(tmp_path, monkeypatch) -> TestClient:
    monkeypatch.setattr(db, "_db_path", None)
    db.configure(tmp_path / "cairn.db")
    with TestClient(app) as test_client:
        yield test_client


@pytest.fixture
def sent(monkeypatch) -> list[tuple[str, str, str, str]]:
    calls: list[tuple[str, str, str, str]] = []

    def fake_send(channel, event, project_id, title, text):
        calls.append((channel.id, event, project_id, title))
        return {"ok": True, "status": 204, "detail": "", "duration_ms": 1}

    monkeypatch.setattr(notifications, "send", fake_send)
    return calls


def _project(client: TestClient) -> str:
    response = client.post(
        "/projects", json={"title": "solved thing", "origin": "start", "goal": "finish"}
    )
    assert response.status_code == 201
    return response.json()["project"]["id"]


def _complete(client: TestClient, project_id: str) -> None:
    response = client.post(
        f"/projects/{project_id}/complete",
        json={"from": ["origin"], "description": "goal reached", "worker": "reasoner"},
    )
    assert response.status_code == 200


def test_channel_crud_round_trip(client: TestClient) -> None:
    created = client.post("/notifications", json=DISCORD)
    assert created.status_code == 201
    channel = created.json()
    assert channel["id"] == "chan_001"
    assert channel["enabled"] is True
    assert client.get("/notifications").json() == [channel]

    updated = client.put(
        f"/notifications/{channel['id']}",
        json={**DISCORD, "name": "renamed", "enabled": False},
    )
    assert updated.status_code == 200
    assert updated.json()["name"] == "renamed"
    assert updated.json()["enabled"] is False

    assert client.delete(f"/notifications/{channel['id']}").status_code == 204
    assert client.get("/notifications").json() == []
    assert client.put("/notifications/chan_999", json=DISCORD).status_code == 404
    assert client.delete("/notifications/chan_999").status_code == 404


@pytest.mark.parametrize(
    ("patch", "field"),
    [
        ({"config": {}}, "webhook_url"),
        ({"config": {"webhook_url": "ftp://example.com"}}, "webhook_url"),
        ({"events": []}, "events"),
        ({"name": "  "}, "name"),
    ],
)
def test_channel_validation(client: TestClient, patch: dict, field: str) -> None:
    response = client.post("/notifications", json={**DISCORD, **patch})
    assert response.status_code == 422
    assert field in response.text


def test_channel_rejects_unknown_kind_and_drops_stray_config(client: TestClient) -> None:
    assert client.post("/notifications", json={**DISCORD, "kind": "irc"}).status_code == 422

    response = client.post(
        "/notifications",
        json={**DISCORD, "config": {"webhook_url": "https://x.test/hook", "extra": "ignored"}},
    )
    assert response.status_code == 201
    assert response.json()["config"] == {"webhook_url": "https://x.test/hook"}


def test_events_reach_subscribed_enabled_channels_only(
    client: TestClient, sent: list[tuple[str, str, str, str]]
) -> None:
    client.post("/notifications", json=DISCORD)
    client.post(
        "/notifications",
        json={
            "name": "disabled",
            "kind": "telegram",
            "enabled": False,
            "events": ["project_completed"],
            "config": {"bot_token": "123:abc", "chat_id": "-1"},
        },
    )
    client.post(
        "/notifications",
        json={
            "name": "created only",
            "kind": "webhook",
            "enabled": True,
            "events": ["project_created"],
            "config": {"url": "https://x.test/hook"},
        },
    )

    project_id = _project(client)
    _complete(client, project_id)
    other = client.post(
        "/projects", json={"title": "stopped thing", "origin": "start", "goal": "finish"}
    ).json()["project"]["id"]
    client.put(f"/projects/{other}/status", json={"status": "stopped"})

    assert sent == [
        ("chan_003", "project_created", project_id, "solved thing"),
        ("chan_001", "project_completed", project_id, "solved thing"),
        ("chan_003", "project_created", other, "stopped thing"),
    ]


def test_stop_and_resume_emit_their_own_events(
    client: TestClient, sent: list[tuple[str, str, str, str]]
) -> None:
    client.post("/notifications", json={**DISCORD, "events": ["project_stopped", "project_resumed"]})
    project_id = _project(client)

    client.put(f"/projects/{project_id}/status", json={"status": "stopped"})
    client.put(f"/projects/{project_id}/status", json={"status": "active"})

    assert [call[1] for call in sent] == ["project_stopped", "project_resumed"]


def test_reopen_emits_with_the_correction(
    client: TestClient, sent: list[tuple[str, str, str, str]]
) -> None:
    client.post("/notifications", json={**DISCORD, "events": ["project_reopened"]})
    project_id = _project(client)
    _complete(client, project_id)

    response = client.post(
        f"/projects/{project_id}/reopen",
        json={"description": "flag was wrong", "creator": "human"},
    )
    assert response.status_code == 200
    assert sent == [("chan_001", "project_reopened", project_id, "solved thing")]


def test_test_endpoint_sends_even_for_a_disabled_channel(
    client: TestClient, sent: list[tuple[str, str, str, str]]
) -> None:
    channel = client.post(
        "/notifications", json={**DISCORD, "enabled": False}
    ).json()

    response = client.post(f"/notifications/{channel['id']}/test")

    assert response.status_code == 200
    assert response.json()["ok"] is True
    assert sent == [("chan_001", "test", "proj_000", "team discord")]


def test_payloads_are_shaped_per_kind() -> None:
    discord = notifications.TargetChannel(
        id="c", name="n", kind="discord", events=["project_completed"],
        config={"webhook_url": "https://d.test/hook"},
    )
    telegram = notifications.TargetChannel(
        id="c", name="n", kind="telegram", events=["project_completed"],
        config={"bot_token": "1:a", "chat_id": "-1"},
    )
    webhook = notifications.TargetChannel(
        id="c", name="n", kind="webhook", events=["project_completed"],
        config={"url": "https://w.test/hook"},
    )

    assert notifications._payload(discord, "e", "p", "t", "body") == (
        "https://d.test/hook",
        {"content": "body", "allowed_mentions": {"parse": []}},
    )
    assert notifications._payload(telegram, "e", "p", "t", "body")[0] == (
        "https://api.telegram.org/bot1:a/sendMessage"
    )
    assert notifications._payload(webhook, "e", "p", "t", "body")[1] == {
        "event": "e",
        "project_id": "p",
        "title": "t",
        "text": "body",
    }


def test_send_survives_a_transport_error(monkeypatch) -> None:
    def boom(*_args, **_kwargs):
        raise notifications.requests.RequestException("connection refused")

    monkeypatch.setattr(notifications.requests, "post", boom)
    channel = notifications.TargetChannel(
        id="c", name="n", kind="webhook", events=["project_completed"],
        config={"url": "https://w.test/hook"},
    )

    result = notifications.send(channel, "project_completed", "p", "t", "body")

    assert result["ok"] is False
    assert "connection refused" in result["detail"]
