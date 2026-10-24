<script setup>
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { Search, X } from 'lucide-vue-next'
import { apiGet, apiPostJson } from '../api.js'
import PlayerNameAutocomplete from './PlayerNameAutocomplete.vue'
import AdminPagination from './AdminPagination.vue'
import { DEFAULT_PAGE_SIZE } from './pagination.js'
import { serverDateAt } from './server-date-clock.js'

const endpoint = '/api/admin/group-chat/query'
const today = ref('')
const fromDate = ref('')
const toDate = ref('')
const player = ref('')
const keyword = ref('')
const result = ref(null)
const searched = ref(null)
const loading = ref(false)
const initializing = ref(true)
const error = ref('')
const pageSize = ref(DEFAULT_PAGE_SIZE)
const sortOrder = ref('asc')
const pages = computed(() => Math.max(1, Math.ceil((result.value?.total || 0) / (result.value?.pageSize || DEFAULT_PAGE_SIZE))))
let abort
let version = 0
let disposed = false
let dateTimer
let serverClock = ''
let clockReceivedAt = 0
const lifecycle = new AbortController()

function updateToday() { if (serverClock) today.value = serverDateAt(serverClock, clockReceivedAt) }

function stop() { ++version; abort?.abort(); loading.value = false }
function clear() {
	updateToday()
  stop(); result.value = null; searched.value = null; error.value = ''
  fromDate.value = today.value; toDate.value = today.value; player.value = ''; keyword.value = ''; pageSize.value = DEFAULT_PAGE_SIZE; sortOrder.value = 'asc'
}
async function run(operation, criteria) {
  stop(); const current = version; abort = new AbortController(); loading.value = true; error.value = ''
  if (criteria) { result.value = null; searched.value = criteria }
  try {
    const value = await operation(abort.signal)
    if (disposed || current !== version) return
    result.value = value
    pageSize.value = value.pageSize
    sortOrder.value = value.sortOrder || 'asc'
    if (criteria) searched.value = criteria
  } catch (err) {
    if (!disposed && current === version && err.name !== 'AbortError') {
      error.value = err.message
      if (err.status === 410) result.value = null
    }
  } finally { if (current === version) loading.value = false }
}
async function search() {
	updateToday()
  if (!fromDate.value || !toDate.value || fromDate.value > toDate.value || toDate.value > today.value) {
    error.value = '请选择有效日期，结束日期不能晚于今天'; return
  }
  const criteria = { fromDate: fromDate.value, toDate: toDate.value, player: player.value.trim(), keyword: keyword.value.trim() }
  await run(signal => apiPostJson(endpoint, { ...criteria, sortOrder: sortOrder.value }, { signal }), criteria)
}
async function page(number, overrides = {}) {
  if (!result.value) return
  const id = result.value.snapshotId
  await run(signal => apiGet(`${endpoint}/${encodeURIComponent(id)}`, { page: number, pageSize: pageSize.value, sortOrder: sortOrder.value, ...overrides }, { signal }))
}
function changePageSize(value) {
  return page(1, { pageSize: value })
}
onMounted(async () => {
  try {
    const clock = await apiGet('/api/stats/server-time', {}, { signal: lifecycle.signal })
    if (disposed) return
    serverClock = clock.serverTime; clockReceivedAt = Date.now()
    updateToday()
    dateTimer = setInterval(updateToday, 60000)
    fromDate.value = today.value; toDate.value = today.value
    await search()
  } catch (err) { if (!disposed && err.name !== 'AbortError') error.value = err.message }
  finally { initializing.value = false }
})
onUnmounted(() => { disposed = true; stop(); lifecycle.abort(); clearInterval(dateTimer) })
</script>

<template>
  <section class="group-chat-page">
    <form class="panel chat-search" @submit.prevent="search">
      <div class="chat-filter-grid">
        <label><span>开始日期</span><input v-model="fromDate" type="date" required :max="today" :disabled="initializing" /></label>
        <label><span>结束日期</span><input v-model="toDate" type="date" required :min="fromDate" :max="today" :disabled="initializing" /></label>
        <PlayerNameAutocomplete v-model="player" server-id="all" />
        <label><span>关键字</span><input v-model="keyword" type="search" maxlength="256" placeholder="详细信息关键字" /></label>
      </div>
      <div class="chat-search-actions">
        <button class="primary-button" type="submit" :disabled="loading || initializing"><Search :size="17" /><span>{{ loading ? '查询中…' : '查询聊天' }}</span></button>
        <button v-if="loading" class="secondary-button" type="button" @click="stop"><X :size="16" />取消查询</button>
        <button v-else class="secondary-button" type="button" :disabled="initializing" @click="clear">清空</button>
      </div>
    </form>

    <p v-if="error" class="notice error" role="alert">{{ error }}</p>
    <p v-if="result?.missingDates?.length" class="notice" role="status">以下日期没有可查询的本地文件或当天远程文件：{{ result.missingDates.join('、') }}。历史文件请先到 TXT 同步页同步。</p>

    <article class="panel chat-results" :aria-busy="loading">
      <div class="chat-result-heading">
        <div><h2>聊天原文</h2><p v-if="result" class="chat-hint">{{ searched?.fromDate }} 至 {{ searched?.toDate }} · 共 {{ result.total }} 条</p></div>
        <AdminPagination v-if="result" :page="result.page" :total-pages="pages" :page-size="pageSize" :sort-order="sortOrder" :loading="loading" @page="page" @page-size="changePageSize" @order="page(1, { sortOrder: $event })" />
      </div>
      <div v-if="result?.rows?.length" class="chat-raw-lines" aria-label="聊天记录"><div v-for="(line, index) in result.rows" :key="`${result.snapshotId}-${result.page}-${index}`" class="chat-raw-line">{{ line }}</div></div>
      <p v-else class="chat-empty">{{ loading ? '正在获取文件并搜索完整原文…' : result ? '没有匹配的聊天记录' : '选择日期、玩家或关键词后查询' }}</p>
    </article>
  </section>
</template>

<style scoped>
.group-chat-page { display: grid; gap: 18px; min-width: 0; }
.chat-filter-grid { display: grid; grid-template-columns: repeat(2, minmax(150px, 1fr)) minmax(210px, 1.15fr) minmax(240px, 1.6fr); gap: 18px; align-items: end; }
.chat-filter-grid > label { display: grid; gap: 8px; color: #526479; font-size: 14px; }
.chat-filter-grid input, .chat-filter-grid :deep(.player-picker-input input) { box-sizing: border-box; width: 100%; min-height: 40px; padding: 9px 12px; border: 1px solid #d9e2eb; border-radius: 7px; background: #fbfcfe; color: #34485e; font: inherit; font-size: 14px; }
.chat-filter-grid :deep(.player-picker-input input) { padding-right: 34px; }
.chat-filter-grid :deep(.player-name-autocomplete) { gap: 8px; }
.chat-filter-grid :deep(.player-name-autocomplete label) { font-size: 14px; font-weight: 400; color: #526479; }
.chat-search-actions { display: flex; align-items: center; flex-wrap: wrap; gap: 12px; margin-top: 20px; }
.chat-hint { color: #7a889b; font-size: 13px; font-weight: 400; margin: 6px 0 0; }
.chat-result-heading { display: flex; align-items: center; justify-content: space-between; gap: 16px; flex-wrap: wrap; padding-bottom: 18px; }
.chat-result-heading h2 { margin: 0; font-size: 18px; }
.chat-raw-lines { border-top: 1px solid #e6ebf2; }
.chat-raw-line { white-space: pre-wrap; overflow-wrap: anywhere; font-family: ui-monospace, 'Cascadia Code', Consolas, monospace; font-size: 14px; line-height: 1.85; color: #34445b; padding: 9px 12px; }
.chat-empty { text-align: center; color: #7a889b; padding: 44px 12px; }
@media (max-width: 1100px) { .chat-filter-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); } }
@media (max-width: 600px) { .chat-filter-grid { grid-template-columns: minmax(0, 1fr); gap: 14px; } .chat-raw-line { font-size: 13px; padding: 9px 0; } }
</style>
