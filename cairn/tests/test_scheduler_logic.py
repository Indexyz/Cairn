from __future__ import annotations

from concurrent.futures import Future
import logging
from pathlib import Path

import pytest
import requests

from cairn.dispatcher.models import ReasonCheckpoint, RunningTask
from cairn.dispatcher.runtime.cancellation import TaskCancellation
from cairn.dispatcher.scheduler.loop import DispatcherLoop
from cairn.dispatcher.scheduler.worker_select import choose_worker
from cairn.server.models import Fact, ProjectSummary

from conftest import make_config, make_intent, make_project


def _loop() -> DispatcherLoop:
    loop = DispatcherLoop.__new__(DispatcherLoop)
    loop.reason_checkpoints = {}
    loop.runtime_project_ids = set()
    loop.cleanup_futures = {}
    loop._cleanup_pending = set()
    loop._inactive_cleanup_done = {}
    loop.worker_unhealthy_until = {}
    loop.worker_rejected_until = {}
    loop._log_state = {}
    loop.project_cursor = 0
    return loop


def _summary(project_id: str, status: str) -> ProjectSummary:
    return ProjectSummary(
        id=project_id,
        title=project_id,
        status=status,
        bootstrap_enabled=True,
        created_at="2026-01-01T00:00:00Z",
        fact_count=2,
        intent_count=0,
        working_intent_count=0,
        unclaimed_intent_count=0,
        hint_count=0,
    )


def test_reason_trigger_detects_new_facts_and_open_intent_completion() -> None:
    loop = _loop()
    project = make_project(intents=[make_intent()])
    loop.reason_checkpoints["proj_001"] = ReasonCheckpoint(
        fact_count=3,
        hint_count=1,
        open_intent_count=1,
    )
    project.facts.append(Fact(id="f002", description="new"))
    project.intents = []

    assert loop._reason_trigger(project) == "facts:3->4,open_intents:1->0"


def test_reason_trigger_returns_none_when_graph_is_unchanged() -> None:
    loop = _loop()
    project = make_project(intents=[make_intent()])
    loop.reason_checkpoints["proj_001"] = ReasonCheckpoint(
        fact_count=3,
        hint_count=1,
        open_intent_count=1,
    )

    assert loop._reason_trigger(project) is None


def test_refresh_runtime_projects_discards_active_and_changed_cleanup_markers() -> None:
    loop = _loop()
    loop.runtime_project_ids = {"active", "stopped", "deleted"}
    loop._inactive_cleanup_done = {
        "active": "stopped",
        "stopped": "stopped",
        "changed": "completed",
        "deleted": "completed",
    }

    loop._refresh_runtime_projects(
        [
            _summary("active", "active"),
            _summary("stopped", "stopped"),
            _summary("changed", "stopped"),
        ]
    )

    assert loop.runtime_project_ids == {"active"}
    assert loop._inactive_cleanup_done == {"stopped": "stopped"}


def test_reap_cleanup_future_records_only_successful_inactive_cleanup() -> None:
    loop = _loop()
    succeeded: Future[bool] = Future()
    failed: Future[bool] = Future()
    succeeded.set_result(True)
    failed.set_result(False)
    loop.cleanup_futures = {
        succeeded: ("container-success", "proj-success", "completed"),
        failed: ("container-failed", "proj-failed", "stopped"),
    }
    loop._cleanup_pending = {"container-success", "container-failed"}
    loop._inactive_cleanup_done = {"proj-failed": "stopped"}

    loop._reap_cleanup_futures()

    assert loop.cleanup_futures == {}
    assert loop._cleanup_pending == set()
    assert loop._inactive_cleanup_done == {"proj-success": "completed"}


def test_choose_worker_prefers_priority_then_lower_running_count() -> None:
    workers = make_config().workers
    first = workers[0].model_copy(update={"name": "first", "priority": 0})
    busy = workers[0].model_copy(update={"name": "busy", "priority": 0})
    lower_priority = workers[0].model_copy(update={"name": "lower", "priority": 1})

    ordered = choose_worker(
        [lower_priority, busy, first],
        {"busy": 2, "first": 0, "lower": 0},
    )

    assert [worker.name for worker in ordered] == ["first", "busy", "lower"]


def test_new_fact_dispatches_reason_before_unclaimed_explore_intent() -> None:
    loop = _loop()
    loop.config = make_config()
    loop.futures = {}
    project = make_project(intents=[make_intent()])
    project.intents[0].worker = None
    project.facts.append(Fact(id="f002", description="new"))
    loop.reason_checkpoints["proj_001"] = ReasonCheckpoint(
        fact_count=3,
        hint_count=1,
        open_intent_count=1,
    )
    loop.container_manager = type("Containers", (), {"container_name": lambda _self, project_id: project_id})()
    loop.client = type(
        "Client",
        (),
        {
            "get_project": lambda _self, _project_id: project,
            "export_project": lambda _self, _project_id: "graph",
        },
    )()
    dispatched: list[tuple[str, str]] = []
    loop._dispatch_reason = lambda _project, _graph, trigger: dispatched.append(("reason", trigger)) or True
    loop._dispatch_explore = lambda *_args: dispatched.append(("explore", "")) or True

    assert loop._try_dispatch_project(_summary("proj_001", "active"))
    assert dispatched == [("reason", "facts:3->4")]


def test_initial_enabled_project_without_bootstrap_worker_dispatches_reason() -> None:
    loop = _loop()
    config = make_config()
    loop.config = config.model_copy(
        update={
            "workers": [
                config.workers[0].model_copy(update={"task_types": ["reason", "explore"]})
            ]
        }
    )
    loop.futures = {}
    project = make_project()
    project.facts = project.facts[:2]
    loop.container_manager = type("Containers", (), {"container_name": lambda _self, project_id: project_id})()
    loop.client = type(
        "Client",
        (),
        {
            "get_project": lambda _self, _project_id: project,
            "export_project": lambda _self, _project_id: "graph",
        },
    )()
    dispatched: list[tuple[str, str]] = []
    loop._dispatch_initial_project = lambda _project: dispatched.append(("bootstrap", "")) or True
    loop._dispatch_reason = lambda _project, _graph, trigger: dispatched.append(("reason", trigger)) or True

    assert loop._try_dispatch_project(_summary("proj_001", "active"))
    assert dispatched == [("reason", "initial")]


def test_initial_disabled_project_skips_configured_bootstrap_worker() -> None:
    loop = _loop()
    loop.config = make_config()
    loop.futures = {}
    project = make_project()
    project.project.bootstrap_enabled = False
    project.facts = project.facts[:2]
    loop.container_manager = type("Containers", (), {"container_name": lambda _self, project_id: project_id})()
    loop.client = type(
        "Client",
        (),
        {
            "get_project": lambda _self, _project_id: project,
            "export_project": lambda _self, _project_id: "graph",
        },
    )()
    dispatched: list[tuple[str, str]] = []
    loop._dispatch_initial_project = lambda _project: dispatched.append(("bootstrap", "")) or True
    loop._dispatch_reason = lambda _project, _graph, trigger: dispatched.append(("reason", trigger)) or True

    assert loop._try_dispatch_project(_summary("proj_001", "active"))
    assert dispatched == [("reason", "initial")]


def test_initial_enabled_project_without_bootstrap_worker_skips_bootstrap() -> None:
    loop = _loop()
    config = make_config()
    loop.config = config.model_copy(
        update={
            "workers": [
                config.workers[0].model_copy(update={"task_types": ["reason", "explore"]})
            ]
        }
    )
    project = make_project()
    project.project.bootstrap_enabled = True
    project.facts = project.facts[:2]

    assert not loop._project_requires_bootstrap(project)


def test_initial_enabled_project_keeps_existing_bootstrap_intent_when_workers_change() -> None:
    loop = _loop()
    config = make_config()
    loop.config = config.model_copy(
        update={
            "workers": [
                config.workers[0].model_copy(update={"task_types": ["reason", "explore"]})
            ]
        }
    )
    project = make_project(intents=[make_intent()])
    project.project.bootstrap_enabled = True
    project.facts = project.facts[:2]
    project.intents[0].description = "bootstrap"
    project.intents[0].creator = "dispatcher.bootstrap"
    project.intents[0].from_ = ["origin"]

    assert loop._project_requires_bootstrap(project)


def test_cancel_inactive_tasks_marks_stopped_and_deleted_projects() -> None:
    loop = _loop()
    stopped = TaskCancellation()
    deleted = TaskCancellation()
    loop.futures = {
        Future(): RunningTask("stopped", "explore", "worker", stopped),
        Future(): RunningTask("deleted", "reason", "worker", deleted),
    }

    loop._cancel_inactive_tasks([_summary("stopped", "stopped")])

    assert stopped.reason == "stopped"
    assert deleted.reason == "deleted"


def test_initialize_reason_checkpoint_only_for_active_projects_with_open_intents() -> None:
    loop = _loop()
    active = _summary("active", "active")
    active.unclaimed_intent_count = 1

    loop._initialize_reason_checkpoints(
        [
            active,
            _summary("idle", "active"),
            _summary("stopped", "stopped"),
        ]
    )

    assert loop.reason_checkpoints == {
        "active": ReasonCheckpoint(fact_count=2, hint_count=0, open_intent_count=1)
    }


def test_select_worker_reports_busy_unhealthy_rejected_and_unsupported_workers(monkeypatch) -> None:
    loop = _loop()
    base = make_config()
    busy = base.workers[0].model_copy(update={"name": "busy", "task_types": ["reason"]})
    unhealthy = base.workers[0].model_copy(update={"name": "unhealthy", "task_types": ["reason"]})
    rejected = base.workers[0].model_copy(update={"name": "rejected", "task_types": ["reason"]})
    unsupported = base.workers[0].model_copy(update={"name": "unsupported", "task_types": ["explore"]})
    loop.config = base.model_copy(update={"workers": [busy, unhealthy, rejected, unsupported]})
    loop.futures = {Future(): RunningTask("proj", "reason", "busy", TaskCancellation())}
    loop.worker_unhealthy_until = {"unhealthy": 110.0}
    loop.worker_rejected_until = {("proj", "reason", "rejected"): 120.0}
    monkeypatch.setattr("cairn.dispatcher.scheduler.loop.time.time", lambda: 100.0)

    selection = loop._select_worker("proj", "reason")

    assert selection.worker is None
    assert selection.blocked_busy == ["busy(1/1)"]
    assert selection.blocked_unhealthy == ["unhealthy(10.0s)"]
    assert selection.blocked_rejected == ["rejected(20.0s)"]
    assert selection.blocked_task_type == ["unsupported"]


def test_disabled_worker_healthcheck_skips_automatic_startup_but_force_runs_diagnostic() -> None:
    loop = _loop()
    config = make_config()
    loop.config = config.model_copy(
        update={"runtime": config.runtime.model_copy(update={"worker_healthcheck": "disabled"})}
    )
    calls: list[bool] = []
    loop._run_startup_healthchecks = lambda *, show_commands: calls.append(show_commands)
    loop._startup_healthchecks_checked = False

    loop.run_startup_healthchecks()

    assert calls == []
    assert loop._startup_healthchecks_checked

    loop._startup_healthchecks_checked = False
    loop.run_startup_healthchecks(show_commands=True, force=True)

    assert calls == [True]


def test_startup_only_worker_healthcheck_runs_automatic_startup_check() -> None:
    loop = _loop()
    config = make_config()
    loop.config = config.model_copy(
        update={"runtime": config.runtime.model_copy(update={"worker_healthcheck": "startup_only"})}
    )
    calls: list[bool] = []
    loop._run_startup_healthchecks = lambda *, show_commands: calls.append(show_commands)
    loop._startup_healthchecks_checked = False

    loop.run_startup_healthchecks()

    assert calls == [False]


class _ModelsClient:
    def __init__(self, models: list | None = None, *, fail: bool = False):
        self.models = models or []
        self.fail = fail
        self.replaced: list[list] = []

    def list_models(self) -> list:
        if self.fail:
            raise requests.RequestException("boom")
        return list(self.models)

    def replace_models(self, models: list) -> list:
        self.replaced.append(list(models))
        self.models = list(models)
        return list(models)


def _model(name: str, priority: int = 0):
    from cairn.dispatcher.config import WorkerConfig

    return WorkerConfig(
        name=name,
        type="mock",
        task_types=["reason"],
        max_running=1,
        priority=priority,
    )


def _sync_loop(client: _ModelsClient, workers: list) -> DispatcherLoop:
    loop = _loop()
    loop.config = make_config().model_copy(update={"workers": workers})
    loop.config_path = Path("dispatch.yaml")
    loop.client = client
    loop._startup_healthchecks_checked = False
    return loop


def test_sync_workers_seeds_empty_database_from_yaml() -> None:
    worker = _model("from-yaml")
    client = _ModelsClient()
    loop = _sync_loop(client, [worker])

    loop._sync_workers(seed=True)

    assert [model.name for model in client.replaced[0]] == ["from-yaml"]
    assert [model.name for model in client.models] == ["from-yaml"]
    assert [model.name for model in loop.config.workers] == ["from-yaml"]


def test_sync_workers_hot_reloads_and_clears_health_flags() -> None:
    client = _ModelsClient([_model("old")])
    loop = _sync_loop(client, [])
    loop.worker_unhealthy_until["old"] = 99.0
    loop.worker_rejected_until[("proj_001", "reason", "old")] = 99.0

    loop._sync_workers()
    assert [model.name for model in loop.config.workers] == ["old"]

    client.models = [_model("new")]
    loop.worker_unhealthy_until["old"] = 99.0
    loop._sync_workers()

    assert [model.name for model in loop.config.workers] == ["new"]
    assert loop.worker_unhealthy_until == {}
    assert loop.worker_rejected_until == {}


def test_sync_workers_merges_common_env() -> None:
    client = _ModelsClient([_model("worker")])
    loop = _sync_loop(client, [])
    loop.config = loop.config.model_copy(update={"common_env": {"SHARED": "yes"}})

    loop._sync_workers()

    assert loop.config.workers[0].env == {"SHARED": "yes"}


def test_sync_workers_keeps_previous_set_on_request_failure() -> None:
    client = _ModelsClient([_model("stable")])
    loop = _sync_loop(client, [])
    loop._sync_workers()

    client.fail = True
    loop._sync_workers()

    assert [model.name for model in loop.config.workers] == ["stable"]


def test_sync_workers_idles_when_no_models_are_configured(caplog) -> None:
    loop = _sync_loop(_ModelsClient(), [])

    with caplog.at_level(logging.WARNING, logger="cairn.dispatcher.scheduler.loop"):
        loop._sync_workers()
        loop._sync_workers()

    assert loop.config.workers == []
    assert "workers/idle" in loop._log_state
    # The warning is emitted once, not on every tick.
    warnings = [record for record in caplog.records if "idling" in record.getMessage()]
    assert len(warnings) == 1


def test_sync_workers_resumes_after_models_are_added() -> None:
    client = _ModelsClient()
    loop = _sync_loop(client, [])
    loop._sync_workers()

    client.models = [_model("added")]
    loop._sync_workers()

    assert [model.name for model in loop.config.workers] == ["added"]
    assert "workers/idle" not in loop._log_state


def test_sync_workers_survives_request_failure_without_previous_models() -> None:
    loop = _sync_loop(_ModelsClient(fail=True), [])

    loop._sync_workers()

    assert loop.config.workers == []


def test_startup_healthchecks_are_deferred_while_no_models_are_configured() -> None:
    loop = _sync_loop(_ModelsClient(), [])
    calls: list[bool] = []
    loop._run_startup_healthchecks = lambda *, show_commands: calls.append(show_commands)
    loop._startup_healthchecks_checked = False

    loop.run_startup_healthchecks()

    assert calls == []
    assert not loop._startup_healthchecks_checked


def test_dispatch_is_skipped_without_models(caplog) -> None:
    loop = _sync_loop(_ModelsClient(), [])
    calls: list[str] = []

    class _Client(_ModelsClient):
        def get_project(self, project_id: str):
            calls.append(project_id)
            return make_project()

    loop.client = _Client()

    with caplog.at_level(logging.INFO, logger="cairn.dispatcher.scheduler.loop"):
        loop._dispatch_available([_summary("proj_001", "active")])

    assert calls == []
    assert any("no worker models" in record.getMessage() for record in caplog.records)


def test_first_models_after_idle_run_startup_healthchecks() -> None:
    client = _ModelsClient()
    loop = _sync_loop(client, [])
    loop._sync_workers()
    calls: list[bool] = []
    loop.run_startup_healthchecks = lambda *, show_commands=False, force=False: calls.append(show_commands)

    client.models = [_model("added")]
    loop._sync_workers()

    assert calls == [False]
    assert [model.name for model in loop.config.workers] == ["added"]
