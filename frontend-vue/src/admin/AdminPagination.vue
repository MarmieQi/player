<script setup>
import { ChevronFirst, ChevronLast, ChevronLeft, ChevronRight } from 'lucide-vue-next'
import AdminSelect from './AdminSelect.vue'
import { DEFAULT_PAGE_SIZE, PAGE_SIZE_OPTIONS } from './pagination.js'

defineProps({
  page: { type: Number, default: 1 },
  totalPages: { type: Number, default: 1 },
  pageSize: { type: Number, default: DEFAULT_PAGE_SIZE },
  loading: Boolean
})
const emit = defineEmits(['page', 'page-size'])
const sizeOptions = PAGE_SIZE_OPTIONS.map(value => ({ value, label: String(value) }))
</script>

<template>
  <div class="admin-pagination" role="group" aria-label="日志分页">
    <AdminSelect :model-value="pageSize" :options="sizeOptions" label="每页条数" caption="每页" :disabled="loading" @update:model-value="emit('page-size', $event)" />
    <span class="page-count">{{ page }} / {{ totalPages }}</span>
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
.page-count { min-width: 52px; text-align: center; font-size: 16px; line-height: 24px; font-variant-numeric: tabular-nums; white-space: nowrap; }
.admin-pagination > button { display: inline-flex; align-items: center; justify-content: center; min-width: 40px; min-height: 44px; padding: 10px; }
</style>
