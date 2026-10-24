<script setup>
import { onMounted, onUnmounted, ref } from 'vue'
import { RefreshCw, Trash2 } from 'lucide-vue-next'
import { apiDelete, apiGet } from '../../api.js'
const rows = ref([]), loading = ref(false), clearing = ref(false), error = ref('')
const abort = new AbortController()
async function load() {
  if (loading.value || clearing.value) return
  loading.value = true; error.value = ''
  try { rows.value = await apiGet('/api/import/auto-task-logs', { limit: 80 }, { signal: abort.signal }) }
  catch (err) { if (err.name !== 'AbortError') error.value = err.message }
  finally { loading.value = false }
}
async function clear() {
  if (clearing.value || loading.value || !window.confirm('确定清空自动任务日志吗？')) return
  clearing.value = true; error.value = ''
  try { await apiDelete('/api/import/auto-task-logs', {}, { signal: abort.signal }); rows.value = [] }
  catch (err) { if (err.name !== 'AbortError') error.value = err.message }
  finally { clearing.value = false }
}
const label = status => ({ FINISHED: '成功', SUCCESS: '成功', FAILED: '失败', RUNNING: '进行中', PARTIAL: '部分成功', FINISHED_WITH_ERRORS: '完成有失败' }[status] || status)
onMounted(load); onUnmounted(() => abort.abort())
</script>
<template>
  <article class="panel settings-log-panel">
    <div class="settings-card-heading"><h3>自动任务记录 <span class="settings-log-count">{{ rows.length }}</span></h3><div class="settings-inline-actions"><button type="button" class="secondary-button" :disabled="loading || clearing" @click="load"><RefreshCw :size="15" :class="{ spin: loading }" />刷新</button><button type="button" class="table-action danger" :disabled="loading || clearing || !rows.length" @click="clear"><Trash2 :size="15" />清空</button></div></div>
    <p v-if="error" class="notice error" role="alert">{{ error }}</p>
    <div class="table-wrap"><table><thead><tr><th>时间</th><th>任务</th><th>状态</th><th>扫描</th><th>成功</th><th>跳过</th><th>失败</th><th>文件</th><th>日志</th></tr></thead><tbody><tr v-for="row in rows" :key="row.id"><td class="settings-nowrap">{{ new Date(row.createdAt).toLocaleString('zh-CN', { timeZone: 'Asia/Shanghai' }) }}</td><td>{{ row.taskLabel || row.serverName }}</td><td><span class="settings-log-status" :class="{ failed: ['FAILED', 'FINISHED_WITH_ERRORS'].includes(row.status) }">{{ label(row.status) }}</span></td><td>{{ row.scannedFiles }}</td><td>{{ row.successFiles }}</td><td>{{ row.skippedFiles }}</td><td>{{ row.failedFiles }}</td><td class="settings-log-detail" :title="row.fileDetails">{{ row.fileDetails || '—' }}</td><td class="settings-log-detail" :title="row.message">{{ row.message || '—' }}</td></tr><tr v-if="!rows.length"><td colspan="9" class="empty">{{ loading ? '正在加载…' : '暂无任务记录' }}</td></tr></tbody></table></div>
  </article>
</template>
