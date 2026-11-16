<script setup lang="ts">
// The three ways to read a snapshot, as the app's "Views" directory lists
// views: a hairline panel of rows, each a 64×40 thumbnail, a name, a line of
// description and a chevron. The thumbnails are schematics drawn in the
// site's own tokens, not screenshots.
import { withBase } from "vitepress";
import Icon from "./Icon.vue";

const ways = [
  {
    kind: "desktop",
    name: "Archstats Desktop",
    meta: "macOS · Windows · Linux",
    text: "Add a folder or clone a repository; it scans and shows the result as pictures and tables.",
    link: "/desktop/",
  },
  {
    kind: "cli",
    name: "The archstats CLI",
    meta: "scripts and CI",
    text: "Writes snapshots, prints any table in the terminal, and checks rules you write in SQL.",
    link: "/cli",
  },
  {
    kind: "sql",
    name: "Your own SQL",
    meta: "any SQLite client",
    text: "A snapshot is a plain SQLite file. Every table and column is documented.",
    link: "/snapshot",
  },
];
</script>

<template>
  <ul class="ways">
    <li v-for="w in ways" :key="w.kind">
      <a class="way" :href="withBase(w.link)">
        <svg class="way-thumb" viewBox="0 0 64 40" aria-hidden="true">
          <template v-if="w.kind === 'desktop'">
            <rect x="0" y="0" width="16" height="40" fill="var(--ground)" />
            <path d="M16 0v40" stroke="var(--hairline)" />
            <rect x="3" y="5" width="9" height="2" rx="1" fill="var(--hairline-strong)" />
            <rect x="3" y="11" width="10" height="3" rx="1" fill="var(--accent-tint)" />
            <rect x="3" y="11" width="1" height="3" fill="var(--accent)" />
            <rect x="3" y="17" width="8" height="2" rx="1" fill="var(--hairline-strong)" />
            <rect x="3" y="22" width="9" height="2" rx="1" fill="var(--hairline-strong)" />
            <path d="M16 7h48" stroke="var(--hairline)" />
            <circle cx="35" cy="24" r="9" fill="none" stroke="var(--hairline-strong)" />
            <circle cx="32" cy="22" r="3.5" fill="var(--surface-hover)" stroke="var(--hairline-strong)" />
            <circle cx="39" cy="27" r="2.5" fill="var(--surface-hover)" stroke="var(--hairline-strong)" />
            <circle cx="53" cy="20" r="5" fill="none" stroke="var(--hairline-strong)" />
            <circle cx="54" cy="31" r="3" fill="none" stroke="var(--hairline-strong)" />
          </template>
          <template v-else-if="w.kind === 'cli'">
            <path d="M6 9l4 3-4 3" fill="none" stroke="var(--ink-muted)" stroke-width="1.2" stroke-linecap="round" stroke-linejoin="round" />
            <rect x="13" y="11" width="30" height="2" rx="1" fill="var(--ink-muted)" />
            <rect x="6" y="19" width="14" height="2" rx="1" fill="var(--hairline-strong)" />
            <rect x="23" y="19" width="10" height="2" rx="1" fill="var(--hairline-strong)" />
            <rect x="36" y="19" width="12" height="2" rx="1" fill="var(--hairline-strong)" />
            <rect x="6" y="24" width="14" height="2" rx="1" fill="var(--hairline-strong)" />
            <rect x="23" y="24" width="8" height="2" rx="1" fill="var(--hairline-strong)" />
            <rect x="36" y="24" width="10" height="2" rx="1" fill="var(--hairline-strong)" />
            <path d="M6 31l4 3-4 3" fill="none" stroke="var(--ink-muted)" stroke-width="1.2" stroke-linecap="round" stroke-linejoin="round" />
            <rect x="13" y="32" width="1.5" height="5" fill="var(--accent)" />
          </template>
          <template v-else>
            <rect x="0" y="0" width="64" height="9" fill="var(--ground)" />
            <path d="M0 9h64M0 17h64M0 25h64M0 33h64" stroke="var(--hairline)" />
            <path d="M26 0v40M45 0v40" stroke="var(--hairline)" />
            <rect x="4" y="3.5" width="14" height="2" rx="1" fill="var(--ink-muted)" />
            <rect x="30" y="3.5" width="10" height="2" rx="1" fill="var(--ink-muted)" />
            <rect x="49" y="3.5" width="10" height="2" rx="1" fill="var(--ink-muted)" />
            <rect x="4" y="12" width="18" height="2" rx="1" fill="var(--hairline-strong)" />
            <rect x="4" y="20" width="14" height="2" rx="1" fill="var(--hairline-strong)" />
            <rect x="4" y="28" width="16" height="2" rx="1" fill="var(--hairline-strong)" />
            <rect x="35" y="12" width="6" height="2" rx="1" fill="var(--hairline-strong)" />
            <rect x="33" y="20" width="8" height="2" rx="1" fill="var(--hairline-strong)" />
            <rect x="36" y="28" width="5" height="2" rx="1" fill="var(--hairline-strong)" />
            <rect x="53" y="12" width="7" height="2" rx="1" fill="var(--hairline-strong)" />
            <rect x="55" y="20" width="5" height="2" rx="1" fill="var(--hairline-strong)" />
            <rect x="52" y="28" width="8" height="2" rx="1" fill="var(--hairline-strong)" />
          </template>
        </svg>
        <span class="way-body">
          <span class="way-name">{{ w.name }}<span class="way-meta">{{ w.meta }}</span></span>
          <span class="way-text">{{ w.text }}</span>
        </span>
        <Icon name="chevron-right" :size="14" class="way-chevron" />
      </a>
    </li>
  </ul>
</template>
