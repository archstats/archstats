<script setup lang="ts">
// A screenshot of the desktop app as an exhibit: a hairline frame, the light
// or dark capture to match the page, and a caption that names the view and
// the codebase it shows. Click to see it full size.
//
// The captures live in docs/public/shots as <name>.webp and <name>-dark.webp,
// taken from the app on public codebases (see docs/.vitepress/shots/README.md).
import { withBase } from "vitepress";
import { onUnmounted, ref, watch } from "vue";
import Icon from "./Icon.vue";

const props = defineProps<{
  name: string;
  title: string;
  source: string;
  wide?: boolean;
  /** Load at once: the first picture on a page. */
  eager?: boolean;
  width?: number;
  height?: number;
}>();

const w = props.width ?? (props.name.startsWith("overview") ? 1760 : 1600);
const h = props.height ?? (props.name.startsWith("overview") ? 1100 : 1200);
const open = ref(false);

function onKey(e: KeyboardEvent) {
  if (e.key === "Escape") open.value = false;
}
watch(open, (v) => (v ? window.addEventListener("keydown", onKey) : window.removeEventListener("keydown", onKey)));
onUnmounted(() => window.removeEventListener("keydown", onKey));
</script>

<template>
  <figure class="shot" :class="{ wide }">
    <button class="shot-frame" type="button" :aria-label="`Show ${title} full size`" @click="open = true">
      <img class="light-only" :src="withBase(`/shots/${name}.webp`)" :width="w" :height="h" :alt="title" :loading="eager ? 'eager' : 'lazy'" decoding="async" />
      <img class="dark-only" :src="withBase(`/shots/${name}-dark.webp`)" :width="w" :height="h" :alt="title" :loading="eager ? 'eager' : 'lazy'" decoding="async" />
    </button>
    <figcaption>
      <span class="shot-head">
        <span class="shot-title">{{ title }}</span>
        <span class="shot-source">{{ source }}</span>
      </span>
      <span class="shot-text"><slot /></span>
    </figcaption>

    <div v-if="open" class="shot-zoom" role="dialog" aria-modal="true" :aria-label="title" @click="open = false">
      <img class="light-only" :src="withBase(`/shots/${name}.webp`)" :alt="title" />
      <img class="dark-only" :src="withBase(`/shots/${name}-dark.webp`)" :alt="title" />
      <button class="btn-icon shot-close" type="button" aria-label="Close" @click.stop="open = false">
        <Icon name="x" />
      </button>
    </div>
  </figure>
</template>
