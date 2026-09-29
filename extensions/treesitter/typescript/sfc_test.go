package typescript

import (
	"strings"
	"testing"

	"github.com/archstats/archstats/core/file"
	"github.com/archstats/archstats/core/unit"
	"github.com/archstats/archstats/extensions/treesitter/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A Nuxt component, the shape of every file in archstats-ui's UI layer.
const chatWindowVue = `<template>
  <div class="chat">
    <!-- <script>import Old from "./Old.vue"</script> -->
    <ChatPanel :messages="messages" @send="onSend" />
    <user-avatar :user="me" />
  </div>
</template>

<script setup lang="ts">
import ChatPanel from '~/components/chat/ChatPanel.vue'
import UserAvatar from '@/components/UserAvatar.vue'
import { formatDate } from '~/utils/dates'
import type { Message } from '../types'

const messages = ref<Message[]>([])

function onSend(text: string) {
  messages.value.push({ text, at: formatDate(new Date()) })
}
</script>

<style scoped>
.chat { display: flex; }
</style>
`

func analyzeSFC(t *testing.T, path, src string) *file.Results {
	t.Helper()
	res := newSFCAnalyzer().analyze(path, []byte(src))
	require.NotNil(t, res)
	return res
}

func snippetsOfType(res *file.Results, typ string) []*file.Snippet {
	var out []*file.Snippet
	for _, s := range res.Snippets {
		if s.Type == typ {
			out = append(out, s)
		}
	}
	return out
}

func unitsByID(units []*unit.Unit) map[string]*unit.Unit {
	out := map[string]*unit.Unit{}
	for _, u := range units {
		out[u.ID] = u
	}
	return out
}

// The 217 .vue files of archstats-ui had no imports at all: nothing read
// their script blocks.
func TestAVueComponentsScriptSetupImportsAreRead(t *testing.T) {
	res := analyzeSFC(t, "app/components/ChatWindow.vue", chatWindowVue)

	assert.ElementsMatch(t,
		[]string{"~/components/chat/ChatPanel.vue", "@/components/UserAvatar.vue", "~/utils/dates"},
		valuesOf(res.Snippets, file.ComponentImport))
	// lang="ts" is read as TypeScript, which is what tells a type-only
	// import from one that stays in the program.
	assert.Equal(t, []string{"../types"}, valuesOf(res.Snippets, file.ComponentImportTypeOnly))
	// The script inside the HTML comment is not code.
	assert.NotContains(t, valuesOf(res.Snippets, file.ComponentImport), "./Old.vue")
}

// Positions are the real file's, so a snippet points at the line an editor
// shows and not at a line of the extracted script.
func TestAVueSnippetIsPositionedInTheWholeFile(t *testing.T) {
	res := analyzeSFC(t, "app/components/ChatWindow.vue", chatWindowVue)

	var dates *file.Snippet
	for _, s := range snippetsOfType(res, file.ComponentImport) {
		if s.Value == "~/utils/dates" {
			dates = s
		}
	}
	require.NotNil(t, dates)
	assert.Equal(t, "'~/utils/dates'", chatWindowVue[dates.Begin.Offset:dates.End.Offset])

	lines := strings.Split(chatWindowVue, "\n")
	lineIdx := dates.Begin.Line - 1
	require.Less(t, lineIdx, len(lines))
	assert.Equal(t, "import { formatDate } from '~/utils/dates'", lines[lineIdx])
	assert.Equal(t, strings.Index(lines[lineIdx], "'")+1, dates.Begin.CharInLine)

	fn := snippetsOfType(res, "js__function__declaration")
	require.Len(t, fn, 1)
	assert.Equal(t, "onSend", fn[0].Value)
	assert.Equal(t, "function onSend(text: string) {", lines[fn[0].Begin.Line-1])
}

// The component is the unit a .vue file exports. Its script and its
// template use things, and those uses are its references.
func TestAVueComponentIsAUnitThatUsesWhatItsTemplateNames(t *testing.T) {
	res := analyzeSFC(t, "app/components/ChatWindow.vue", chatWindowVue)
	byID := unitsByID(res.Units)

	cw := byID["app/components/ChatWindow#ChatWindow"]
	require.NotNil(t, cw, "the component is named for its file")
	assert.Equal(t, unit.KindType, cw.Kind)
	assert.True(t, cw.HasMarker("vue_component"))
	assert.Contains(t, cw.Refs, unit.Ref{Module: "~/components/chat/ChatPanel.vue", Name: "ChatPanel"},
		"used as <ChatPanel /> in the template")
	assert.Contains(t, cw.Refs, unit.Ref{Module: "@/components/UserAvatar.vue", Name: "UserAvatar"},
		"used as <user-avatar> in the template")
	assert.NotContains(t, byID, "app/components/ChatWindow#", "the component stands where the module unit would")

	onSend := byID["app/components/ChatWindow#onSend"]
	require.NotNil(t, onSend)
	assert.Equal(t, cw.ID, onSend.Owner, "a function of <script setup> is the component's")
	assert.Contains(t, onSend.Refs, unit.Ref{Module: "~/utils/dates", Name: "formatDate"})
}

// End to end through the resolver: `~/` and `@/` are aliases for the source
// root, `.vue` is spelled out in the import, and an importer may call the
// default whatever it likes.
func TestVueImportsResolveToUnits(t *testing.T) {
	var units []*unit.Unit
	units = append(units, analyzeSFC(t, "app/components/ChatWindow.vue", chatWindowVue).Units...)
	units = append(units, analyzeSFC(t, "app/components/chat/ChatPanel.vue",
		"<template><div /></template>\n<script setup lang=\"ts\">\ndefineProps<{ messages: string[] }>()\n</script>\n").Units...)
	units = append(units, analyzeSFC(t, "app/components/UserAvatar.vue",
		"<template><img /></template>\n").Units...)

	ts := &tsAnalyzer{lp: createTypeScriptLanguagePack(false)}
	for path, src := range map[string]string{
		"app/utils/dates.ts": "export function formatDate(d: Date) { return d.toISOString() }\n",
		"app/router.ts":      "import Window from './components/ChatWindow.vue'\nexport function routes() { return [Window] }\n",
	} {
		res := ts.lp.AnalyzeFileContent(path, []byte(src))
		require.NotNil(t, res)
		units = append(units, common.JSUnitsFrom(path, []byte(src), res)...)
	}

	edges := map[string]bool{}
	for _, c := range unit.Connections(units) {
		edges[c.From+" -> "+c.To] = true
	}
	for _, want := range []string{
		"app/components/ChatWindow#ChatWindow -> app/components/chat/ChatPanel#ChatPanel",
		"app/components/ChatWindow#ChatWindow -> app/components/UserAvatar#UserAvatar",
		"app/components/ChatWindow#onSend -> app/utils/dates#formatDate",
		"app/router#routes -> app/components/ChatWindow#ChatWindow",
	} {
		assert.True(t, edges[want], "missing edge %s; have %v", want, edges)
	}
}

// A plain <script> is JavaScript, and the Options API names what it uses in
// an object rather than in a function.
func TestAVueOptionsAPIComponentInJavaScript(t *testing.T) {
	src := `<script>
import ChatPanel from './ChatPanel.vue'
import { load } from '../api'

export default {
  components: { ChatPanel },
  methods: {
    refresh() { return load() }
  }
}
</script>

<template><ChatPanel /></template>
`
	a := newSFCAnalyzer()
	assert.Same(t, a.js, a.packFor(scriptBlocks([]byte(src))))
	res := analyzeSFC(t, "src/views/Inbox.vue", src)
	byID := unitsByID(res.Units)

	inbox := byID["src/views/Inbox#Inbox"]
	require.NotNil(t, inbox)
	assert.Contains(t, inbox.Refs, unit.Ref{Module: "./ChatPanel.vue", Name: "ChatPanel"})
	refresh := byID["src/views/Inbox#refresh"]
	require.NotNil(t, refresh)
	assert.Equal(t, inbox.ID, refresh.Owner)
	assert.Contains(t, refresh.Refs, unit.Ref{Module: "../api", Name: "load"})
}

func TestASvelteComponentsScriptIsRead(t *testing.T) {
	src := `<script context="module" lang="ts">
  export const prerender = true
</script>

<script lang="ts">
  import Panel from '$lib/Panel.svelte'
  import { session } from './stores'
  let open = $session.open
</script>

<Panel {open} />
`
	res := analyzeSFC(t, "src/routes/Home.svelte", src)
	assert.ElementsMatch(t, []string{"$lib/Panel.svelte", "./stores"},
		valuesOf(res.Snippets, file.ComponentImport))
	home := unitsByID(res.Units)["src/routes/Home#Home"]
	require.NotNil(t, home)
	assert.True(t, home.HasMarker("svelte_component"))
	assert.Contains(t, home.Refs, unit.Ref{Module: "$lib/Panel.svelte", Name: "Panel"})
}

// A template-only component still exists and can be imported.
func TestAVueComponentWithNoScriptIsStillAUnit(t *testing.T) {
	res := analyzeSFC(t, "components/Spacer.vue", "<template><div class=\"spacer\" /></template>\n")
	assert.Empty(t, snippetsOfType(res, file.ComponentImport))
	require.Len(t, res.Units, 1)
	assert.Equal(t, "components/Spacer#Spacer", res.Units[0].ID)
}

func TestOnlyComponentFilesAreClaimed(t *testing.T) {
	assert.Nil(t, newSFCAnalyzer().analyze("src/app.ts", []byte("import x from './y'\n")))
}

// Nuxt names a route parameter in brackets, and its dots are not an
// extension; `.client` is a Nuxt mode suffix, not part of the name.
func TestAComponentIsNamedForItsFile(t *testing.T) {
	for path, want := range map[string]string{
		"pages/files/[...name].vue":       "[...name]",
		"pages/groups/[id].vue":           "[id]",
		"components/ChatPanel.client.vue": "ChatPanel",
		"components/ChatPanel.vue":        "ChatPanel",
		"routes/blog/[slug]/+page.svelte": "+page",
	} {
		assert.Equal(t, want, componentName(path), path)
	}
}
