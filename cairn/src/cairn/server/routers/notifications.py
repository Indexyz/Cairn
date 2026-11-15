from __future__ import annotations

import json
import sqlite3

from fastapi import APIRouter, HTTPException

from cairn.server import notifications
from cairn.server.db import get_conn
from cairn.server.models import (
    NotificationChannel,
    NotificationChannelPayload,
    NotificationTestResult,
)
from cairn.server.services import next_channel_id, utcnow

router = APIRouter(tags=["notifications"])


def _row_to_channel(row: sqlite3.Row) -> NotificationChannel:
    return NotificationChannel(
        id=row["id"],
        name=row["name"],
        kind=row["kind"],
        enabled=bool(row["enabled"]),
        events=json.loads(row["events"]),
        config=json.loads(row["config"]),
        created_at=row["created_at"],
        updated_at=row["updated_at"],
    )


def _require_channel(conn: sqlite3.Connection, channel_id: str) -> sqlite3.Row:
    row = conn.execute("SELECT * FROM notification_channels WHERE id = ?", (channel_id,)).fetchone()
    if row is None:
        raise HTTPException(404, f"channel {channel_id} not found")
    return row


@router.get("/notifications", response_model=list[NotificationChannel])
def list_channels():
    with get_conn() as conn:
        rows = conn.execute(
            "SELECT * FROM notification_channels ORDER BY created_at, id"
        ).fetchall()
        return [_row_to_channel(row) for row in rows]


@router.post("/notifications", response_model=NotificationChannel, status_code=201)
def create_channel(body: NotificationChannelPayload):
    with get_conn() as conn:
        now = utcnow()
        channel_id = next_channel_id(conn)
        conn.execute(
            "INSERT INTO notification_channels (id, name, kind, enabled, events, config, created_at, updated_at)"
            " VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
            (
                channel_id,
                body.name,
                body.kind,
                int(body.enabled),
                json.dumps(body.events),
                json.dumps(body.config),
                now,
                now,
            ),
        )
        return _row_to_channel(_require_channel(conn, channel_id))


@router.put("/notifications/{channel_id}", response_model=NotificationChannel)
def update_channel(channel_id: str, body: NotificationChannelPayload):
    with get_conn() as conn:
        _require_channel(conn, channel_id)
        conn.execute(
            "UPDATE notification_channels SET name = ?, kind = ?, enabled = ?, events = ?,"
            " config = ?, updated_at = ? WHERE id = ?",
            (
                body.name,
                body.kind,
                int(body.enabled),
                json.dumps(body.events),
                json.dumps(body.config),
                utcnow(),
                channel_id,
            ),
        )
        return _row_to_channel(_require_channel(conn, channel_id))


@router.delete("/notifications/{channel_id}", status_code=204)
def delete_channel(channel_id: str):
    with get_conn() as conn:
        _require_channel(conn, channel_id)
        conn.execute("DELETE FROM notification_channels WHERE id = ?", (channel_id,))


@router.post("/notifications/{channel_id}/test", response_model=NotificationTestResult)
def test_channel(channel_id: str):
    """Send a sample message to one channel, regardless of its enabled flag."""
    with get_conn() as conn:
        channel = notifications.channel_from_row(_require_channel(conn, channel_id))

    text = notifications.render_test(channel.name)
    return NotificationTestResult(
        **notifications.send(channel, "test", "proj_000", channel.name, text)
    )
