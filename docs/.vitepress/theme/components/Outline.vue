<script setup lang="ts">
import { onContentUpdated } from "vitepress";
import { computed, onMounted, onUnmounted, ref } from "vue";

// "On this page": the page's h2 and h3, read from the rendered page. Long
// reference pages (Metrics has seventy entries) show a section's h3 only
// while that section is the one being read.
type Heading = { id: string; text: string; level: number; mono: boolean; parent: string };

const headings = ref<Heading[]>([]);
const activeId = ref("");

function read() {
  const nodes = Array.from(document.querySelectorAll<HTMLElement>(".doc h2[id], .doc h3[id]"));
  let parent = "";
  headings.value = nodes.map((el) => {
    const level = el.tagName === "H2" ? 2 : 3;
    if (level === 2) parent = el.id;
    const clone = el.cloneNode(true) as HTMLElement;
    clone.querySelectorAll(".header-anchor").forEach((a) => a.remove());
    const code = clone.querySelector("code");
    const mono = !!code && clone.textContent?.trim().startsWith(code.textContent ?? "\u0000") === true;
    // A metric heading reads `id` (Name): the outline keeps the id.
    const text = mono ? code!.textContent ?? "" : clone.textContent?.trim() ?? "";
    return { id: el.id, text, level, mono, parent: level === 2 ? el.id : parent };
  });
  track();
}

function track() {
  const offset = 72;
  let id = "";
  for (const h of headings.value) {
    const el = document.getElementById(h.id);
    if (el && el.getBoundingClientRect().top - offset <= 0) id = h.id;
  }
  const atBottom = window.innerHeight + window.scrollY >= document.documentElement.scrollHeight - 2;
  if (atBottom && headings.value.length) id = headings.value[headings.value.length - 1].id;
  activeId.value = id;
}

const activeParent = computed(() => headings.value.find((h) => h.id === activeId.value)?.parent ?? "");
const manyH3 = computed(() => headings.value.filter((h) => h.level === 3).length > 24);
const shown = computed(() =>
  headings.value.filter((h) => h.level === 2 || !manyH3.value || h.parent === activeParent.value),
);

onContentUpdated(read);
onMounted(() => {
  read();
  window.addEventListener("scroll", track, { passive: true });
});
onUnmounted(() => window.removeEventListener("scroll", track));
</script>

<template>
  <aside v-if="shown.length > 1" class="outline" aria-label="On this page">
    <h2 class="overline">On this page</h2>
    <ul>
      <li v-for="h in shown" :key="h.id">
        <a
          :href="`#${h.id}`"
          class="outline-row"
          :class="{ active: h.id === activeId, sub: h.level === 3, mono: h.mono }"
          >{{ h.text }}</a
        >
      </li>
    </ul>
  </aside>
</template>
