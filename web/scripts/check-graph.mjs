// Headless regression check: the project graph must render its nodes.
// Usage: start the server on :8130, then `npm run check:graph`
// On NixOS set CHROME=$(nix build --print-out-paths nixpkgs#chromium)/bin/chromium
import { chromium } from "playwright";

const base = process.env.CAIRN_BASE_URL ?? "http://127.0.0.1:8130";
const json = (body) => ({
  method: "POST",
  headers: { "content-type": "application/json" },
  body: JSON.stringify(body),
});

const created = await fetch(`${base}/projects`, json({ title: "graph check", origin: "o", goal: "g" })).then(
  (response) => response.json(),
);
const projectId = created.project.id;
await fetch(`${base}/projects/${projectId}/intents`, json({ from: ["origin"], description: "look", creator: "human" }));

const browser = await chromium.launch(process.env.CHROME ? { executablePath: process.env.CHROME } : {});
const page = await browser.newPage({ viewport: { width: 1400, height: 900 } });
const errors = [];
page.on("pageerror", (error) => errors.push(String(error)));

await page.goto(`${base}/#/projects/${projectId}`);
await page.waitForTimeout(2500);

const report = await page.evaluate(() => {
  const cy = window.__cy;
  const container = document.querySelector('div[class*="h-full"][class*="w-full"]');
  const rect = container?.getBoundingClientRect();
  return {
    hasCy: Boolean(cy),
    nodes: cy ? cy.nodes().length : -1,
    edges: cy ? cy.edges().length : -1,
    height: rect ? Math.round(rect.height) : 0,
    canvases: container?.querySelectorAll("canvas").length ?? 0,
  };
});

console.log(JSON.stringify(report));
if (errors.length > 0) console.log("errors:", errors.slice(0, 5));
await browser.close();

const problems = [];
if (!report.hasCy) problems.push("cytoscape instance missing");
if (report.nodes !== 3) problems.push(`expected 3 nodes, got ${report.nodes}`);
if (report.edges !== 1) problems.push(`expected 1 edge, got ${report.edges}`);
if (report.canvases < 1) problems.push("no canvas rendered");
if (report.height < 100) problems.push(`graph container height ${report.height}`);

if (problems.length > 0) {
  console.error("FAIL:", problems.join("; "));
  process.exit(1);
}
console.log("OK");
