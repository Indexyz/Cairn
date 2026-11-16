from __future__ import annotations

from cairn.server.models import WorkerModel


def model(worker_type: str, env: dict[str, str], name: str | None = None) -> WorkerModel:
    return WorkerModel(
        name=name or worker_type,
        type=worker_type,
        task_types=["reason"],
        max_running=1,
        priority=0,
        env=env,
    )
