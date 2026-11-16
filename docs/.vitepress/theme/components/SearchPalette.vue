<script setup lang="ts">
// The search palette: the app's ⌘P "Go to anything" popover over the site's
// local search index. Type to filter, arrows to move, Enter to open.
// @ts-ignore virtual module provided by VitePress's local search
import localSearchIndex from "@localSearchIndex";
import MiniSearch, { type SearchResult } from "minisearch";
import { useRouter } from "vitepress";
import { computed, nextTick, onMounted, ref, shallowRef, watch } from "vue";
import Icon from "./Icon.vue";

const emit = defineEmits<{ close: [] }>();
const router = useRouter();

type Doc = { title: string; titles: string[] };
type Hit = SearchResult & Doc;

const query = ref("");
const input = ref<HTMLInputElement>();
const list = ref<HTMLElement>();
const index = shallowRef<MiniSearch<Doc>>();
const active = ref(0);

onMounted(async () => {
  input.value?.focus();
  const data = await localSearchIndex.root();
  index.value = MiniSearch.loadJSON<Doc>(data.default, {
    fields: ["title", "titles", "text"],
    storeFields: ["title", "titles"],
    searchOptions: { fuzzy: 0.2, prefix: true, boost: { title: 4, text: 2, titles: 1 } },
  });
});

const hits = computed<Hit[]>(() => {
  const q = query.value.trim();
  if (!q || !index.value) return [];
  return index.value.search(q).slice(0, 40) as Hit[];
});

// The index stores titles as HTML; the palette shows them as text.
const decoder = typeof document !== "undefined" ? document.createElement("textarea") : null;
function text(html: string) {
  if (!decoder) return html;
  decoder.innerHTML = html;
  return decoder.value;
}

// Group by page, the way the app groups hits by kind; pages keep the order
// of their best hit.
const grouped = computed(() => {
  const pages = new Map<string, Hit[]>();
  for (const hit of hits.value) {
    const page = text(hit.titles[0] ?? hit.title);
    pages.set(page, [...(pages.get(page) ?? []), hit]);
  }
  return pages;
});

const ordered = computed(() => [...grouped.value.values()].flat());

const rows = computed(() => {
  const out: ({ heading: string } | { hit: Hit; i: number })[] = [];
  let i = 0;
  for (const [page, pageHits] of grouped.value) {
    out.push({ heading: page });
    for (const hit of pageHits) out.push({ hit, i: i++ });
  }
  return out;
});

watch(query, () => (active.value = 0));

function open(hit: Hit | undefined) {
  if (!hit) return;
  emit("close");
  router.go(hit.id);
}

function move(delta: number) {
  const n = ordered.value.length;
  if (!n) return;
  active.value = (active.value + delta + n) % n;
  nextTick(() => list.value?.querySelector(".palette-row.active")?.scrollIntoView({ block: "nearest" }));
}

function pathOf(hit: Hit) {
  return hit.titles.slice(1).map(text);
}
</script>

<template>
  <div class="palette-scrim" @click.self="emit('close')">
    <div class="palette" role="dialog" aria-modal="true" aria-label="Search the docs">
      <div class="palette-field">
        <Icon name="search" :size="14" />
        <input
          ref="input"
          v-model="query"
          type="search"
          placeholder="Search pages, tables, metrics, flags"
          aria-controls="palette-results"
          autocomplete="off"
          spellcheck="false"
          @keydown.down.prevent="move(1)"
          @keydown.up.prevent="move(-1)"
          @keydown.enter.prevent="open(ordered[active])"
          @keydown.esc.prevent="emit('close')"
        />
        <span v-if="query && index" class="palette-count">{{ hits.length }}</span>
      </div>

      <ul id="palette-results" ref="list" class="palette-results" role="listbox">
        <template v-for="(row, k) in rows" :key="k">
          <li v-if="'heading' in row" class="palette-heading" role="presentation">{{ row.heading }}</li>
          <li
            v-else
            class="palette-row"
            :class="{ active: row.i === active }"
            role="option"
            :aria-selected="row.i === active"
            @mousemove="active = row.i"
            @click="open(row.hit)"
          >
            <Icon :name="pathOf(row.hit).length ? 'hash' : 'file'" :size="13" />
            <span class="palette-title">{{ text(row.hit.title) }}</span>
            <span class="palette-path">{{ pathOf(row.hit).join(" › ") }}</span>
            <kbd v-if="row.i === active">↵</kbd>
          </li>
        </template>
        <li v-if="query && index && !hits.length" class="palette-empty" role="presentation">
          Nothing matches “{{ query }}”. Try a table name, a metric id, or a command.
        </li>
        <li v-if="!query" class="palette-empty" role="presentation">
          Pages, sections, snapshot tables, metric ids and flags are all searchable.
        </li>
      </ul>

      <div class="palette-foot">
        <span><kbd>↑↓</kbd> move</span>
        <span><kbd>↵</kbd> open</span>
        <span class="right"><kbd>Esc</kbd> close</span>
      </div>
    </div>
  </div>
</template>
