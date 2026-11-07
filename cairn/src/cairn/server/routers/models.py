from __future__ import annotations

import json
import sqlite3
import time

from fastapi import APIRouter, HTTPException, Query

from cairn.dispatcher.workers.registry import get_driver
from cairn.server.db import get_conn
from cairn.server.models import ModelTestResult, ReplaceModelsRequest, WorkerModel
from cairn.server.services import utcnow

router = APIRouter(tags=["models"])


def _row_to_model(row: sqlite3.Row) -> WorkerModel:
    return WorkerModel(
        name=row["name"],
        type=row["type"],
        task_types=json.loads(row["task_types"]),
        max_running=row["max_running"],
        priority=row["priority"],
        env=json.loads(row["env"]),
    )


def _list_models(conn: sqlite3.Connection) -> list[WorkerModel]:
    rows = conn.execute("SELECT * FROM models ORDER BY priority, name").fetchall()
    return [_row_to_model(row) for row in rows]


def _insert_model(conn: sqlite3.Connection, model: WorkerModel, now: str) -> None:
    conn.execute(
        "INSERT INTO models (name, type, task_types, max_running, priority, env, created_at, updated_at)"
        " VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
        (
            model.name,
            model.type,
            json.dumps(model.task_types),
            model.max_running,
            model.priority,
            json.dumps(model.env),
            now,
            now,
        ),
    )


@router.get("/models", response_model=list[WorkerModel])
def list_models():
    with get_conn() as conn:
        return _list_models(conn)


@router.post("/models", response_model=WorkerModel, status_code=201)
def create_model(body: WorkerModel):
    with get_conn() as conn:
        existing = conn.execute("SELECT 1 FROM models WHERE name = ?", (body.name,)).fetchone()
        if existing is not None:
            raise HTTPException(409, f"model {body.name} already exists")
        _insert_model(conn, body, utcnow())
        return body


@router.put("/models", response_model=list[WorkerModel])
def replace_models(body: ReplaceModelsRequest):
    with get_conn() as conn:
        now = utcnow()
        keep = {model.name for model in body.models}
        existing = {row["name"] for row in conn.execute("SELECT name FROM models").fetchall()}
        for name in existing - keep:
            conn.execute("DELETE FROM models WHERE name = ?", (name,))
        for model in body.models:
            conn.execute(
                """
                INSERT INTO models (name, type, task_types, max_running, priority, env, created_at, updated_at)
                VALUES (?, ?, ?, ?, ?, ?, ?, ?)
                ON CONFLICT(name) DO UPDATE SET
                    type = excluded.type,
                    task_types = excluded.task_types,
                    max_running = excluded.max_running,
                    priority = excluded.priority,
                    env = excluded.env,
                    updated_at = excluded.updated_at
                """,
                (
                    model.name,
                    model.type,
                    json.dumps(model.task_types),
                    model.max_running,
                    model.priority,
                    json.dumps(model.env),
                    now,
                    now,
                ),
            )
        return _list_models(conn)


@router.put("/models/{name}", response_model=WorkerModel)
def update_model(name: str, body: WorkerModel):
    with get_conn() as conn:
        existing = conn.execute("SELECT * FROM models WHERE name = ?", (name,)).fetchone()
        if existing is None:
            raise HTTPException(404, f"model {name} not found")
        if body.name != name:
            conflict = conn.execute("SELECT 1 FROM models WHERE name = ?", (body.name,)).fetchone()
            if conflict is not None:
                raise HTTPException(409, f"model {body.name} already exists")
        conn.execute(
            "UPDATE models SET name = ?, type = ?, task_types = ?, max_running = ?, priority = ?,"
            " env = ?, updated_at = ? WHERE name = ?",
            (
                body.name,
                body.type,
                json.dumps(body.task_types),
                body.max_running,
                body.priority,
                json.dumps(body.env),
                utcnow(),
                name,
            ),
        )
        return body


def _require_model(conn: sqlite3.Connection, name: str) -> WorkerModel:
    row = conn.execute("SELECT * FROM models WHERE name = ?", (name,)).fetchone()
    if row is None:
        raise HTTPException(404, f"model {name} not found")
    return _row_to_model(row)


@router.delete("/models/{name}", status_code=204)
def delete_model(name: str):
    with get_conn() as conn:
        _require_model(conn, name)
        conn.execute("DELETE FROM models WHERE name = ?", (name,))


@router.post("/models/{name}/test", response_model=ModelTestResult)
def test_model(name: str, timeout: float = Query(default=10.0, gt=0, le=60)):
    """Run the dispatcher's own worker healthcheck for this model, from the server host."""
    with get_conn() as conn:
        model = _require_model(conn, name)

    driver = get_driver(model.type)
    started = time.perf_counter()
    try:
        result = driver.check_health(model, timeout=timeout)
    except KeyError as exc:
        raise HTTPException(422, f"model {name} is missing env key {exc.args[0]}") from exc
    except ValueError as exc:
        raise HTTPException(422, str(exc)) from exc
    duration_ms = int((time.perf_counter() - started) * 1000)

    return ModelTestResult(
        ok=result.ok,
        status=result.status,
        detail=result.detail,
        endpoint=driver.describe_health(model),
        duration_ms=duration_ms,
    )
