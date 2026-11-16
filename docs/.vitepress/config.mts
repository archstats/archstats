import { dirname, relative, resolve, sep } from "node:path";
import { fileURLToPath } from "node:url";
import { defineConfig } from "vitepress";
import { codeThemes } from "./code-themes";

const root = resolve(dirname(fileURLToPath(import.meta.url)), "../..");

// The documentation site, published to https://archstats.github.io/archstats/
// by .github/workflows/docs.yml. The theme in ./theme is the desktop app's own
// design system (archstats-ui DESIGN.md): its tokens, Inter and JetBrains Mono,
// hairlines, and one orange. Build it locally from docs/:
//   npm ci && npm run dev

const repo = "https://github.com/archstats/archstats";

export default defineConfig({
  title: "Archstats",
  description: "Reads a codebase and writes its architecture, history and pipelines to one SQLite snapshot.",
  base: "/archstats/",
  lang: "en",
  cleanUrls: true,
  lastUpdated: false,
  // The ADR folder keeps its template beside the records; it is not a page.
  srcExclude: ["adr/template.md", "**/node_modules/**"],
  // README.md in a folder is that folder's page, as on GitHub.
  rewrites: { "adr/README.md": "adr/index.md" },

  head: [
    ["link", { rel: "icon", type: "image/png", href: "/archstats/img/archstats-mark.png" }],
    ["link", { rel: "preload", href: "/archstats/fonts/InterVariable.woff2", as: "font", type: "font/woff2", crossorigin: "" }],
    ["meta", { name: "theme-color", content: "#F7F8FA", media: "(prefers-color-scheme: light)" }],
    ["meta", { name: "theme-color", content: "#262932", media: "(prefers-color-scheme: dark)" }],
  ],

  markdown: {
    theme: codeThemes,
    // Tables get a frame that scrolls sideways on its own, so a wide
    // reference table never pushes the page wider than the window.
    config(md) {
      md.renderer.rules.table_open = () => '<div class="table-frame"><table>\n';
      md.renderer.rules.table_close = () => "</table></div>\n";
      // A link to a file that is not a page (a rules file, a source folder)
      // opens it on GitHub, where it lives.
      md.core.ruler.push("repo_links", (state) => {
        const from = state.env?.path as string | undefined;
        if (!from) return;
        for (const block of state.tokens) {
          for (const token of block.children ?? []) {
            if (token.type !== "link_open") continue;
            const href = token.attrGet("href");
            if (!href || /^([a-z]+:|#|\/)/i.test(href)) continue;
            const [target, hash] = href.split("#");
            if (/\.md$/.test(target)) continue;
            const file = relative(root, resolve(dirname(from), target)).split(sep).join("/");
            if (file.startsWith("..")) continue;
            token.attrSet("href", `${repo}/blob/main/${file}${hash ? "#" + hash : ""}`);
          }
        }
      });
    },
  },

  themeConfig: {
    repo,
    editBase: `${repo}/edit/main/docs/`,
    search: { provider: "local" },
    // Families, as the app's rail groups its views.
    sidebar: [
      {
        text: "Start",
        items: [
          { text: "Overview", link: "/", icon: "home" },
          { text: "What you can do", link: "/use-cases", icon: "grid" },
          { text: "Install", link: "/install", icon: "download" },
        ],
      },
      {
        text: "Desktop app",
        items: [
          { text: "Getting started", link: "/desktop/", icon: "app" },
          { text: "Updates and releases", link: "/desktop/updates", icon: "refresh" },
        ],
      },
      {
        text: "CLI",
        items: [
          { text: "Commands", link: "/cli", icon: "terminal" },
          { text: "Rules", link: "/rules", icon: "check" },
        ],
      },
      {
        text: "Reference",
        items: [
          { text: "Snapshot tables", link: "/snapshot", icon: "table" },
          { text: "Metrics", link: "/metrics", icon: "gauge" },
          { text: "Ecosystems", link: "/ecosystems", icon: "layers" },
          { text: "Decisions", link: "/adr/", icon: "file" },
        ],
      },
      {
        text: "Project",
        items: [{ text: "Releases", link: "/releases", icon: "tag" }],
      },
    ],
  },
});
