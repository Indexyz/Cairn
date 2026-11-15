"""Outbound notifications for project state changes.

Channels are stored in the database and read on every event, so edits apply
immediately and a broken channel never blocks the write that triggered it:
delivery runs as a FastAPI background task after the response is sent.
"""

from __future__ import annotations

import json
import logging
import sqlite3
import time
from dataclasses import dataclass
from datetime import datetime, timezone

import requests
from fastapi import BackgroundTasks

log = logging.getLogger(__name__)

TIMEOUT = 10.0

# Config keys each channel kind needs, in the order the UI renders them.
KIND_FIELDS: dict[str, tuple[str, ...]] = {
    "discord": ("webhook_url",),
    "telegram": ("bot_token", "chat_id"),
    "webhook": ("url",),
}

URL_FIELDS = {"webhook_url", "url"}

EVENT_LABELS: dict[str, str] = {
    "project_created": "Project created",
    "project_stopped": "Project paused",
    "project_resumed": "Project resumed",
    "project_blocked": "Project blocked (step limit reached)",
    "project_completed": "Project completed",
    "project_reopened": "Project reopened",
}

EVENTS: tuple[str, ...] = tuple(EVENT_LABELS)


@dataclass
class TargetChannel:
    id: str
    name: str
    kind: str
    events: list[str]
    config: dict[str, str]


def utcnow() -> str:
    return datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")


def validate_config(kind: str, config: dict[str, str]) -> dict[str, str]:
    """Normalize a channel config to exactly the keys its kind needs."""
    fields = KIND_FIELDS.get(kind)
    if fields is None:
        raise ValueError(f"unknown channel kind: {kind}")

    cleaned: dict[str, str] = {}
    for key in fields:
        value = str(config.get(key) or "").strip()
        if not value:
            raise ValueError(f"{key} must not be empty")
        if key in URL_FIELDS and not value.startswith(("http://", "https://")):
            raise ValueError(f"{key} must be an http(s) URL")
        cleaned[key] = value
    return cleaned


def channel_from_row(row: sqlite3.Row) -> TargetChannel:
    return TargetChannel(
        id=row["id"],
        name=row["name"],
        kind=row["kind"],
        events=json.loads(row["events"]),
        config=json.loads(row["config"]),
    )


def render_message(event: str, project_id: str, title: str, detail: str = "") -> str:
    lines = [
        f"[Cairn] {EVENT_LABELS.get(event, event)}",
        f"{project_id} · {title}",
    ]
    if detail.strip():
        lines.append(detail.strip())
    lines.append(utcnow())
    return "\n".join(lines)


def render_test(channel_name: str) -> str:
    return "\n".join(
        [
            "[Cairn] Test notification",
            channel_name,
            "If you can read this, the channel is configured correctly.",
            utcnow(),
        ]
    )


def _payload(channel: TargetChannel, event: str, project_id: str, title: str, text: str):
    if channel.kind == "discord":
        url = channel.config["webhook_url"]
        body = {"content": text, "allowed_mentions": {"parse": []}}
    elif channel.kind == "telegram":
        url = f"https://api.telegram.org/bot{channel.config['bot_token']}/sendMessage"
        body = {
            "chat_id": channel.config["chat_id"],
            "text": text,
            "disable_web_page_preview": True,
        }
    else:
        url = channel.config["url"]
        body = {"event": event, "project_id": project_id, "title": title, "text": text}
    return url, body


def send(channel: TargetChannel, event: str, project_id: str, title: str, text: str) -> dict:
    """Deliver one message; never raises so a background batch always finishes."""
    url, body = _payload(channel, event, project_id, title, text)
    started = time.perf_counter()
    try:
        response = requests.post(url, json=body, timeout=TIMEOUT)
    except requests.RequestException as exc:
        log.warning("notification channel %s failed: %s", channel.id, exc)
        return {"ok": False, "status": None, "detail": str(exc), "duration_ms": 0}

    duration_ms = int((time.perf_counter() - started) * 1000)
    if not response.ok:
        log.warning(
            "notification channel %s answered %s: %s",
            channel.id,
            response.status_code,
            response.text[:300],
        )
    return {
        "ok": response.ok,
        "status": response.status_code,
        "detail": response.text[:300],
        "duration_ms": duration_ms,
    }


def notify(
    background: BackgroundTasks,
    conn: sqlite3.Connection,
    event: str,
    *,
    project_id: str,
    title: str,
    detail: str = "",
) -> None:
    """Queue a project event for every enabled channel subscribed to it."""
    channels = [
        channel_from_row(row)
        for row in conn.execute(
            "SELECT * FROM notification_channels WHERE enabled = 1 ORDER BY created_at, id"
        ).fetchall()
    ]
    targets = [channel for channel in channels if event in channel.events]
    if not targets:
        return
    text = render_message(event, project_id, title, detail)
    background.add_task(_deliver_all, targets, event, project_id, title, text)


def _deliver_all(
    targets: list[TargetChannel], event: str, project_id: str, title: str, text: str
) -> None:
    for channel in targets:
        send(channel, event, project_id, title, text)
