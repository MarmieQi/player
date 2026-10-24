<script setup>
import { onUnmounted, reactive, ref } from 'vue'
import { RefreshCw, Server } from 'lucide-vue-next'
import SettingsSecretField from './SettingsSecretField.vue'
import { apiGet, apiPost } from '../../api.js'
import SmbDirectorySettings from '../SmbDirectorySettings.vue'
const props = defineProps({ model: Object, dirty: Boolean, saving: Boolean })
const testing = ref(false), message = ref(''), error = ref('')
const previews = reactive({})
const abort = new AbortController()
async function test() {
  if (testing.value || props.dirty || props.saving) return
  testing.value = true; message.value = ''; error.value = ''
  try { const response = await apiPost('/api/admin/settings/smb/test', {}, { signal: abort.signal }); if (!response.success) throw new Error(response.message); message.value = response.message }
  catch (err) { if (err.name !== 'AbortError') error.value = err.message }
  finally { testing.value = false }
}
async function preview(id) {
  if (props.dirty || props.saving || previews[id]?.loading) return
  previews[id] = { loading: true, files: [], listed: false }
  error.value = ''
  try {
    const response = id === 'chat' ? await apiGet('/api/admin/settings/smb/chat-files', {}, { signal: abort.signal }) : await apiGet('/api/admin/settings/smb/files', { sourceId: id }, { signal: abort.signal })
    if (id !== 'chat' && !response.success) throw new Error(response.message)
    previews[id] = { loading: false, files: id === 'chat' ? response : response.files, listed: true }
  } catch (err) { if (err.name !== 'AbortError') error.value = err.message; previews[id].loading = false }
}
onUnmounted(() => abort.abort())
</script>
<template>
  <div class="settings-stack">
    <article class="panel settings-connection">
      <div class="settings-card-heading"><div><h3><Server :size="18" />SMB 连接</h3><p>CSV 与 TXT 共用连接，只读取远程文件。</p></div><button type="button" class="secondary-button" :disabled="dirty || saving || testing" :title="dirty ? '请先保存后测试连接' : '测试已保存的连接'" @click="test"><RefreshCw :size="15" :class="{ spin: testing }" />{{ testing ? '测试中…' : '测试连接' }}</button></div>
      <div class="settings-fields settings-connection-fields">
        <label class="settings-host"><span>主机地址</span><input v-model="model.host" type="text" placeholder="例: 192.168.1.100" maxlength="255" /></label>
        <label><span>端口</span><input v-model.number="model.port" type="number" min="1" max="65535" /></label>
        <label><span>共享名</span><input v-model="model.share" type="text" placeholder="例: logs" maxlength="255" /></label>
        <label><span>用户名</span><input v-model="model.username" type="text" autocomplete="off" maxlength="255" /></label>
        <SettingsSecretField v-model="model.password" label="密码" placeholder="留空则不修改" autocomplete="new-password" />
        <label><span>域</span><input v-model="model.domain" type="text" placeholder="可留空" maxlength="255" /></label>
      </div>
      <p v-if="message" class="notice success" role="status">{{ message }}</p>
    </article>
    <p v-if="error" class="notice error" role="alert">{{ error }}</p>
    <div class="settings-group-caption"><h3>日志目录</h3><span>共享内的相对路径</span></div>
    <SmbDirectorySettings v-for="source in model.sources" :key="source.sourceId" :title="`${source.sourceName} · CSV`" v-model:enabled="source.enabled" v-model:directory="source.smbDirectory" v-model:file-glob="source.smbFileGlob" directory-placeholder="例: logs\main" glob-placeholder="例: player_actions_*.csv" :disabled="saving" :refresh-disabled="dirty" :refreshing="previews[source.sourceId]?.loading" :files="previews[source.sourceId]?.files || []" :listed="previews[source.sourceId]?.listed" @refresh="preview(source.sourceId)" />
    <SmbDirectorySettings title="群服聊天 · TXT" v-model:enabled="model.chat.enabled" v-model:directory="model.chat.smbDirectory" v-model:file-glob="model.chat.fileGlob" directory-placeholder="例: logs\chat" glob-placeholder="例: *.txt" :disabled="saving" :refresh-disabled="dirty || !model.chat.smbDirectory" :refreshing="previews.chat?.loading" :files="previews.chat?.files || []" :listed="previews.chat?.listed" @refresh="preview('chat')" />
  </div>
</template>
