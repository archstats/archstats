import { readdirSync, readFileSync } from "node:fs";
import { dirname, join } from "node:path";

// The overview's stat strip, counted from the repository at build time so it
// cannot drift from what the pages list.
export interface Stats {
  release: string;
  tables: number;
  metrics: number;
  decisions: number;
}

declare const data: Stats;
export { data };

export default {
  load(): Stats {
    const docs: string = (globalThis as any).VITEPRESS_CONFIG.srcDir;
    const root = dirname(docs);
    const read = (p: string) => readFileSync(join(root, p), "utf8");

    const description = read("DESCRIPTION.md");
    const tables = description.split("<!-- #region tables -->")[1]?.split("<!-- #endregion tables -->")[0] ?? "";
    const release = read("CHANGELOG.md").match(/^## (v\d+\.\d+\.\d+)/m)?.[1] ?? "unreleased";

    return {
      release,
      tables: tables.split("\n").filter((l) => l.startsWith("| `")).length,
      metrics: read("docs/metrics.md").split("\n").filter((l) => l.startsWith("### `")).length,
      decisions: readdirSync(join(docs, "adr")).filter((f) => /^\d{4}-.*\.md$/.test(f)).length,
    };
  },
};
