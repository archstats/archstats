<script setup lang="ts">
// The overview's two next steps: one primary (the app), one secondary (the tour).
import { withBase } from "vitepress";
import Icon from "./Icon.vue";
</script>

<template>
  <div class="home-actions">
    <a class="btn btn-primary" :href="withBase('/desktop/')"><Icon name="download" :size="14" />Get the desktop app</a>
    <a class="btn" :href="withBase('/tour')">Tour the app<Icon name="chevron-right" :size="14" /></a>
    <a class="btn-quiet" :href="withBase('/cli')"><Icon name="terminal" :size="14" />Use the CLI</a>
  </div>
</template>
