<script setup>
import { computed, nextTick, onMounted, onUnmounted, reactive, ref } from 'vue'
import { Save, Check, AlertCircle } from 'lucide-vue-next'
import { apiGet, apiPutJson, getAuthToken } from '../api.js'
import { settingsTabs, settingsDraft, validateSettings } from './settings-model.js'
import SettingsSmbPanel from './settings/SettingsSmbPanel.vue'
import SettingsSchedulePanel from './settings/SettingsSchedulePanel.vue'
import SettingsSharingPanel from './settings/SettingsSharingPanel.vue'
import SettingsRuntimePanel from './settings/SettingsRuntimePanel.vue'
import SettingsLogsPanel from './settings/SettingsLogsPanel.vue'
import './settings/settings.css'

const emit = defineEmits(['unauthorized', 'saved'])
const active = ref('smb')
const sections = reactive(Object.fromEntries(settingsTabs.filter(tab => tab.id !== 'logs').map(tab => [tab.id, { draft: null, response: null, saved: '', loading: false, saving: false, error: '', success: '' }])))
const current = computed(() => sections[active.value])
const tab = computed(() => settingsTabs.find(tab => tab.id === active.value))
const dirty = section => Boolean(section?.draft) && JSON.stringify(section.draft) !== section.saved
const hasUnsaved = computed(() => Object.values(sections).some(section => dirty(section)))
const saving = computed(() => Object.values(sections).some(section => section.saving))
const abort = new AbortController()
function report(section, err) {
  if (err.name === 'AbortError') return
  section.error = err.message
  if (!getAuthToken()) emit('unauthorized')
}
function accept(id, response) {
  const section = sections[id]
  section.response = response; section.draft = settingsDraft(id, response); section.saved = JSON.stringify(section.draft)
}
async function load(id, refresh = false) {
  const section = sections[id]
  if (!section || section.loading || section.saving || (section.draft && (!refresh || dirty(section)))) return
  section.loading = true; section.error = ''; section.success = ''
  try { accept(id, await apiGet(`/api/admin/settings/${id}`, {}, { signal: abort.signal })) }
  catch (err) { report(section, err) }
  finally { section.loading = false }
}
function select(id) { active.value = id; load(id, id === 'runtime') }
async function tabKey(event, index) {
  const next = event.key === 'ArrowRight' ? (index + 1) % settingsTabs.length : event.key === 'ArrowLeft' ? (index + settingsTabs.length - 1) % settingsTabs.length : event.key === 'Home' ? 0 : event.key === 'End' ? settingsTabs.length - 1 : null
  if (next === null) return
  event.preventDefault()
  const buttons = event.currentTarget.parentElement.children
  select(settingsTabs[next].id)
  await nextTick()
  buttons[next].focus()
}
async function save() {
  const id = active.value, section = sections[id]
  if (!section || section.saving || !dirty(section)) return
  section.error = ''; section.success = ''
  const validation = validateSettings(id, section.draft)
  if (validation) { section.error = validation; return }
  section.saving = true
  try {
    accept(id, await apiPutJson(`/api/admin/settings/${id}`, section.draft, { signal: abort.signal }))
    section.success = id === 'runtime' && section.response.restartRequired ? '已保存，重启后端后生效' : '已保存'
    emit('saved', id)
  } catch (err) { report(section, err) }
  finally { section.saving = false }
}
function canLeave() {
  if (saving.value) { window.alert('正在保存设置，请稍候'); return false }
  return !hasUnsaved.value || window.confirm('有未保存的设置，确定离开吗？')
}
function beforeUnload(event) { if (hasUnsaved.value || saving.value) { event.preventDefault(); event.returnValue = '' } }
defineExpose({ canLeave })
onMounted(() => { load('smb'); window.addEventListener('beforeunload', beforeUnload) })
onUnmounted(() => { abort.abort(); window.removeEventListener('beforeunload', beforeUnload) })
</script>

<template>
  <section class="system-settings">
    <nav class="settings-tabs" aria-label="系统设置分组" role="tablist">
      <button v-for="(item, index) in settingsTabs" :id="`settings-tab-${item.id}`" :key="item.id" type="button" role="tab" :tabindex="active === item.id ? 0 : -1" :aria-selected="active === item.id" :aria-controls="`settings-panel-${item.id}`" :class="{ active: active === item.id }" @click="select(item.id)" @keydown="tabKey($event, index)">
        {{ item.label }}<span v-if="dirty(sections[item.id])" class="settings-dirty-dot" aria-label="未保存"></span>
      </button>
    </nav>
    <div class="settings-section-heading">
      <div><h2>{{ tab.title }}</h2><p>{{ tab.description }}</p></div>
      <div v-if="current?.draft" class="settings-save-area">
        <span v-if="dirty(current)" class="settings-change-label">有未保存的修改</span>
        <button type="button" class="primary-button" :disabled="current.saving || !dirty(current)" @click="save"><Save :size="16" />{{ current.saving ? '保存中…' : '保存' }}</button>
      </div>
    </div>
    <div :id="`settings-panel-${active}`" role="tabpanel" :aria-labelledby="`settings-tab-${active}`">
      <p v-if="current?.error" class="notice error" role="alert"><AlertCircle :size="16" />{{ current.error }}<button v-if="!current.draft" type="button" class="table-action" @click="load(active)">重新加载</button></p>
      <p v-if="current?.success && !dirty(current)" class="notice success" role="status"><Check :size="16" />{{ current.success }}</p>
      <div v-if="current?.loading" class="empty-state">正在加载…</div>
      <fieldset v-else-if="current?.draft" class="settings-section-body" :disabled="current.saving">
        <SettingsSmbPanel v-if="active === 'smb'" :key="current.saved" :model="current.draft" :dirty="dirty(current)" :saving="current.saving" />
        <SettingsSchedulePanel v-else-if="active === 'schedules'" :model="current.draft" />
        <SettingsSharingPanel v-else-if="active === 'sharing'" :model="current.draft" />
        <SettingsRuntimePanel v-else-if="active === 'runtime'" :model="current.draft" :response="current.response" />
      </fieldset>
      <SettingsLogsPanel v-else-if="active === 'logs'" />
    </div>
  </section>
</template>
