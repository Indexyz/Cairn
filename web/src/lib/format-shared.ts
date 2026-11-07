export const LAYOUT_MODES = [
  "dagre_tb",
  "dagre_lr",
  "klay_tb",
  "klay_lr",
  "elk_tb",
  "elk_lr",
] as const;

export type LayoutMode = (typeof LAYOUT_MODES)[number];

export function isValidLayoutMode(mode: string): mode is LayoutMode {
  return (LAYOUT_MODES as readonly string[]).includes(mode);
}
