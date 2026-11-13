import { expect, test } from "vitest";

import { downloadText, snapshotFilename } from "@/lib/download";

test("snapshot filenames keep the id and end in .yaml", () => {
  expect(snapshotFilename("proj_001")).toBe("proj_001-snapshot.yaml");
  expect(snapshotFilename("proj/1 b")).toBe("proj-1-b-snapshot.yaml");
});

test("downloadText clicks a link carrying the filename", () => {
  const downloaded: string[] = [];
  const original = HTMLAnchorElement.prototype.click;
  HTMLAnchorElement.prototype.click = function click() {
    downloaded.push(this.download);
  };

  try {
    downloadText("snapshot.yaml", "facts: []");
  } finally {
    HTMLAnchorElement.prototype.click = original;
  }

  expect(downloaded).toEqual(["snapshot.yaml"]);
});
