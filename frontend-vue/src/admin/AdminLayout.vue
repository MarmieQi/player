<script setup>
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import { Activity, ArrowDownToLine, ChevronLeft, Database, LayoutDashboard, LogOut, Menu, PanelLeftClose, Search, Settings, ShieldCheck, UserRound, X } from 'lucide-vue-next'
import { adminNavigation, adminPage } from './navigation'
import './admin-theme.css'
import './admin.css'

const props = defineProps({ active: { type: String, required: true }, account: String, serverTime: String })
const emit = defineEmits(['navigate', 'logout'])
const icons = { overview: LayoutDashboard, search: Search, analysis: Activity, sync: ArrowDownToLine, database: Database, settings: Settings }
const page = computed(() => adminPage(props.active))
const collapsed = ref(false)
const mobile = ref(false)
const drawerOpen = ref(false)
const sidebar = ref(null)
const menuButton = ref(null)
let media
let compactMedia
let previousOverflow
let drawerEffect = 0

function closeDrawer() { drawerOpen.value = false }
function selectPage(id) {
  closeDrawer()
  emit('navigate', id)
}
function updateViewport() {
  mobile.value = media.matches
  collapsed.value = compactMedia.matches
  closeDrawer()
}
function handleDrawerKey(event) {
  if (!drawerOpen.value) return
  if (event.key === 'Escape') { event.preventDefault(); closeDrawer(); return }
  if (event.key !== 'Tab') return
  const controls = [...sidebar.value.querySelectorAll('button:not(:disabled), a[href]')]
  const first = controls[0]
  const last = controls.at(-1)
  if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last?.focus() }
  else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first?.focus() }
}
watch(drawerOpen, async open => {
  const effect = ++drawerEffect
  if (open) {
    previousOverflow = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    await nextTick()
    requestAnimationFrame(() => {
      if (effect === drawerEffect) sidebar.value?.querySelector('button')?.focus()
    })
  } else {
    if (previousOverflow !== undefined) {
      document.body.style.overflow = previousOverflow
      previousOverflow = undefined
    }
    await nextTick()
    if (effect === drawerEffect && mobile.value) menuButton.value?.focus()
  }
})
watch(() => props.active, closeDrawer)
onMounted(() => {
  media = window.matchMedia('(max-width: 767px)')
  compactMedia = window.matchMedia('(min-width: 768px) and (max-width: 1199px)')
  updateViewport()
  media.addEventListener('change', updateViewport)
  compactMedia.addEventListener('change', updateViewport)
  document.addEventListener('keydown', handleDrawerKey)
})
onUnmounted(() => {
  ++drawerEffect
  media?.removeEventListener('change', updateViewport)
  compactMedia?.removeEventListener('change', updateViewport)
  document.removeEventListener('keydown', handleDrawerKey)
  if (previousOverflow !== undefined) document.body.style.overflow = previousOverflow
})
</script>

<template>
  <div class="admin-workspace" :class="{ 'nav-collapsed': collapsed, 'drawer-open': drawerOpen }">
    <a class="admin-skip-link" href="#admin-main">跳到主要内容</a>
    <div v-if="drawerOpen" class="admin-nav-backdrop" aria-hidden="true" @click="closeDrawer"></div>
    <aside id="admin-navigation" ref="sidebar" class="admin-sidebar" :inert="mobile && !drawerOpen" :aria-hidden="mobile && !drawerOpen ? true : undefined" :role="mobile && drawerOpen ? 'dialog' : undefined" :aria-modal="mobile && drawerOpen ? true : undefined" aria-label="后台导航">
      <div class="admin-brand">
        <span class="admin-brand-mark"><Database :size="23" /></span>
        <div class="admin-nav-copy"><strong>日志管理</strong><span>服务器管理控制台</span></div>
        <button class="admin-mobile-close" type="button" aria-label="关闭导航" @click="closeDrawer"><X :size="20" /></button>
      </div>
      <nav class="admin-navigation" aria-label="管理菜单">
        <div v-for="group in adminNavigation" :key="group.label" class="admin-nav-group">
          <p class="admin-group-label">{{ group.label }}</p>
          <button v-for="item in group.items" :key="item.id" type="button" class="admin-nav-item" :class="{ active: active === item.id }" :aria-current="active === item.id ? 'page' : undefined" :title="item.label" @click="selectPage(item.id)">
            <component :is="icons[item.icon]" :size="19" />
            <span class="admin-nav-copy">{{ item.label }}</span>
            <span v-if="active === item.id" class="admin-nav-indicator"></span>
          </button>
        </div>
      </nav>
      <div class="admin-sidebar-bottom">
        <button class="admin-collapse-toggle" type="button" :aria-expanded="!collapsed" aria-label="切换侧栏宽度" @click="collapsed = !collapsed"><PanelLeftClose :size="18" /><span class="admin-nav-copy">收起导航</span></button>
        <button class="admin-account" :class="{ active: active === 'profile' }" type="button" title="个人中心" @click="selectPage('profile')">
          <span class="admin-avatar"><UserRound :size="19" /></span>
          <span class="admin-nav-copy"><strong>{{ account || '管理员' }}</strong><small>管理员账号</small></span>
          <ChevronLeft class="admin-account-arrow" :size="16" />
        </button>
        <button class="admin-logout" type="button" title="退出登录" @click="emit('logout')"><LogOut :size="17" /><span class="admin-nav-copy">退出登录</span></button>
      </div>
    </aside>
    <div class="admin-frame" :inert="mobile && drawerOpen">
      <header class="admin-header">
        <div class="admin-header-leading">
          <button ref="menuButton" class="admin-menu-toggle" type="button" aria-label="打开导航" aria-controls="admin-navigation" :aria-expanded="drawerOpen" @click="drawerOpen = true"><Menu :size="21" /></button>
          <div><h1>{{ page.label }}</h1><p class="admin-page-description">{{ page.description }}</p></div>
        </div>
        <div class="admin-header-meta"><span v-if="serverTime" class="admin-clock">{{ serverTime }}</span><span class="admin-security"><ShieldCheck :size="16" /><span>管理员</span></span></div>
      </header>
      <div id="admin-main" class="admin-main" tabindex="-1">
        <slot />
      </div>
      <footer class="admin-footer">日志管理 <span>·</span> 服务器行为管理</footer>
    </div>
  </div>
</template>
