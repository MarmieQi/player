<script setup>
import { computed, onMounted, onUnmounted, ref } from 'vue'
const props = defineProps({ disabled: Boolean })
import { apiGet, apiPutJson } from '../api.js'
const endpoint = '/api/admin/group-chat/copy-schedule'
const schedule = ref({ enabled: false, time: '00:10' })
const saved = ref('')
const loading = ref(true)
const saving = ref(false)
const error = ref('')
const ready = computed(() => !loading.value && Boolean(saved.value))
const dirty = computed(() => JSON.stringify(schedule.value) !== saved.value)
const abort = new AbortController()
function accept(value) { schedule.value = { enabled: value.enabled, time: value.time }; saved.value = JSON.stringify(schedule.value) }
async function save() {
  if (!ready.value) throw new Error('TXT 自动任务配置尚未加载成功')
  if (!dirty.value) return
  saving.value = true; error.value = ''
  try { accept(await apiPutJson(endpoint, schedule.value, { signal: abort.signal })) }
  catch (err) { error.value = err.message; throw err }
  finally { saving.value = false }
}
function validate() {
  if (!ready.value) throw new Error('TXT 自动任务配置尚未加载成功')
  if (!/^([01]\d|2[0-3]):[0-5]\d$/.test(schedule.value.time)) throw new Error('TXT 同步时间必须是有效的 HH:mm 时间')
}
defineExpose({ save, validate, ready })
onMounted(async () => {
  try { accept(await apiGet(endpoint, {}, { signal: abort.signal })) }
  catch (err) { if (err.name !== 'AbortError') error.value = err.message }
  finally { loading.value = false }
})
onUnmounted(() => abort.abort())
</script>
<template>
  <div class="chat-schedule">
    <div class="auto-task-row">
      <div class="auto-task-server"><strong>群服聊天</strong><span>TXT 文件</span></div>
      <div class="admin-schedule-control" :class="{ 'is-enabled': schedule.enabled }">
        <label class="toggle-label admin-schedule-toggle"><input v-model="schedule.enabled" type="checkbox" role="switch" :disabled="props.disabled || loading || saving" /><span>自动同步 TXT</span></label>
        <label class="time-field"><span>同步时间</span><input v-model="schedule.time" type="time" required :disabled="!schedule.enabled || props.disabled || loading || saving" /></label>
      </div>
    </div>
    <p v-if="error" class="notice error" role="alert">{{ error }}</p>
    <p class="settings-hint">每日按系统时区执行；需先启用并保存群服聊天的 SMB 目录。</p>
  </div>
</template>
<style scoped>
.chat-schedule { margin-top: 12px; }
.chat-schedule .settings-hint { margin: 10px 0 0; }
</style>
