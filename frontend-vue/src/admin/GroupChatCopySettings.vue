<script setup>
import { computed, onMounted, onUnmounted, ref } from 'vue'
import SmbDirectorySettings from './SmbDirectorySettings.vue'
const props = defineProps({ disabled: Boolean, previewDisabled: Boolean })
import { apiGet, apiPutJson } from '../api.js'
const endpoint = '/api/admin/group-chat/config'
const config = ref({ enabled: false, smbDirectory: '', fileGlob: '*.txt' })
const saved = ref('')
const loading = ref(true)
const saving = ref(false)
const error = ref('')
const files = ref([])
const listed = ref(false)
const refreshing = ref(false)
const dirty = computed(() => JSON.stringify(config.value) !== saved.value)
const ready = computed(() => !loading.value && Boolean(saved.value))
const abort = new AbortController()
function accept(value) { config.value = value; saved.value = JSON.stringify(value) }
async function save() {
  if (!ready.value) throw new Error('TXT 目录配置尚未加载成功')
  if (!dirty.value) return
  saving.value = true; error.value = ''
  try { accept(await apiPutJson(endpoint, config.value, { signal: abort.signal })); files.value = []; listed.value = false }
  catch (err) { error.value = err.message; throw err }
  finally { saving.value = false }
}
async function refresh() {
  refreshing.value = true; error.value = ''
  try { files.value = await apiGet('/api/admin/group-chat/directory-files', {}, { signal: abort.signal }); listed.value = true }
  catch (err) { if (err.name !== 'AbortError') error.value = err.message }
  finally { refreshing.value = false }
}
function validate() {
  if (!ready.value) throw new Error('TXT 目录配置尚未加载成功')
  if (config.value.enabled && !config.value.smbDirectory.trim()) throw new Error('请填写群服聊天的目录路径')
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
  <SmbDirectorySettings title="群服聊天 - SMB 目录" v-model:enabled="config.enabled" v-model:directory="config.smbDirectory" v-model:file-glob="config.fileGlob" directory-placeholder="例: logs\chat" glob-placeholder="例: *.txt" :disabled="props.disabled || loading || saving" :refreshing="refreshing" :refresh-disabled="!ready || dirty || props.previewDisabled || !config.smbDirectory" :files="files" :listed="listed" @refresh="refresh">
    <template #notice><p v-if="error" class="notice error" role="alert">{{ error }}</p></template>
  </SmbDirectorySettings>
</template>
