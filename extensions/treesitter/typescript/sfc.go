package typescript

import (
	"path"
	"regexp"
	"strings"

	"github.com/archstats/archstats/core/file"
	"github.com/archstats/archstats/core/unit"
	"github.com/archstats/archstats/extensions/treesitter/common"
	"github.com/archstats/archstats/extensions/treesitter/javascript"
	"github.com/gobwas/glob"
)

// Single-file components: Vue's .vue and Svelte's .svelte.
//
// A component's code is TypeScript or JavaScript in a `<script>` block
// inside markup, and no pack read it: archstats-ui is a Nuxt app whose 217
// .vue files had no component and no unit edges, so the whole UI layer was
// invisible to every structural view.
//
// The script blocks are read by the ordinary packs over a copy of the file
// with everything outside them blanked to spaces. Every byte keeps its
// offset and every newline stays, so a snippet's position is its position in
// the real file with no arithmetic, and a block's line numbers are the ones
// an editor shows.
type sfcAnalyzer struct {
	glob        glob.Glob
	ts, tsx, js *common.LanguagePack
}

func newSFCAnalyzer() *sfcAnalyzer {
	return &sfcAnalyzer{
		glob: glob.MustCompile("**/*.{vue,svelte}"),
		ts:   createTypeScriptLanguagePack(false),
		tsx:  createTypeScriptLanguagePack(true),
		js:   javascript.LanguagePack(),
	}
}

func (a *sfcAnalyzer) AnalyzeFile(f file.File) *file.Results {
	return a.analyze(f.Path(), f.Content())
}

func (a *sfcAnalyzer) analyze(filePath string, content []byte) *file.Results {
	if !a.glob.Match(filePath) {
		return nil
	}
	blocks := scriptBlocks(content)
	res := a.packFor(blocks).AnalyzeContent(filePath, maskOutside(content, blocks))
	if res == nil {
		return nil
	}
	common.KeepReactOnlyWhereUsed(filePath, res)
	res.Units = sfcUnits(filePath, content, res, blocks)
	return res
}

// packFor is the grammar the script blocks are written in. `lang="ts"` is
// TypeScript and anything else JavaScript; a component whose two blocks
// disagree is read as the stricter, since TypeScript's grammar parses the
// JavaScript one's and not the reverse.
func (a *sfcAnalyzer) packFor(blocks []scriptBlock) *common.LanguagePack {
	pack := a.js
	for _, b := range blocks {
		switch b.lang {
		case "tsx":
			return a.tsx
		case "ts", "typescript":
			pack = a.ts
		}
	}
	return pack
}

// A scriptBlock is the body of one `<script>` element, by byte offset.
type scriptBlock struct {
	begin, end int
	lang       string
}

var (
	scriptOpen  = regexp.MustCompile(`(?i)<script(\s[^>]*)?>`)
	scriptClose = regexp.MustCompile(`(?i)</script\s*>`)
	langAttr    = regexp.MustCompile(`(?i)\blang\s*=\s*["']?([a-z]+)`)
	htmlComment = regexp.MustCompile(`(?s)<!--.*?-->`)
)

// scriptBlocks finds the `<script>` and `<script setup>` elements of a
// component. Vue allows one of each and Svelte a module and an instance
// script; both are read. A `<script>` inside an HTML comment is not code, so
// comments are blanked before looking.
func scriptBlocks(content []byte) []scriptBlock {
	text := []byte(string(content))
	for _, loc := range htmlComment.FindAllIndex(text, -1) {
		for i := loc[0]; i < loc[1]; i++ {
			if text[i] != '\n' {
				text[i] = ' '
			}
		}
	}
	var blocks []scriptBlock
	for from := 0; from < len(text); {
		open := scriptOpen.FindSubmatchIndex(text[from:])
		if open == nil {
			break
		}
		bodyBegin := from + open[1]
		lang := ""
		if open[2] >= 0 {
			attrs := text[from+open[2] : from+open[3]]
			if m := langAttr.FindSubmatch(attrs); m != nil {
				lang = strings.ToLower(string(m[1]))
			}
		}
		closing := scriptClose.FindIndex(text[bodyBegin:])
		if closing == nil {
			// Unterminated: the rest of the file is the script, which is
			// what a browser would make of it too.
			blocks = append(blocks, scriptBlock{begin: bodyBegin, end: len(text), lang: lang})
			break
		}
		bodyEnd := bodyBegin + closing[0]
		blocks = append(blocks, scriptBlock{begin: bodyBegin, end: bodyEnd, lang: lang})
		from = bodyBegin + closing[1]
	}
	return blocks
}

// maskOutside is the file with everything but the script blocks turned to
// spaces, newlines kept, so the parser sees only the code at the offsets it
// has in the real file.
func maskOutside(content []byte, blocks []scriptBlock) []byte {
	out := make([]byte, len(content))
	for i, c := range content {
		if c == '\n' || c == '\r' {
			out[i] = c
		} else {
			out[i] = ' '
		}
	}
	for _, b := range blocks {
		copy(out[b.begin:b.end], content[b.begin:b.end])
	}
	return out
}

// sfcUnits are the component and what its script declares.
//
// The component is a unit of its own, named for its file, because that is
// the one thing a .vue file exports and what `import ChatPanel from
// "./ChatPanel.vue"` takes. It makes the references nothing else in the file
// does: `<script setup>` is the component's body, not a module's top level,
// so what the module unit would have held is the component's. Functions
// declared in the script belong to it.
//
// Usages are looked for in the whole file, markup included, so a component
// used only as `<ChatPanel />` in the template counts as used. Vue also lets
// a template write it `<chat-panel>`, which no textual search for the name
// finds; those tags are matched to imported names separately.
func sfcUnits(filePath string, content []byte, res *file.Results, blocks []scriptBlock) []*unit.Unit {
	module := common.ModuleOf(filePath)
	name := componentName(filePath)
	component := &unit.Unit{
		ID:      module + "#" + name,
		Kind:    unit.KindType,
		Name:    name,
		Files:   []string{filePath},
		Markers: []unit.Marker{{Source: unit.SourceFilename, Key: frameworkOf(filePath) + "_component"}},
	}

	out := []*unit.Unit{component}
	for _, u := range common.JSUnitsFrom(filePath, content, res) {
		switch {
		case u.Kind == unit.KindModule && u.ID == module+"#":
			component.Refs = appendRefs(component.Refs, u.Refs...)
			continue
		case u.ID == component.ID:
			// A script declaring a function named like its own file is
			// declaring the component's own logic; one unit is enough.
			component.Refs = appendRefs(component.Refs, u.Refs...)
			continue
		}
		// Everything the script declares is the component's: its functions,
		// but also its `interface Props` and its local types, which stood
		// beside it as models of the codebase -- 80 of archstats-ui's 459
		// interfaces were a component's own props.
		if u.Owner == "" {
			u.Owner = component.ID
		}
		out = append(out, u)
	}

	bindings := common.ImportBindings(res)
	for _, tag := range kebabTags(content, blocks) {
		if source, ok := bindings[pascalCase(tag)]; ok {
			component.Refs = appendRefs(component.Refs, common.ImportedRef(source, pascalCase(tag)))
		}
	}
	return out
}

// componentName is what the component is called: its file's name, up to the
// first dot. Vue registers `components/ChatPanel.vue` as ChatPanel, and Nuxt
// `ChatPanel.client.vue` as ChatPanel too.
func componentName(filePath string) string {
	return common.DefaultExportName(filePath)
}

func frameworkOf(filePath string) string {
	return strings.TrimPrefix(strings.ToLower(path.Ext(filePath)), ".")
}

var kebabTag = regexp.MustCompile(`<([a-z][a-z0-9]*(?:-[a-z0-9]+)+)[\s/>]`)

// kebabTags are the hyphenated element names the markup opens, outside the
// script blocks: `<chat-panel>` is how Vue's in-DOM templates, and many of
// its users, write ChatPanel.
func kebabTags(content []byte, blocks []scriptBlock) []string {
	markup := maskInside(content, blocks)
	seen := map[string]bool{}
	var out []string
	for _, m := range kebabTag.FindAllSubmatch(markup, -1) {
		tag := string(m[1])
		if !seen[tag] {
			seen[tag] = true
			out = append(out, tag)
		}
	}
	return out
}

// maskInside is the file with the script blocks blanked: the markup alone.
func maskInside(content []byte, blocks []scriptBlock) []byte {
	out := append([]byte(nil), content...)
	for _, b := range blocks {
		for i := b.begin; i < b.end; i++ {
			out[i] = ' '
		}
	}
	return out
}

func pascalCase(kebab string) string {
	var sb strings.Builder
	for _, part := range strings.Split(kebab, "-") {
		if part == "" {
			continue
		}
		sb.WriteString(strings.ToUpper(part[:1]) + part[1:])
	}
	return sb.String()
}

func appendRefs(list []unit.Ref, refs ...unit.Ref) []unit.Ref {
next:
	for _, r := range refs {
		for _, existing := range list {
			if existing == r {
				continue next
			}
		}
		list = append(list, r)
	}
	return list
}
