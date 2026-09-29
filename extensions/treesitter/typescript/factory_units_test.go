package typescript

import (
	"testing"

	"github.com/archstats/archstats/core/unit"
	"github.com/archstats/archstats/extensions/treesitter/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func tsUnits(t *testing.T, path, src string) map[string]*unit.Unit {
	t.Helper()
	res := createTypeScriptLanguagePack(false).AnalyzeFileContent(path, []byte(src))
	require.NotNil(t, res)
	return unitsByID(common.JSUnitsFrom(path, []byte(src), res))
}

// archstats-ui's reports.store.ts read as five interfaces and 55 loose
// functions: the store, a constant, was no unit, and its actions belonged to
// nothing.
func TestAPiniaOptionsStoreIsAUnitThatOwnsItsActions(t *testing.T) {
	byID := tsUnits(t, "src/stores/reports.store.ts", `
import { defineStore } from "pinia"
import { saveReport } from "~/platform/api"

export interface ReportRecord { id: string }

export const useReportsStore = defineStore("reports", {
  state: () => ({ open: [] as ReportRecord[] }),
  actions: {
    async save(r: ReportRecord) { await saveReport(r) },
    undo() {},
  },
})
`)
	store := byID["src/stores/reports.store#useReportsStore"]
	require.NotNil(t, store)
	assert.Equal(t, unit.KindType, store.Kind)
	assert.True(t, store.HasMarker("defineStore"), "the factory is what it is made from")

	save := byID["src/stores/reports.store#useReportsStore.save"]
	require.NotNil(t, save, "an action is the store's")
	assert.Equal(t, store.ID, save.Owner)
	assert.Contains(t, save.Refs, unit.Ref{Module: "~/platform/api", Name: "saveReport"})
	assert.NotNil(t, byID["src/stores/reports.store#useReportsStore.undo"])
	assert.Nil(t, byID["src/stores/reports.store#save"], "not a loose function of the file")
	assert.NotNil(t, byID["src/stores/reports.store#ReportRecord"])
}

// A setup store declares its actions as functions inside the call.
func TestAPiniaSetupStoreOwnsTheFunctionsDeclaredInIt(t *testing.T) {
	byID := tsUnits(t, "src/stores/cart.ts", `
import { defineStore } from "pinia"
import { ref } from "vue"

export const useCartStore = defineStore("cart", () => {
  const items = ref<string[]>([])
  function add(id: string) { items.value.push(id) }
  const clear = () => { items.value = [] }
  return { items, add, clear }
})

export function totalOf(ids: string[]) { return ids.length }
`)
	store := byID["src/stores/cart#useCartStore"]
	require.NotNil(t, store)
	for _, action := range []string{"add", "clear"} {
		a := byID["src/stores/cart#useCartStore."+action]
		require.NotNil(t, a, action)
		assert.Equal(t, store.ID, a.Owner, action)
	}
	total := byID["src/stores/cart#totalOf"]
	require.NotNil(t, total, "a function after the store is the file's own")
	assert.Empty(t, total.Owner)
}

func TestAReduxSliceIsAUnit(t *testing.T) {
	byID := tsUnits(t, "src/features/cart/slice.ts", `
import { createSlice } from "@reduxjs/toolkit"
export const cartSlice = createSlice({
  name: "cart",
  initialState: [] as string[],
  reducers: { add(state, action) { state.push(action.payload) } },
})
`)
	slice := byID["src/features/cart/slice#cartSlice"]
	require.NotNil(t, slice)
	assert.True(t, slice.HasMarker("createSlice"))
	assert.Equal(t, slice.ID, byID["src/features/cart/slice#cartSlice.add"].Owner)
}

// Vue's compiler macros and any other call are values, not units.
func TestAConstantFromAnyOtherCallIsNoUnit(t *testing.T) {
	byID := tsUnits(t, "src/components/panel.ts", `
import { computed, ref } from "vue"
const props = defineProps<{ id: string }>()
const emit = defineEmits(["close"])
const count = ref(0)
const doubled = computed(() => count.value * 2)
const store = useCartStore()
`)
	for _, name := range []string{"props", "emit", "count", "doubled", "store"} {
		assert.Nil(t, byID["src/components/panel#"+name], name)
	}
}

// In a .vue file a store defined inline is still a store, and the macros
// around it are not.
func TestAStoreDefinedInAVueComponentsScriptIsAUnit(t *testing.T) {
	res := analyzeSFC(t, "src/components/Cart.vue", `<template><div /></template>
<script setup lang="ts">
import { defineStore } from "pinia"
const props = defineProps<{ id: string }>()
const useLocal = defineStore("local", { actions: { ping() {} } })
</script>`)
	byID := unitsByID(res.Units)
	assert.NotNil(t, byID["src/components/Cart#useLocal"])
	assert.Nil(t, byID["src/components/Cart#props"])
}
