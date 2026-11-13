export function downloadText(filename: string, text: string, mimeType = "text/yaml;charset=utf-8"): void {
  const url = URL.createObjectURL(new Blob([text], { type: mimeType }));
  const anchor = document.createElement("a");
  anchor.href = url;
  anchor.download = filename;
  anchor.rel = "noopener";
  document.body.appendChild(anchor);
  anchor.click();
  anchor.remove();
  URL.revokeObjectURL(url);
}

export function snapshotFilename(projectId: string, extension = "yaml"): string {
  return `${projectId.replace(/[^a-zA-Z0-9._-]+/g, "-")}-snapshot.${extension}`;
}
