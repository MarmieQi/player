<script setup>
import { onMounted, onUnmounted, ref, useId, watch } from 'vue'
import { Search, UserRound, ArrowUpLeft } from 'lucide-vue-next'
import { apiGet } from '../api.js'
import { createPlayerSuggestionSearch } from './player-suggestions.js'

const props = defineProps({ modelValue: { type: String, default: '' }, serverId: { type: String, required: true } })
const emit = defineEmits(['update:modelValue', 'change'])
const id = useId()
const root = ref(null)
const open = ref(false)
const active = ref(-1)
const composing = ref(false)
const state = ref({ items: [], loading: false, error: '' })
const suggestions = createPlayerSuggestionSearch({
  request: (serverId, query, signal) => apiGet('/api/stats/player-suggestions', { serverId, q: query }, { signal }),
  onChange: value => { state.value = value; active.value = -1 }
})
function input(event) {
  const value = event.target.value
  emit('update:modelValue', value)
  if (composing.value || event.isComposing) return
  open.value = Boolean(value.trim())
  suggestions.search(props.serverId, value)
}
function focus() {
  open.value = Boolean(props.modelValue.trim())
  suggestions.search(props.serverId, props.modelValue)
}
function close() { open.value = false; active.value = -1; suggestions.cancel() }
function choose(name) { emit('update:modelValue', name); emit('change'); close() }
function outside(event) { if (!root.value?.contains(event.target)) close() }
function keys(event) {
  if (composing.value || event.isComposing) return
  if (event.key === 'Escape') { if (open.value) { event.preventDefault(); event.stopPropagation(); close() } }
  else if (['ArrowDown', 'ArrowUp'].includes(event.key) && state.value.items.length) {
    event.preventDefault(); open.value = true
    active.value = (active.value + (event.key === 'ArrowDown' ? 1 : -1) + state.value.items.length) % state.value.items.length
  } else if (event.key === 'Enter' && open.value && active.value >= 0 && state.value.items[active.value]) {
    event.preventDefault(); choose(state.value.items[active.value])
  }
}
watch(() => props.serverId, () => { close(); state.value = { items: [], loading: false, error: '' } })
onMounted(() => document.addEventListener('pointerdown', outside))
onUnmounted(() => { suggestions.cancel(); document.removeEventListener('pointerdown', outside) })
</script>
<template>
  <div ref="root" class="player-name-autocomplete">
    <label :for="id">玩家名</label>
    <div class="player-picker-input"><input :id="id" :value="modelValue" type="text" placeholder="输入玩家名搜索" autocomplete="off" spellcheck="false" role="combobox" aria-autocomplete="list" :aria-expanded="Boolean(open && modelValue.trim())" :aria-controls="`${id}-names`" :aria-activedescendant="active >= 0 && open ? `${id}-name-${active}` : undefined" @input="input" @focus="focus" @blur="close" @keydown="keys" @change="emit('change')" @compositionstart="composing = true; close()" @compositionend="composing = false; input($event)" /><Search :size="15" aria-hidden="true" /></div>
    <div v-if="open && modelValue.trim()" class="player-picker-menu">
      <div class="player-picker-caption">匹配玩家</div>
      <div :id="`${id}-names`" role="listbox" aria-label="匹配的玩家" :aria-busy="state.loading">
        <button v-for="(name, index) in state.items" :id="`${id}-name-${index}`" :key="name" type="button" role="option" tabindex="-1" :aria-selected="index === active" :class="{ active: index === active }" @pointerdown.prevent @click="choose(name)"><UserRound :size="16" /><span>{{ name }}</span><ArrowUpLeft :size="14" /></button>
      </div>
      <p v-if="state.loading" role="status">正在查找…</p>
      <p v-else-if="state.error" role="status">{{ state.error }}</p>
      <p v-else-if="!state.items.length" role="status">暂无匹配玩家，可直接输入完整名字查询</p>
    </div>
  </div>
</template>
<style scoped>
.player-name-autocomplete { position: relative; display: grid; align-content: start; gap: 6px; min-width: 0; }
.player-name-autocomplete label { font-size: 13px; font-weight: 500; color: #46556a; }
.player-picker-input { position: relative; }
.player-picker-input input { width: 100%; box-sizing: border-box; padding-right: 34px; }
.player-picker-input > svg { position: absolute; right: 12px; top: 50%; transform: translateY(-50%); color: #94a1b3; pointer-events: none; }
.player-picker-menu { position: absolute; top: calc(100% + 6px); left: 0; width: 100%; box-sizing: border-box; z-index: 40; padding: 5px; border: 1px solid #e0e5ee; border-radius: 9px; background: #fff; box-shadow: 0 8px 24px #1a2b441a; }
.player-picker-caption { display: flex; justify-content: space-between; padding: 7px 9px; color: #8291a4; font-size: 12px; }
.player-picker-menu button { display: flex; align-items: center; gap: 9px; width: 100%; min-height: 38px; padding: 9px; border: 0; border-radius: 6px; background: #fff; color: #34485e; text-align: left; font-size: 14px; cursor: pointer; }
.player-picker-menu button:hover, .player-picker-menu button.active { background: #edf1ff; color: #465fc9; }
.player-picker-menu button span { flex: 1; min-width: 0; overflow-wrap: anywhere; }
.player-picker-menu button > svg { flex-shrink: 0; color: #8795ac; }
.player-picker-menu p { margin: 0; padding: 9px; color: #8291a4; font-size: 12px; line-height: 1.6; }
</style>
