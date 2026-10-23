<script setup>
import { computed, onMounted, onUnmounted, ref } from 'vue'
import FileSyncPage from './FileSyncPage.vue'
import { apiGet, apiPost, apiPostJson } from '../api.js'

defineEmits(['settings'])
const base = '/api/admin/group-chat'
const jobKey = 'player-stats-group-chat-copy-job'
const config = ref({ enabled: false, smbDirectory: '', fileGlob: '*.txt' })
const loading = ref(true)
const listing = ref(false)
const starting = ref(false)
const skipToday = ref(true)
const files = ref([])
const localFiles = ref([])
const deleting = ref(false)
const job = ref(null)
const error = ref('')
const message = ref('')
const running = computed(() => ['PENDING', 'RUNNING'].includes(job.value?.status))
const abort = new AbortController()
let timer
let disposed = false

function report(err) { if (!disposed && err.name !== 'AbortError') error.value = err.message }

async function refresh() {
  if (disposed || listing.value) return
  listing.value = true; error.value = ''
  try {
    const results = await Promise.allSettled([
      config.value.enabled ? apiGet(`${base}/files`, { skipToday: skipToday.value }, { signal: abort.signal }) : Promise.resolve([]),
      apiGet(`${base}/local-files`, {}, { signal: abort.signal })
    ])
    if (results[0].status === 'fulfilled') files.value = results[0].value
    else report(results[0].reason)
    if (results[1].status === 'fulfilled') localFiles.value = results[1].value
    else report(results[1].reason)
  } catch (err) { report(err) }
  finally { listing.value = false }
}
function poll(delay = 1000) {
  clearTimeout(timer)
  if (disposed || !running.value) return
  timer = setTimeout(async () => {
    try {
      job.value = await apiGet(`${base}/copy-jobs/${encodeURIComponent(job.value.jobId)}`, {}, { signal: abort.signal })
      if (running.value) poll()
      else { message.value = `同步结束：成功 ${job.value.importedFiles}，跳过 ${job.value.skippedFiles}，失败 ${job.value.failedFiles}`; await refresh() }
    } catch (err) {
      report(err)
      if (err.status === 404) {
        job.value = { ...job.value, status: 'FAILED', message: '同步任务已失效，请刷新列表后重新开始' }
        try { localStorage.removeItem(jobKey) } catch {}
      } else if (err.status !== 401) poll(5000)
    }
  }, delay)
}
async function start() {
  if (starting.value || running.value || !config.value.enabled) return
  starting.value = true; error.value = ''; message.value = ''
  try {
    job.value = await apiPost(`${base}/copy-jobs`, { skipToday: skipToday.value }, { signal: abort.signal })
    try { localStorage.setItem(jobKey, job.value.jobId) } catch {}
    poll()
  } catch (err) { report(err) }
  finally { starting.value = false }
}
async function deleteLocal(selectedFiles) {
  if (deleting.value || running.value || starting.value) return
  if (!selectedFiles.length) return
  if (!window.confirm(`只删除后端本地已同步的 TXT，不删除远程 SMB 文件。\n本次将删除 ${selectedFiles.length} 个本地 TXT。确定继续吗？`)) return
  deleting.value = true; error.value = ''; message.value = ''
  try {
    const results = await apiPostJson(`${base}/local-files/delete`, { files: selectedFiles.map(file => ({ remotePath: file.remotePath })) }, { signal: abort.signal })
    const count = results.filter(file => file.deleted).length
    message.value = `已删除 ${count} 个本地 TXT${count < results.length ? `，失败 ${results.length - count} 个` : ''}`
    await refresh()
  } catch (err) { report(err) }
  finally { deleting.value = false }
}
onMounted(async () => {
  try {
    config.value = await apiGet(`${base}/config`, {}, { signal: abort.signal })
    let id = ''
    try { id = localStorage.getItem(jobKey) || '' } catch {}
    if (id) {
      try { const restored = await apiGet(`${base}/copy-jobs/${encodeURIComponent(id)}`, {}, { signal: abort.signal }); if (restored.jobId) { job.value = restored; poll() } }
      catch (err) { if (err.status === 404) localStorage.removeItem(jobKey); else throw err }
    }
    await refresh()
  } catch (err) { report(err) }
  finally { loading.value = false }
})
onUnmounted(() => { disposed = true; clearTimeout(timer); abort.abort() })
</script>

<template>
  <FileSyncPage extension="TXT" scope-name="群服聊天" :remote-files="files" :local-files="localFiles" :job="job" :loading="loading || listing" :starting="starting" :deleting="deleting" :available="config.enabled" v-model:skip-today="skipToday" :error="error" :message="message" @refresh="refresh" @start="start" @delete="deleteLocal">
    <template #scope><span class="txt-sync-scope">群服聊天</span></template>
    <template #notice><p v-if="!loading && !config.enabled" class="notice">请先在系统设置中启用并保存群服聊天的 SMB 目录。<button type="button" class="table-action" @click="$emit('settings')">前往设置</button></p></template>
  </FileSyncPage>
</template>
<style scoped>
.txt-sync-scope { margin-right: auto; color: #526479; font-size: 14px; }
</style>
