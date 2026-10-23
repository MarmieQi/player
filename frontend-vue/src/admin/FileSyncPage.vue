<script setup>
import { computed, ref, watch } from 'vue'
import { RefreshCw, Play, FileText, CheckCircle2, CircleSlash, XCircle, Trash2 } from 'lucide-vue-next'
import { fileNameFromPath, fileSize, fileCompareKey, localFileKey, mergeSyncFiles, compareLocalCopy, syncStatusClass, syncStatusLabel } from './file-sync-model.js'
const props = defineProps({ extension: { type: String, required: true }, scopeName: String, remoteFiles: { type: Array, default: () => [] }, localFiles: { type: Array, default: () => [] }, job: Object, loading: Boolean, starting: Boolean, deleting: Boolean, available: { type: Boolean, default: true }, skipToday: Boolean, error: String, message: String })
const emit = defineEmits(['refresh', 'start', 'delete', 'update:skipToday'])
const running = computed(() => ['PENDING', 'RUNNING'].includes(props.job?.status))
const rows = computed(() => mergeSyncFiles(props.remoteFiles, props.job?.files || [], props.localFiles))
const localMap = computed(() => new Map(props.localFiles.map(file => [fileCompareKey(file), file])))
const selected = ref(new Set())
const localFor = file => localMap.value.get(fileCompareKey(file))
const keyFor = file => localFor(file) ? localFileKey(localFor(file)) : ''
const eligible = computed(() => rows.value.filter(file => keyFor(file)))
const selectedFiles = computed(() => eligible.value.filter(file => selected.value.has(keyFor(file))).map(localFor))
const allSelected = computed(() => eligible.value.length > 0 && selectedFiles.value.length === eligible.value.length)
function select(file, checked) { const key = keyFor(file); const next = new Set(selected.value); if (checked) next.add(key); else next.delete(key); selected.value = next }
function selectAll(checked) { selected.value = checked ? new Set(eligible.value.map(keyFor)) : new Set() }
watch(() => props.localFiles, () => { selected.value = new Set() })
const metrics = computed(() => [
  { label: '扫描文件', value: props.job?.scannedFiles || 0, icon: FileText, color: 'blue' },
  { label: '已同步', value: props.job?.importedFiles || 0, icon: CheckCircle2, color: 'green' },
  { label: '跳过', value: props.job?.skippedFiles || 0, icon: CircleSlash, color: 'amber' },
  { label: '失败', value: props.job?.failedFiles || 0, icon: XCircle, color: 'red' }
])
const info = file => compareLocalCopy(file, localFor(file))
function fileMessage(file) { return file.status === 'RUNNING' ? '正在同步文件' : file.message || '-' }
</script>
<template>
  <section class="result-page">
    <div class="topbar"><div class="actions">
      <slot name="scope" />
      <button class="icon-button ghost" type="button" title="刷新文件对比" :disabled="loading || running || deleting" @click="emit('refresh')"><RefreshCw :size="18" :class="{ spin: loading }" /></button>
      <label class="toggle-label"><input :checked="skipToday" type="checkbox" :disabled="running || starting" @change="emit('update:skipToday', $event.target.checked)" /><span>跳过当天文件</span></label>
      <button class="primary-button" type="button" :disabled="!available || loading || starting || running || deleting" @click="emit('start')"><Play :size="18" /><span>{{ running ? '同步中' : `开始同步 ${scopeName || extension}` }}</span></button>
    </div></div>
    <p v-if="error" class="notice error" role="alert">{{ error }}</p>
    <p v-if="message" class="notice success" role="status">{{ message }}</p>
    <slot name="notice" />
    <section class="result-summary-grid"><article v-for="metric in metrics" :key="metric.label" class="metric-card compact"><div class="metric-icon" :class="metric.color"><component :is="metric.icon" :size="20" /></div><p>{{ metric.label }}</p><strong>{{ metric.value.toLocaleString('zh-CN') }}</strong></article></section>
    <article class="panel wide">
      <div class="panel-title"><h2>文件同步情况</h2><span>{{ syncStatusLabel(job?.status || 'READY') }}</span></div>
      <div class="bulk-actions">
        <label class="bulk-check"><input type="checkbox" :checked="allSelected" :disabled="running || starting || deleting || !eligible.length" @change="selectAll($event.target.checked)" /><span>选择已在本地</span></label>
        <button class="table-action danger" type="button" :disabled="running || starting || deleting || !selectedFiles.length" @click="emit('delete', selectedFiles)"><Trash2 :size="15" /><span>{{ deleting ? '删除中' : `删除本地 ${extension} ${selectedFiles.length}` }}</span></button>
      </div>
      <div class="table-wrap"><table><thead><tr><th>选择</th><th>文件</th><th>服务器</th><th>状态</th><th>远程大小</th><th>本地对比</th><th>本地大小</th><th>说明</th></tr></thead><tbody>
        <tr v-for="file in rows" :key="fileCompareKey(file)">
          <td><input type="checkbox" :checked="selected.has(keyFor(file))" :disabled="running || starting || deleting || !localFor(file)" @change="select(file, $event.target.checked)" /></td>
          <td class="path-cell" :title="file.remotePath">{{ file.fileName || fileNameFromPath(file.remotePath) }}</td><td>{{ file.serverName }}</td>
          <td><span class="status-pill" :class="syncStatusClass(file.status)">{{ file.localOnly ? '仅本地' : syncStatusLabel(file.status) }}</span></td>
          <td>{{ file.localOnly ? '-' : fileSize(file.fileSize) }}</td><td><span class="status-pill" :class="info(file).className" :title="info(file).message">{{ info(file).label }}</span></td><td>{{ info(file).fileSize === null ? '-' : fileSize(info(file).fileSize) }}</td><td class="message-cell">{{ fileMessage(file) }}</td>
        </tr>
        <tr v-if="!rows.length"><td colspan="8" class="empty">{{ loading ? '正在加载文件对比...' : running ? `正在同步 ${extension} 文件` : '未找到远程文件，请检查 SMB 设置' }}</td></tr>
      </tbody></table></div>
    </article>
  </section>
</template>
