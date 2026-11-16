<script setup lang="ts">
// One hairline-framed strip, label over value, as every overview in the app
// opens. The values are counted from the repository when the site is built.
import { withBase } from "vitepress";
import { data } from "../stats.data";

const cells = [
  { label: "Latest release", value: data.release, link: "/releases", mono: true },
  { label: "Snapshot tables", value: data.tables, link: "/snapshot" },
  { label: "Metrics", value: data.metrics, link: "/metrics" },
  { label: "Decisions recorded", value: data.decisions, link: "/adr/" },
];
</script>

<template>
  <dl class="stat-strip">
    <a v-for="c in cells" :key="c.label" class="stat-cell" :href="withBase(c.link)">
      <dt>{{ c.label }}</dt>
      <dd :class="{ mono: c.mono }">{{ c.value }}</dd>
    </a>
  </dl>
</template>
