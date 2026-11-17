<script setup lang="ts">
// The site is laid out like the desktop app's tool window: a rail of families
// on the ground tone, a 40px toolbar naming where you are, the page on the
// surface, and the page's outline on the right.
import { Content, useData, useRoute, useRouter, withBase } from "vitepress";
import { computed, onMounted, onUnmounted, ref, watch } from "vue";
import Icon from "./components/Icon.vue";
import Outline from "./components/Outline.vue";
import Rail from "./components/Rail.vue";
import SearchPalette from "./components/SearchPalette.vue";
import { useNav } from "./nav";

const { page, frontmatter, isDark, site, theme } = useData();
const route = useRoute();
const router = useRouter();
const { owner, active, prev, next, editUrl, sourcePath, href } = useNav();

const railOpen = ref(false);
const searchOpen = ref(false);
const isHome = computed(() => frontmatter.value.layout === "home");

watch(
  () => route.path,
  () => {
    railOpen.value = false;
    searchOpen.value = false;
  },
);

function onKey(e: KeyboardEvent) {
  const typing = e.target instanceof HTMLElement && /^(INPUT|TEXTAREA|SELECT)$/.test(e.target.tagName);
  if ((e.key === "k" || e.key === "K") && (e.metaKey || e.ctrlKey)) {
    e.preventDefault();
    searchOpen.value = !searchOpen.value;
  } else if (e.key === "/" && !typing && !searchOpen.value) {
    e.preventDefault();
    searchOpen.value = true;
  } else if (e.key === "Escape" && railOpen.value) {
    railOpen.value = false;
  }
}

// Code blocks carry an empty copy button; this fills its behaviour in.
function onClick(e: MouseEvent) {
  const button = (e.target as HTMLElement).closest<HTMLButtonElement>('div[class*="language-"] > button.copy');
  if (!button) return;
  const code = button.parentElement?.querySelector("pre code");
  if (!code) return;
  navigator.clipboard.writeText(code.textContent ?? "").then(() => {
    button.classList.add("copied");
    setTimeout(() => button.classList.remove("copied"), 1600);
  });
}

onMounted(() => {
  window.addEventListener("keydown", onKey);
  document.addEventListener("click", onClick);
  if (page.value.isNotFound) {
    // A page that moved: its old address goes to where its text now lives.
    const path = window.location.pathname;
    const key = "/" + path.slice(site.value.base.length).replace(/\.html$/, "").replace(/\/+$/, "");
    const moved = (theme.value.moved ?? {})[key];
    if (moved) router.go(withBase(moved));
    // The MkDocs site this replaced served /cli/; this one serves /cli.
    else if (path.length > site.value.base.length && path.endsWith("/")) router.go(path.slice(0, -1) + window.location.hash);
  }
});
onUnmounted(() => {
  window.removeEventListener("keydown", onKey);
  document.removeEventListener("click", onClick);
});

const crumbFamily = computed(() => owner.value?.family);
const crumbParent = computed(() => (owner.value && owner.value !== active.value ? owner.value : undefined));
</script>

<template>
  <div class="shell" :class="{ 'rail-open': railOpen }">
    <Rail @search="searchOpen = true" />
    <div class="rail-scrim" @click="railOpen = false" />

    <div class="main">
      <header class="toolbar">
        <button class="btn-icon rail-toggle" type="button" aria-label="Show the contents" @click="railOpen = !railOpen">
          <Icon name="panel" />
        </button>

        <nav class="crumbs" aria-label="Breadcrumb">
          <template v-if="page.isNotFound">
            <span class="crumb-current">Not found</span>
          </template>
          <template v-else>
            <span v-if="crumbFamily" class="crumb">{{ crumbFamily }}</span>
            <Icon v-if="crumbFamily" name="chevron-right" :size="12" class="crumb-sep" />
            <template v-if="crumbParent">
              <a class="crumb" :href="href(crumbParent.link)">{{ crumbParent.text }}</a>
              <Icon name="chevron-right" :size="12" class="crumb-sep" />
            </template>
            <span class="crumb-current">{{ active?.text ?? page.title }}</span>
          </template>
        </nav>

        <div class="toolbar-end">
          <button class="toolbar-search" type="button" aria-label="Search the docs" @click="searchOpen = true">
            <Icon name="search" :size="13" />
          </button>
          <a v-if="editUrl && !page.isNotFound" class="btn-quiet edit" :href="editUrl" target="_blank" rel="noopener" :title="`Edit ${sourcePath} on GitHub`">
            <Icon name="pencil" :size="13" />
            <span class="mono">{{ sourcePath }}</span>
          </a>
          <span class="toolbar-sep" aria-hidden="true" />
          <button class="btn-icon" type="button" :aria-label="isDark ? 'Use the light appearance' : 'Use the dark appearance'" @click="isDark = !isDark">
            <Icon :name="isDark ? 'sun' : 'moon'" />
          </button>
        </div>
      </header>

      <div class="page" :class="{ home: isHome }">
        <main class="doc-column">
          <article v-if="page.isNotFound" class="doc">
            <div class="empty-state">
              <Icon name="file-x" :size="20" />
              <p class="empty-title">There is no page here</p>
              <p class="empty-reason">The address may be from an older version of these docs. Search finds every page and section.</p>
              <div class="empty-actions">
                <a class="btn" :href="withBase('/')">Go to the overview</a>
                <button class="btn" type="button" @click="searchOpen = true">Search the docs</button>
              </div>
            </div>
          </article>
          <article v-else class="doc">
            <Content />
          </article>

          <nav v-if="!page.isNotFound && (prev || next)" class="pager" aria-label="Previous and next page">
            <a v-if="prev" class="pager-link" :href="href(prev.link)">
              <span class="overline">Previous</span>
              <span class="pager-title"><Icon name="chevron-left" :size="13" />{{ prev.text }}</span>
            </a>
            <span v-else />
            <a v-if="next" class="pager-link next" :href="href(next.link)">
              <span class="overline">Next</span>
              <span class="pager-title">{{ next.text }}<Icon name="chevron-right" :size="13" /></span>
            </a>
          </nav>

          <footer class="page-foot">
            <span>Apache-2.0</span>
            <span aria-hidden="true">·</span>
            <a href="https://github.com/archstats/archstats" target="_blank" rel="noopener">archstats/archstats</a>
            <span aria-hidden="true">·</span>
            <a href="https://github.com/archstats/archstats-ui" target="_blank" rel="noopener">archstats/archstats-ui</a>
          </footer>
        </main>

        <Outline v-if="!isHome && !page.isNotFound" :key="route.path" />
      </div>
    </div>

    <SearchPalette v-if="searchOpen" @close="searchOpen = false" />
  </div>
</template>
