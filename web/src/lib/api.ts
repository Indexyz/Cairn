export async function api<T>(method: string, path: string, body?: unknown): Promise<T> {
  const options: RequestInit = { method, headers: { "Content-Type": "application/json" } };
  if (body !== undefined) options.body = JSON.stringify(body);
  const response = await fetch(path, options);
  if (response.status === 204) return null as T;
  const text = await response.text();
  let data: unknown = null;
  if (text) {
    try {
      data = JSON.parse(text);
    } catch {
      data = null;
    }
  }
  if (!response.ok) {
    const detail = (data as { detail?: unknown } | null)?.detail;
    let message = `HTTP ${response.status}`;
    if (typeof detail === "string") message = detail;
    else if (Array.isArray(detail)) {
      message = detail.map((item) => (item as { msg?: string }).msg ?? "invalid value").join("; ");
    }
    throw new Error(message);
  }
  return data as T;
}

export async function fetchText(path: string): Promise<string> {
  const response = await fetch(path);
  const text = await response.text();
  if (!response.ok) {
    let detail = text;
    try {
      const data = JSON.parse(text) as { detail?: string };
      detail = data.detail ?? text;
    } catch {
      /* keep raw text */
    }
    throw new Error(detail || `HTTP ${response.status}`);
  }
  return text;
}

export async function copyText(text: string): Promise<void> {
  if (navigator.clipboard?.writeText) {
    await navigator.clipboard.writeText(text);
    return;
  }
  const textarea = document.createElement("textarea");
  textarea.value = text;
  textarea.setAttribute("readonly", "");
  textarea.style.position = "fixed";
  textarea.style.left = "-9999px";
  document.body.appendChild(textarea);
  textarea.select();
  try {
    if (!document.execCommand("copy")) throw new Error("copy command rejected");
  } finally {
    document.body.removeChild(textarea);
  }
}
