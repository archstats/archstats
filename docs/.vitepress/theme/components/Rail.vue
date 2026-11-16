<script setup lang="ts">
import { withBase } from "vitepress";
import { onMounted, ref } from "vue";
import Icon from "./Icon.vue";
import { useNav } from "../nav";

defineEmits<{ search: [] }>();
const { families, isActive, isCurrent, href } = useNav();
// Read after hydration, so the server's render and the client's agree.
const isMac = ref(true);
onMounted(() => (isMac.value = /Mac|iPhone|iPad/.test(navigator.platform)));
</script>

<template>
  <nav class="rail" aria-label="Documentation">
    <a class="rail-brand" :href="withBase('/')" aria-label="Archstats documentation">
      <img class="mark light-only" :src="withBase('/img/archstats-mark.png')" alt="" width="20" height="20" />
      <img class="mark dark-only" :src="withBase('/img/archstats-mark-dark.png')" alt="" width="20" height="20" />
      <img class="wordmark light-only" :src="withBase('/img/archstats-wordmark.png')" alt="Archstats" height="13" />
      <img class="wordmark dark-only" :src="withBase('/img/archstats-wordmark-dark.png')" alt="Archstats" height="13" />
      <span class="rail-brand-meta">docs</span>
    </a>

    <button class="rail-search" type="button" @click="$emit('search')">
      <Icon name="search" :size="13" />
      <span>Search the docs</span>
      <kbd>{{ isMac ? "⌘K" : "Ctrl K" }}</kbd>
    </button>

    <div class="rail-families">
      <section v-for="family in families" :key="family.text" class="rail-family">
        <h2 class="overline">{{ family.text }}</h2>
        <ul>
          <li v-for="item in family.items" :key="item.link">
            <a class="nav-row" :class="{ active: isCurrent(item.link) }" :href="href(item.link)" :aria-current="isActive(item.link) ? 'page' : undefined">
              <Icon :name="item.icon ?? 'file'" />
              <span>{{ item.text }}</span>
            </a>
          </li>
        </ul>
      </section>
    </div>

    <footer class="rail-footer">
      <a class="nav-row" href="https://github.com/archstats/archstats" target="_blank" rel="noopener">
        <Icon name="github" />
        <span>Engine and CLI</span>
        <Icon name="external" :size="12" class="trail" />
      </a>
      <a class="nav-row" href="https://github.com/archstats/archstats-ui" target="_blank" rel="noopener">
        <Icon name="github" />
        <span>Desktop app</span>
        <Icon name="external" :size="12" class="trail" />
      </a>
    </footer>
  </nav>
</template>
