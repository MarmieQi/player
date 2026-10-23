<script setup>
import { RefreshCw, ChevronDown } from 'lucide-vue-next'
defineProps({ title: String, enabled: Boolean, directory: String, fileGlob: String, directoryPlaceholder: String, globPlaceholder: String, disabled: Boolean, refreshing: Boolean, refreshDisabled: Boolean, files: { type: Array, default: () => [] }, listed: Boolean })
defineEmits(['update:enabled', 'update:directory', 'update:fileGlob', 'refresh'])
function size(value) {
  const bytes = Number(value) || 0
  if (bytes < 1024) return `${bytes} B`
  return bytes < 1048576 ? `${(bytes / 1024).toFixed(1)} KB` : `${(bytes / 1048576).toFixed(1)} MB`
}
</script>
<template>
  <article class="panel wide admin-source-panel">
    <div class="panel-title"><h2>{{ title }}</h2><label class="toggle-label"><input :checked="enabled" type="checkbox" :disabled="disabled" @change="$emit('update:enabled', $event.target.checked)" /><span>启用</span></label></div>
    <slot name="notice" />
    <div class="settings-form">
      <label><span>目录路径</span><input :value="directory" type="text" :placeholder="directoryPlaceholder" :disabled="disabled" @input="$emit('update:directory', $event.target.value)" /></label>
      <label><span>文件匹配</span><input :value="fileGlob" type="text" :placeholder="globPlaceholder" :disabled="disabled" @input="$emit('update:fileGlob', $event.target.value)" /></label>
      <div class="settings-actions"><button type="button" class="secondary-button" :title="refreshDisabled ? '请先保存配置后刷新' : '读取已保存的目录配置'" :disabled="disabled || refreshing || refreshDisabled" @click="$emit('refresh')"><RefreshCw :size="16" :class="{ spin: refreshing }" /><span>{{ refreshing ? '加载中...' : '刷新文件列表' }}</span></button></div>
    </div>
    <details class="admin-source-details">
      <summary class="admin-source-file-summary"><ChevronDown class="admin-source-chevron" :size="16" /><span>文件列表</span><small v-if="listed">{{ files.length }} 个文件</small></summary>
      <div v-if="files.length" class="source-file-list"><table class="data-table compact"><thead><tr><th>文件名</th><th>大小</th><th>修改时间</th></tr></thead><tbody>
        <tr v-for="file in files" :key="file.remotePath || file.path || file.fileName"><td class="path-cell">{{ file.fileName }}</td><td>{{ size(file.fileSize ?? file.size) }}</td><td>{{ file.lastModified ? new Date(file.lastModified).toLocaleString('zh-CN') : '-' }}</td></tr>
      </tbody></table></div>
      <p v-else class="settings-hint">{{ refreshing ? '正在读取文件列表' : listed ? '该目录下没有文件，请检查目录路径设置' : '点击刷新文件列表查看远程文件' }}</p>
    </details>
  </article>
</template>
