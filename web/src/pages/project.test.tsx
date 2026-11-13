import { beforeEach, expect, test, vi } from "vitest";
import { render } from "vitest-browser-react";
import { MemoryRouter, Route, Routes } from "react-router-dom";

import { PrefsProvider } from "@/hooks/use-prefs";
import { ToastProvider } from "@/hooks/use-toast";
import { ProjectPage } from "@/pages/project";
import "@/styles.css";

declare global {
  interface Window {
    __cy?: { nodes: () => { length: number }; edges: () => { length: number } };
  }
}

const project = {
  project: {
    id: "proj_001",
    title: "graph",
    status: "active",
    bootstrap_enabled: false,
    created_at: "2026-01-01T00:00:00Z",
    reason: null,
  },
  facts: [
    { id: "origin", description: "start" },
    { id: "goal", description: "finish" },
  ],
  intents: [
    {
      id: "i001",
      from: ["origin"],
      to: null,
      description: "look around",
      creator: "human",
      worker: null,
      last_heartbeat_at: null,
      created_at: "2026-01-01T00:00:01Z",
      concluded_at: null,
    },
  ],
  hints: [],
};

function json(body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { "content-type": "application/json" },
  });
}

beforeEach(() => {
  vi.stubGlobal(
    "fetch",
    vi.fn(async (input: RequestInfo | URL) => {
      const url = String(input);
      if (url.endsWith("/projects/proj_001")) return json(project);
      return json({});
    }),
  );
});

test("renders the project graph nodes and edge", async () => {
  render(
    <div style={{ height: 800 }}>
      <ToastProvider>
        <PrefsProvider>
          <MemoryRouter initialEntries={["/projects/proj_001"]}>
            <Routes>
              <Route path="/projects/:projectId" element={<ProjectPage />} />
            </Routes>
          </MemoryRouter>
        </PrefsProvider>
      </ToastProvider>
    </div>,
  );

  await vi.waitFor(() => expect(window.__cy).toBeDefined(), { timeout: 5000 });
  const graph = window.__cy!;
  await vi.waitFor(() => expect(graph.nodes().length).toBe(3), { timeout: 5000 });

  expect(graph.edges().length).toBe(1);

  const container = document.querySelector<HTMLElement>('div[class*="h-full"][class*="w-full"]');
  expect(container).not.toBeNull();
  expect(container!.getBoundingClientRect().height).toBeGreaterThan(100);
  expect(container!.querySelectorAll("canvas").length).toBeGreaterThan(0);
});
