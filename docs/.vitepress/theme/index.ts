import type { Theme } from "vitepress";
import Layout from "./Layout.vue";
import HomeActions from "./components/HomeActions.vue";
import Shot from "./components/Shot.vue";
import StatStrip from "./components/StatStrip.vue";
import WaysIn from "./components/WaysIn.vue";
import "./tokens.css";
import "./style.css";

export default {
  Layout,
  enhanceApp({ app }) {
    app.component("HomeActions", HomeActions);
    app.component("Shot", Shot);
    app.component("StatStrip", StatStrip);
    app.component("WaysIn", WaysIn);
  },
} satisfies Theme;
