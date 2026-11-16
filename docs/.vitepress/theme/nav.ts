import { computed } from "vue";
import { useData, useRoute, withBase } from "vitepress";

export type NavItem = { text: string; link: string; icon?: string };
export type NavFamily = { text: string; items: NavItem[] };

// A route path without the site base, ".html" or a trailing "index", so a
// sidebar link and the page it names compare equal.
export function normalize(path: string, base: string): string {
  let p = decodeURI(path).split("#")[0].split("?")[0];
  if (base !== "/" && p.startsWith(base)) p = "/" + p.slice(base.length);
  // A trailing slash is ignored on both sides: /desktop/ and /desktop are one
  // page, and the MkDocs site this replaced linked /install/.
  p = p.replace(/\.html$/, "").replace(/\/index$/, "/").replace(/\/+$/, "");
  return p || "/";
}

const key = (link: string) => link.replace(/\/+$/, "") || "/";

export function useNav() {
  const { theme, site, page } = useData();
  const route = useRoute();

  const families = computed<NavFamily[]>(() => theme.value.sidebar ?? []);
  const flat = computed(() => families.value.flatMap((f) => f.items.map((item) => ({ ...item, family: f.text }))));
  const current = computed(() => normalize(route.path, site.value.base));

  const active = computed(() => flat.value.find((i) => key(i.link) === current.value));
  // A page outside the rail (a decision record) belongs to the family of the
  // nearest rail page above it: /adr/0001-… sits under Decisions.
  const owner = computed(
    () =>
      active.value ??
      flat.value
        .filter((i) => i.link !== "/" && i.link.endsWith("/") && current.value.startsWith(i.link))
        .sort((a, b) => b.link.length - a.link.length)[0],
  );

  const index = computed(() => (active.value ? flat.value.indexOf(active.value) : -1));
  const prev = computed(() => (index.value > 0 ? flat.value[index.value - 1] : undefined));
  const next = computed(() => (index.value >= 0 && index.value < flat.value.length - 1 ? flat.value[index.value + 1] : undefined));

  const editUrl = computed(() => (page.value.filePath ? theme.value.editBase + page.value.filePath : undefined));
  const sourcePath = computed(() => (page.value.filePath ? "docs/" + page.value.filePath : ""));

  const isActive = (link: string) => key(link) === current.value;
  // The rail row to mark: the page itself, or the rail page that holds it.
  const isCurrent = (link: string) => owner.value?.link === link;
  const href = (link: string) => withBase(link);

  return { families, flat, current, active, owner, prev, next, editUrl, sourcePath, isActive, isCurrent, href };
}
