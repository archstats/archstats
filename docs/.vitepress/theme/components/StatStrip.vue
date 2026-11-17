<script setup lang="ts">
// One hairline-framed strip, label over value, as every overview in the app
// opens. The values are counted from the repository when the site is built.
// A <dl> may hold <div> groups but not links, so each cell is a <div> whose
// value link is stretched over the whole cell.
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
    <div v-for="c in cells" :key="c.label" class="stat-cell">
      <dt>{{ c.label }}</dt>
      <dd :class="{ mono: c.mono }">
        <a class="stat-link" :href="withBase(c.link)">{{ c.value }}</a>
      </dd>
    </div>
  </dl>
</template>
