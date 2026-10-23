<script setup>
import { computed, nextTick, onUnmounted, ref, useId, watch } from 'vue'
import { Check, ChevronDown } from 'lucide-vue-next'

const props = defineProps({
  modelValue: [String, Number],
  options: { type: Array, required: true },
  label: { type: String, required: true },
  disabled: Boolean
})
const emit = defineEmits(['update:modelValue'])
const id = useId()
const trigger = ref(null)
const menu = ref(null)
const open = ref(false)
const active = ref(0)
const menuStyle = ref({})
const selectedIndex = computed(() => props.options.findIndex(option => option.value === props.modelValue))
const selected = computed(() => props.options[selectedIndex.value])

function close() { open.value = false }
function outside(event) {
  if (!trigger.value?.contains(event.target) && !menu.value?.contains(event.target)) close()
}
function closeOnScroll(event) {
  if (!menu.value?.contains(event.target)) close()
}
async function reveal() {
  if (props.disabled || !props.options.length) return
  const bounds = trigger.value.getBoundingClientRect()
  const below = window.innerHeight - bounds.bottom - 16
  const above = bounds.top - 16
  const upwards = below < Math.min(180, props.options.length * 38 + 8) && above > below
  const theme = getComputedStyle(trigger.value)
  const width = Math.min(Math.max(bounds.width, 120), window.innerWidth - 24)
  menuStyle.value = {
    left: `${Math.max(12, Math.min(bounds.left, window.innerWidth - width - 12))}px`,
    width: `${width}px`,
    maxHeight: `${Math.max(64, Math.min(248, upwards ? above : below))}px`,
    ...(upwards ? { bottom: `${window.innerHeight - bounds.top + 6}px` } : { top: `${bounds.bottom + 6}px` }),
    '--select-accent': theme.getPropertyValue('--admin-accent').trim() || '#465fc9',
    '--select-soft': theme.getPropertyValue('--admin-accent-soft').trim() || '#edf1ff'
  }
  active.value = Math.max(0, selectedIndex.value)
  open.value = true
  await nextTick()
  keepVisible()
}
function keepVisible() {
  menu.value?.children[active.value]?.scrollIntoView({ block: 'nearest' })
}
function choose(index) {
  const option = props.options[index]
  if (!option) return
  emit('update:modelValue', option.value)
  close()
  trigger.value?.focus()
}
async function onKey(event) {
  if (props.disabled) return
  if (event.key === 'Tab') { close(); return }
  if (event.key === 'Escape') {
    if (open.value) { event.preventDefault(); event.stopPropagation(); close() }
    return
  }
  if (['ArrowDown', 'ArrowUp', 'Home', 'End'].includes(event.key)) {
    event.preventDefault()
    if (!open.value) {
      await reveal()
      if (event.key === 'Home') active.value = 0
      else if (event.key === 'End') active.value = props.options.length - 1
      await nextTick()
      keepVisible()
      return
    }
    active.value = event.key === 'Home' ? 0 : event.key === 'End' ? props.options.length - 1
      : Math.max(0, Math.min(props.options.length - 1, active.value + (event.key === 'ArrowDown' ? 1 : -1)))
    await nextTick()
    keepVisible()
  } else if (event.key === 'Enter' || event.key === ' ') {
    event.preventDefault()
    if (open.value) choose(active.value)
    else await reveal()
  } else if (open.value && event.key.length === 1 && !event.ctrlKey && !event.metaKey) {
    const index = props.options.findIndex(option => String(option.label).toLowerCase().startsWith(event.key.toLowerCase()))
    if (index >= 0) { event.preventDefault(); active.value = index; await nextTick(); keepVisible() }
  }
}
function removeListeners() {
  document.removeEventListener('pointerdown', outside, true)
  window.removeEventListener('resize', close)
  window.removeEventListener('scroll', closeOnScroll, true)
}
watch(open, value => {
  if (!value) { removeListeners(); return }
  document.addEventListener('pointerdown', outside, true)
  window.addEventListener('resize', close)
  window.addEventListener('scroll', closeOnScroll, true)
})
watch(() => props.disabled, value => { if (value) close() })
watch(() => props.modelValue, () => { active.value = Math.max(0, selectedIndex.value) })
onUnmounted(removeListeners)
</script>

<template>
  <span class="admin-select-root">
    <button ref="trigger" type="button" role="combobox" class="admin-select-trigger" :class="{ 'is-open': open }" :aria-label="label" aria-haspopup="listbox" :aria-expanded="open" :aria-controls="`${id}-menu`" :aria-activedescendant="open ? `${id}-option-${active}` : undefined" :disabled="disabled" @click="open ? close() : reveal()" @keydown="onKey">
      <span>{{ selected?.label ?? '请选择' }}</span><ChevronDown :size="15" aria-hidden="true" />
    </button>
    <Teleport to="body">
      <div v-if="open" :id="`${id}-menu`" ref="menu" role="listbox" :aria-label="label" class="admin-select-menu" :style="menuStyle">
        <div v-for="(option, index) in options" :id="`${id}-option-${index}`" :key="option.value" role="option" :aria-selected="option.value === modelValue" class="admin-select-option" :class="{ 'is-active': index === active, 'is-selected': option.value === modelValue }" @pointermove="active = index" @mousedown.prevent @click="choose(index)">
          <span>{{ option.label }}</span><Check v-if="option.value === modelValue" :size="15" aria-hidden="true" />
        </div>
      </div>
    </Teleport>
  </span>
</template>

<style scoped>
.admin-select-root { display: inline-flex; min-width: 76px; vertical-align: middle; }
.admin-select-trigger { display: inline-flex; align-items: center; justify-content: space-between; gap: 16px; width: 100%; min-height: 38px; margin: 0; padding: 0 11px; border: 1px solid #d9e2eb; border-radius: 8px; background: #fff; color: #34445b; font: inherit; font-size: 14px; font-weight: 400; line-height: 1.4; cursor: pointer; white-space: nowrap; }
.admin-select-trigger svg { color: #7b899b; flex-shrink: 0; }
.admin-select-trigger:hover:not(:disabled) { border-color: #b8c5d5; }
.admin-select-trigger:focus-visible, .admin-select-trigger.is-open { outline: none; border-color: var(--admin-accent, #465fc9); box-shadow: 0 0 0 3px var(--admin-accent-shadow, #526bd41a); }
.admin-select-trigger:disabled { opacity: .55; cursor: not-allowed; }
.admin-select-menu { position: fixed; z-index: 120; box-sizing: border-box; overflow-y: auto; overscroll-behavior: contain; margin: 0; padding: 5px; border: 1px solid #e0e5ee; border-radius: 10px; background: #fff; box-shadow: 0 8px 24px #1a2b441a, 0 2px 5px #1a2b4408; color: #34445b; font-family: "Segoe UI", "Microsoft YaHei", system-ui, sans-serif; font-size: 14px; }
.admin-select-option { display: flex; align-items: center; justify-content: space-between; gap: 16px; min-height: 36px; box-sizing: border-box; padding: 8px 10px; border-radius: 6px; line-height: 1.4; cursor: pointer; user-select: none; }
.admin-select-option.is-active { background: #f3f5f9; }
.admin-select-option.is-selected { color: var(--select-accent); font-weight: 500; }
.admin-select-option.is-active.is-selected { background: var(--select-soft); }
@media (forced-colors: active) { .admin-select-option.is-active { outline: 1px solid Highlight; } }
@media (pointer: coarse) { .admin-select-trigger { min-height: 40px; } .admin-select-option { min-height: 44px; } }
</style>
