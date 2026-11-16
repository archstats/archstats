// Syntax colours for code blocks, as the desktop app's code viewer draws them:
// comments in Ink Secondary italic, keywords violet, strings green, numbers
// amber, functions blue, types a deeper blue. Every colour is a step of the
// app's data ramps (archstats-ui frontend/design/tokens.cjs, step 700 and 800),
// light and dark, so both appearances hold without a third-party theme.

type Palette = {
  fg: string; comment: string; keyword: string; string: string; number: string;
  fn: string; type: string; attr: string; meta: string; deleted: string; punct: string;
};

const light: Palette = {
  fg: "#262932", comment: "#6C7280", keyword: "#4E3898", string: "#27663E", number: "#8F5B12",
  fn: "#234AA8", type: "#1D3C85", attr: "#262932", meta: "#6F4711", deleted: "#9B2F2F", punct: "#5C6270",
};

const dark: Palette = {
  fg: "#D5D9E0", comment: "#8B919E", keyword: "#C3B6F1", string: "#99D4AE", number: "#F1C77F",
  fn: "#9EBDF6", type: "#BDD2F9", attr: "#E3E6EB", meta: "#F6D8A5", deleted: "#F4A2A2", punct: "#9AA0AC",
};

function theme(name: string, type: "light" | "dark", p: Palette) {
  return {
    name,
    type,
    colors: { "editor.foreground": p.fg, "editor.background": "#00000000" },
    tokenColors: [
      { settings: { foreground: p.fg } },
      { scope: ["comment", "punctuation.definition.comment"], settings: { foreground: p.comment, fontStyle: "italic" } },
      { scope: ["keyword", "storage", "keyword.other.DML", "keyword.other.DDL", "keyword.control", "constant.language", "support.type.primitive"], settings: { foreground: p.keyword } },
      { scope: ["keyword.operator", "punctuation", "meta.brace"], settings: { foreground: p.punct } },
      { scope: ["string", "string.quoted", "markup.inline.raw"], settings: { foreground: p.string } },
      { scope: ["constant.numeric", "constant.other", "variable.other.constant"], settings: { foreground: p.number } },
      { scope: ["entity.name.function", "support.function", "entity.name.command", "support.function.builtin"], settings: { foreground: p.fn } },
      { scope: ["entity.name.type", "support.type", "support.class", "entity.name.class", "entity.name.tag", "storage.type.class"], settings: { foreground: p.type } },
      { scope: ["entity.other.attribute-name", "variable.parameter", "support.type.property-name", "meta.object-literal.key"], settings: { foreground: p.attr } },
      { scope: ["meta.preprocessor", "meta.directive", "keyword.other.unit", "variable.other.normal.shell", "variable.other.special.shell"], settings: { foreground: p.meta } },
      { scope: ["markup.deleted", "invalid"], settings: { foreground: p.deleted } },
      { scope: ["markup.inserted"], settings: { foreground: p.string } },
      { scope: ["markup.heading", "markup.bold"], settings: { fontStyle: "bold" } },
    ],
  };
}

export const codeThemes = {
  light: theme("archstats-light", "light", light),
  dark: theme("archstats-dark", "dark", dark),
} as any;
