<script setup>
import { ArrowDown, ArrowUp, ChevronFirst, ChevronLast, ChevronLeft, ChevronRight } from 'lucide-vue-next'
import AdminSelect from './AdminSelect.vue'
import { DEFAULT_PAGE_SIZE, PAGE_SIZE_OPTIONS } from './pagination.js'

defineProps({
  page: { type: Number, default: 1 },
  totalPages: { type: Number, default: 1 },
  pageSize: { type: Number, default: DEFAULT_PAGE_SIZE },
  loading: Boolean,
  sortOrder: { type: String, default: 'asc' }
})
const emit = defineEmits(['page', 'page-size', 'order'])
const sizeOptions = PAGE_SIZE_OPTIONS.map(value => ({ value, label: String(value) }))
</script>

<template>
  <div class="admin-pagination" role="group" aria-label="日志分页">
    <AdminSelect :model-value="pageSize" :options="sizeOptions" label="每页条数" caption="每页" :disabled="loading" @update:model-value="emit('page-size', $event)" />
    <span class="page-count" :aria-label="`第 ${page} 页，共 ${totalPages} 页`"><span class="current-page">{{ page }}</span><span class="page-divider" aria-hidden="true">/</span><span class="total-pages">{{ totalPages }}</span></span>
    <button type="button" class="secondary-button sort-toggle" :class="{ 'is-descending': sortOrder === 'desc' }" :disabled="loading" :aria-label="sortOrder === 'desc' ? '时间排序：早的在后' : '时间排序：早的在前'" :aria-pressed="sortOrder === 'desc'" :title="sortOrder === 'desc' ? '早的在后，点击切换为早的在前' : '早的在前，点击切换为早的在后'" @click="emit('order', sortOrder === 'asc' ? 'desc' : 'asc')"><component :is="sortOrder === 'desc' ? ArrowDown : ArrowUp" :size="18" /></button>
    <button type="button" class="secondary-button" aria-label="第一页" title="第一页" :disabled="loading || page <= 1" @click="emit('page', 1)"><ChevronFirst :size="17" /></button>
    <button type="button" class="secondary-button" aria-label="上一页" title="上一页" :disabled="loading || page <= 1" @click="emit('page', page - 1)"><ChevronLeft :size="17" /></button>
    <button type="button" class="secondary-button" aria-label="下一页" title="下一页" :disabled="loading || page >= totalPages" @click="emit('page', page + 1)"><ChevronRight :size="17" /></button>
    <button type="button" class="secondary-button" aria-label="最后一页" title="最后一页" :disabled="loading || page >= totalPages" @click="emit('page', totalPages)"><ChevronLast :size="17" /></button>
  </div>
</template>

<style scoped>
.admin-pagination { display: flex; align-items: center; flex-wrap: wrap; gap: 10px; color: #526479; }
.admin-pagination :deep(.admin-select-root) { width: auto; min-width: 0; }
.admin-pagination :deep(.admin-select-trigger) { width: auto; min-height: 44px; padding: 0 14px; gap: 14px; }
.admin-pagination :deep(.admin-select-caption) { padding-right: 12px; border-right: 1px solid #e5eaf0; color: #526479; font: inherit; font-size: 14px; font-weight: 400; line-height: 20px; }
.page-count { display: inline-flex; align-items: center; justify-content: center; gap: 8px; box-sizing: border-box; min-width: 68px; height: 44px; padding: 0 12px; border: 1px solid #d9e2eb; border-radius: 8px; background: #fff; font-size: 16px; line-height: 24px; font-variant-numeric: tabular-nums; white-space: nowrap; }
.current-page { color: #526479; font-weight: 400; }
.page-divider { color: #a1adbd; font-size: 13px; }
.total-pages { color: #526479; }
.admin-pagination > button { display: inline-flex; align-items: center; justify-content: center; min-width: 40px; min-height: 44px; padding: 10px; }
.admin-pagination > .sort-toggle { margin-right: 8px; }
.admin-pagination > .sort-toggle.is-descending { color: var(--admin-accent, #465fc9); border-color: var(--admin-accent, #465fc9); background: var(--admin-accent-soft, #edf1ff); }
</style>
