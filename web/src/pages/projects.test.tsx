import { expect, test, vi } from "vitest";
import { render } from "vitest-browser-react";
import { MemoryRouter } from "react-router-dom";

import { PrefsProvider } from "@/hooks/use-prefs";
import { ToastProvider } from "@/hooks/use-toast";
import { ProjectsPage } from "@/pages/projects";

function project(id: string, title: string, scheduleId: string | null) {
  return {
    id,
    title,
    status: "completed",
    bootstrap_enabled: false,
    max_steps: null,
    schedule_id: scheduleId,
    created_at: "2026-01-01T00:00:00Z",
    reason: null,
    fact_count: 2,
    intent_count: 1,
    working_intent_count: 0,
    unclaimed_intent_count: 0,
    hint_count: 0,
  };
}

function json(body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { "content-type": "application/json" },
  });
}

function stubRoutes(projects: unknown[], schedules: unknown[]) {
  vi.stubGlobal(
    "fetch",
    vi.fn(async (input: RequestInfo | URL) => {
      const url = String(input);
      if (url.endsWith("/schedules")) return json(schedules);
      if (url.endsWith("/projects")) return json(projects);
      return json({});
    }),
  );
}

function mount() {
  render(
    <ToastProvider>
      <PrefsProvider>
        <MemoryRouter initialEntries={["/"]}>
          <ProjectsPage />
        </MemoryRouter>
      </PrefsProvider>
    </ToastProvider>,
  );
}

test("projects started by a schedule collapse into a group", async () => {
  stubRoutes(
    [project("proj_001", "manual", null), project("proj_002", "run a", "sch_1"), project("proj_003", "run b", "sch_1")],
    [{ id: "sch_1", name: "nightly" }],
  );

  mount();

  await vi.waitFor(() => expect(document.body.textContent).toContain("nightly"), { timeout: 5000 });
  expect(document.body.textContent).toContain("2 runs");
  expect(document.body.textContent).toContain("manual");
  // Members stay collapsed until the group is opened.
  expect(document.body.textContent).not.toContain("proj_002");

  const toggle = [...document.querySelectorAll("button")].find((button) =>
    button.getAttribute("title")?.includes("Show"),
  );
  expect(toggle).toBeTruthy();
  toggle!.click();

  await vi.waitFor(() => expect(document.body.textContent).toContain("proj_002"), { timeout: 5000 });
  expect(document.body.textContent).toContain("proj_003");
  expect(document.body.textContent).toContain("run a");
});

test("projects without a schedule stay flat", async () => {
  stubRoutes([project("proj_001", "manual", null)], []);

  mount();

  await vi.waitFor(() => expect(document.body.textContent).toContain("manual"), { timeout: 5000 });
  expect(document.body.textContent).toContain("proj_001");
  expect(document.body.textContent).not.toContain("runs");
});

function detail(hints: unknown[]) {
  return {
    project: project("proj_001", "manual", null),
    facts: [
      { id: "origin", description: "start" },
      { id: "goal", description: "finish" },
    ],
    intents: [],
    hints,
  };
}

test("forking lets you pick which hints to copy", async () => {
  const bodies: string[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      if (url.endsWith("/schedules")) return json([]);
      if (url.endsWith("/projects")) return json([project("proj_001", "manual", null)]);
      if (url.endsWith("/projects/proj_001/fork")) {
        bodies.push(String(init?.body ?? ""));
        return json(detail([]));
      }
      if (url.endsWith("/projects/proj_001")) {
        return json(
          detail([
            { id: "h001", content: "keep clue", creator: "human", created_at: "2026-01-01T00:00:00Z" },
            { id: "h002", content: "drop clue", creator: "human", created_at: "2026-01-02T00:00:00Z" },
          ]),
        );
      }
      return json({});
    }),
  );

  mount();

  await vi.waitFor(() => expect(document.body.textContent).toContain("manual"), { timeout: 5000 });
  const fork = [...document.querySelectorAll("button")].find(
    (button) => button.getAttribute("title") === "Fork",
  );
  expect(fork).toBeTruthy();
  fork!.click();

  await vi.waitFor(() => expect(document.body.textContent).toContain("keep clue"), { timeout: 5000 });
  const checkboxes = [...document.querySelectorAll<HTMLInputElement>('[role="dialog"] input[type="checkbox"]')];
  expect(checkboxes.map((box) => box.checked)).toEqual([true, true]);

  checkboxes[1].click();
  await vi.waitFor(() => expect(checkboxes[1].checked).toBe(false));

  const confirm = [...document.querySelectorAll<HTMLButtonElement>('[role="dialog"] button')].find(
    (button) => button.textContent?.trim() === "Fork",
  );
  confirm!.click();

  await vi.waitFor(() => expect(bodies.length).toBe(1), { timeout: 5000 });
  expect(JSON.parse(bodies[0])).toEqual({ title: "manual (fork)", hint_ids: ["h001"] });
});
