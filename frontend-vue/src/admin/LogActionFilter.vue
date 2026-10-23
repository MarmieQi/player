<script setup>
import { computed, nextTick, onMounted, onUnmounted, ref, useId } from 'vue'
import { Check, ChevronDown, X } from 'lucide-vue-next'
import { logActionOptions } from './log-action-options.js'

const props = defineProps({ modelValue: { type: Array, default: () => [] } })
const emit = defineEmits(['update:modelValue'])
const id = useId()
const root = ref(null)
const trigger = ref(null)
const open = ref(false)
const options = logActionOptions.filter(option => option.value)
const selected = computed(() => props.modelValue.map(value => ({ value, label: options.find(option => option.value === value)?.label || value })))
function toggle(value) {
  emit('update:modelValue', props.modelValue.includes(value)
    ? props.modelValue.filter(item => item !== value)
    : [...props.modelValue, value])
}
function outside(event) {
  if (!root.value?.contains(event.target)) open.value = false
}
function tabOutside() {
  // Wait for the browser's Tab navigation; pointer clicks inside the panel
  // must not close it when blank space moves focus to the document body.
  window.setTimeout(() => {
    if (!root.value?.contains(document.activeElement)) open.value = false
  }, 0)
}
async function escape(event) {
  if (!open.value) return
  event.preventDefault()
  event.stopPropagation()
  open.value = false
  await nextTick()
  trigger.value?.focus()
}
onMounted(() => {
  document.addEventListener('pointerdown', outside)
})
onUnmounted(() => {
  document.removeEventListener('pointerdown', outside)
})
</script>

<template>
  <fieldset ref="root" class="log-action-filter" @keydown.esc="escape" @keydown.tab="tabOutside">
    <legend :id="`${id}-label`">行为筛选 <span>可多选</span></legend>
    <div class="log-action-control" :class="{ 'is-open': open }" @click="open = !open">
      <div class="log-action-tags">
        <button v-for="option in selected" :key="option.value" type="button" class="log-action-tag" :aria-label="`移除${option.label}`" @click.stop="toggle(option.value)">
          <span>{{ option.label }}</span><X :size="12" aria-hidden="true" />
        </button>
        <span v-if="!modelValue.length" class="log-action-placeholder">全部行为</span>
      </div>
      <div class="log-action-controls">
        <button v-if="modelValue.length" class="log-action-clear" type="button" aria-label="清空行为筛选" title="清空行为筛选" @click.stop="emit('update:modelValue', [])"><X :size="16" /></button>
        <button ref="trigger" class="log-action-trigger" type="button" aria-label="展开行为筛选" :aria-expanded="open" :aria-controls="`${id}-options`" @click.stop="open = !open">
          <ChevronDown :size="16" aria-hidden="true" :class="{ rotated: open }" />
        </button>
      </div>
    </div>
    <div v-if="open" :id="`${id}-options`" class="log-action-dropdown" role="group" :aria-labelledby="`${id}-label`">
      <div v-if="modelValue.length" class="log-action-menu-heading"><span>已选 {{ modelValue.length }} 项</span></div>
      <div class="log-action-options">
        <button type="button" class="log-action-option" :class="{ selected: !modelValue.length }" :aria-pressed="!modelValue.length" @click="emit('update:modelValue', [])">全部行为</button>
        <button v-for="option in options" :key="option.value" type="button" class="log-action-option" :class="{ selected: modelValue.includes(option.value) }" :aria-pressed="modelValue.includes(option.value)" @click="toggle(option.value)">
          <span class="log-action-check" aria-hidden="true"><Check v-if="modelValue.includes(option.value)" :size="12" /></span>
          <span>{{ option.label }}</span>
        </button>
      </div>
    </div>
  </fieldset>
</template>

<style scoped>
.log-action-filter { position: relative; width: 100%; min-width: 0; box-sizing: border-box; margin: 0; padding: 0; border: 0; }
.log-action-filter legend { margin-bottom: 8px; padding: 0; color: var(--admin-text, #34485e); font-size: 13px; font-weight: 500; }
.log-action-filter legend span { margin-left: 7px; color: var(--admin-muted, #8291a4); font-size: 12px; font-weight: 400; }
.log-action-control { display: flex; align-items: center; gap: 6px; min-height: 42px; box-sizing: border-box; padding: 6px 8px; border: 1px solid var(--admin-border, #d9e2eb); border-radius: 8px; background: #fff; cursor: pointer; }
.log-action-control:focus-within, .log-action-control.is-open { border-color: var(--admin-accent, #465fc9); box-shadow: 0 0 0 3px var(--admin-accent-shadow, #526bd41a); }
.log-action-tags { display: flex; flex: 1; align-items: center; flex-wrap: wrap; gap: 6px; min-width: 0; }
.log-action-filter button { display: inline-flex; align-items: center; justify-content: center; gap: 6px; margin: 0; font: inherit; font-size: 13px; font-weight: 400; cursor: pointer; }
.log-action-filter .log-action-tag { min-width: 0; min-height: 28px; max-width: 100%; padding: 4px 8px; border: 0; border-radius: 5px; background: var(--admin-accent-soft, #edf1ff); color: var(--admin-accent, #465fc9); }
.log-action-tag span { min-width: 0; overflow-wrap: anywhere; text-align: left; }
.log-action-tag svg { flex-shrink: 0; }
.log-action-filter .log-action-tag:hover { background: #e1e7fc; }
.log-action-filter .log-action-trigger { flex: 0 0 28px; width: 28px; min-height: 28px; padding: 5px; border-radius: 5px; border: 0; background: transparent; color: #8291a4; text-align: left; }
.log-action-placeholder { padding: 4px; color: #8291a4; font-size: 13px; }
.log-action-controls { display: flex; flex: 0 0 auto; align-items: center; align-self: center; gap: 4px; margin-left: 4px; padding-left: 8px; border-left: 1px solid #edf0f6; }
.log-action-trigger svg { flex-shrink: 0; }
.log-action-filter .log-action-trigger:hover { background: #f0f3f8; color: #46556a; }
.log-action-trigger svg.rotated { transform: rotate(180deg); }
.log-action-filter .log-action-clear { flex: 0 0 28px; min-height: 28px; padding: 5px; border: 0; border-radius: 5px; background: transparent; color: #8291a4; }
.log-action-filter .log-action-clear:hover { background: #f0f3f8; color: #46556a; }
.log-action-dropdown { position: absolute; top: calc(100% + 6px); left: 0; z-index: 30; width: min(100%, 520px); box-sizing: border-box; padding: 6px; border: 1px solid #e0e5ee; border-radius: 10px; background: #fff; box-shadow: 0 8px 24px #1a2b441a; }
.log-action-menu-heading { display: flex; align-items: center; justify-content: space-between; gap: 10px; padding: 4px 8px 7px; border-bottom: 1px solid #edf0f6; color: #8291a4; font-size: 12px; }
.log-action-options { display: flex; flex-wrap: wrap; gap: 8px; max-height: 260px; overflow-y: auto; overscroll-behavior: contain; padding: 12px 6px 6px; }
.log-action-filter .log-action-option { display: flex; align-items: center; gap: 8px; min-width: 0; min-height: 38px; box-sizing: border-box; padding: 8px 12px; border: 1px solid #d9e2eb; border-radius: 7px; background: #fff; color: #526075; font-size: 13px; font-weight: 400; cursor: pointer; }
.log-action-option:hover { background: #f5f7fb; }
.log-action-filter .log-action-option.selected { border-color: var(--admin-accent, #465fc9); background: var(--admin-accent-soft, #edf1ff); color: var(--admin-accent, #465fc9); }
.log-action-check { display: inline-flex; flex: 0 0 14px; align-items: center; justify-content: center; width: 14px; height: 14px; box-sizing: border-box; border: 1px solid #c4cedd; border-radius: 4px; background: #fff; }
.selected .log-action-check { border-color: var(--admin-accent, #465fc9); background: var(--admin-accent, #465fc9); color: #fff; }
.log-action-check svg { flex-shrink: 0; }
.log-action-option span { overflow-wrap: anywhere; }
.log-action-filter button:focus-visible { outline: 2px solid var(--admin-accent, #465fc9); outline-offset: 2px; }
.log-action-filter .log-action-trigger:focus-visible { outline: none; box-shadow: none; }
@media (pointer: coarse) { .log-action-filter .log-action-option { min-height: 44px; } .log-action-filter .log-action-tag { min-height: 34px; } }
</style>
