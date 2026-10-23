<script setup>
import { nextTick, onUnmounted, ref, useId, watch } from 'vue'
import { ChevronUp, LogOut, UserRound } from 'lucide-vue-next'

defineProps({ account: String, active: Boolean })
const emit = defineEmits(['profile', 'logout'])
const id = useId()
const root = ref(null)
const trigger = ref(null)
const panel = ref(null)
const open = ref(false)

function close(restoreFocus = false) {
  open.value = false
  if (restoreFocus) trigger.value?.focus()
}
async function show(last = false) {
  open.value = true
  await nextTick()
  const items = panel.value?.querySelectorAll('[role="menuitem"]')
  const target = last ? items?.[items.length - 1] : items?.[0]
  target?.focus()
}
function outside(event) { if (!root.value?.contains(event.target)) close() }
function leave(event) { if (!root.value?.contains(event.relatedTarget)) close() }
function choose(action) { close(true); emit(action) }
function keyboard(event) {
  if (event.key === 'Escape') { event.preventDefault(); event.stopPropagation(); close(true); return }
  if (!['ArrowDown', 'ArrowUp', 'Home', 'End'].includes(event.key)) return
  event.preventDefault()
  const items = [...panel.value.querySelectorAll('[role="menuitem"]')]
  const index = items.indexOf(document.activeElement)
  const next = event.key === 'Home' ? 0 : event.key === 'End' ? items.length - 1
    : (index + (event.key === 'ArrowDown' ? 1 : -1) + items.length) % items.length
  items[next]?.focus()
}
watch(open, value => {
  if (value) document.addEventListener('pointerdown', outside)
  else document.removeEventListener('pointerdown', outside)
})
onUnmounted(() => document.removeEventListener('pointerdown', outside))
</script>

<template>
  <div ref="root" class="admin-account-control" @focusout="leave">
    <div v-if="open" :id="id" ref="panel" class="admin-account-menu" role="menu" aria-label="账号操作" @keydown="keyboard">
      <p>账号管理</p>
      <button type="button" role="menuitem" :class="{ active }" @click="choose('profile')"><UserRound :size="17" /><span>个人中心</span></button>
      <div class="admin-account-divider" role="separator"></div>
      <button type="button" role="menuitem" class="admin-account-signout" @click="choose('logout')"><LogOut :size="17" /><span>退出登录</span></button>
    </div>
    <button ref="trigger" class="admin-account" :class="{ active: active || open }" type="button" title="账号菜单" aria-label="账号菜单" aria-haspopup="menu" :aria-controls="open ? id : undefined" :aria-expanded="open" @click="open ? close() : show()" @keydown.down.prevent="show()" @keydown.up.prevent="show(true)">
      <span class="admin-avatar"><UserRound :size="19" /></span>
      <span class="admin-nav-copy"><strong>{{ account || '管理员' }}</strong><small>管理员账号</small></span>
      <ChevronUp class="admin-account-arrow" :class="{ 'is-open': open }" :size="16" />
    </button>
  </div>
</template>
