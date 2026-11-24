import { expect, test, vi } from "vitest";
import { render } from "vitest-browser-react";

import { ToastProvider } from "@/hooks/use-toast";
import { MCPPage } from "@/pages/mcp";

function json(body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { "content-type": "application/json" },
  });
}

function server(name: string, command: string) {
  return {
    name,
    type: "stdio",
    command,
    args: ["-y", "pkg"],
    env: { TOKEN: "x" },
    url: "",
    headers: {},
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:00:00Z",
  };
}

function mount() {
  render(
    <ToastProvider>
      <MCPPage />
    </ToastProvider>,
  );
}

test("mcp servers render with their target", async () => {
  vi.stubGlobal(
    "fetch",
    vi.fn(async () => json([server("fs", "mcp-fs")])),
  );

  mount();

  await vi.waitFor(() => expect(document.body.textContent).toContain("fs"), { timeout: 5000 });
  expect(document.body.textContent).toContain("mcp-fs -y pkg");
  expect(document.body.textContent).toContain("stdio");
});

test("editing a server keeps the name and sends the trimmed payload", async () => {
  const bodies: string[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (_input: RequestInfo | URL, init?: RequestInit) => {
      if (init?.method === "PUT") {
        bodies.push(String(init.body ?? ""));
        return json(server("fs", "uvx"));
      }
      return json([server("fs", "mcp-fs")]);
    }),
  );

  mount();

  await vi.waitFor(() => expect(document.body.textContent).toContain("fs"), { timeout: 5000 });
  const edit = [...document.querySelectorAll("button")].find(
    (button) => button.getAttribute("title") === "Edit",
  );
  edit!.click();

  const command = await vi.waitFor(() => {
    const input = document.querySelector<HTMLInputElement>("#mcp-command");
    expect(input).toBeTruthy();
    return input!;
  }, { timeout: 5000 });
  expect(command.value).toBe("mcp-fs");

  const setter = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, "value")!.set!;
  setter.call(command, "uvx");
  command.dispatchEvent(new Event("input", { bubbles: true }));

  const save = [...document.querySelectorAll<HTMLButtonElement>('[role="dialog"] button')].find(
    (button) => button.textContent?.trim() === "Save",
  );
  await vi.waitFor(() => expect(save!.disabled).toBe(false), { timeout: 5000 });
  save!.click();

  await vi.waitFor(() => expect(bodies.length).toBe(1), { timeout: 5000 });
  expect(JSON.parse(bodies[0])).toMatchObject({ name: "fs", command: "uvx", args: ["-y", "pkg"] });
});
