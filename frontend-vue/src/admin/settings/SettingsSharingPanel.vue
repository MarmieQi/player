<script setup>
import { onMounted, onUnmounted, ref } from 'vue'
import { Copy, RefreshCw } from 'lucide-vue-next'
import SettingsSecretField from './SettingsSecretField.vue'
import { apiGet, apiPost } from '../../api.js'
defineProps({ model: Object })
const key = ref(''), loading = ref(true), resetting = ref(false), error = ref(''), message = ref('')
const abort = new AbortController()
async function load() {
  loading.value = true; error.value = ''
  try { key.value = (await apiGet('/api/config/astrbot-key', {}, { signal: abort.signal })).apiKey }
  catch (err) { if (err.name !== 'AbortError') error.value = err.message }
  finally { loading.value = false }
}
async function reset() {
  if (resetting.value || !window.confirm('重置后旧插件密钥立即失效，确定重置吗？')) return
  resetting.value = true; error.value = ''; message.value = ''
  try { key.value = (await apiPost('/api/config/astrbot-key/reset', {}, { signal: abort.signal })).apiKey; message.value = '密钥已重置，请更新插件配置' }
  catch (err) { if (err.name !== 'AbortError') error.value = err.message }
  finally { resetting.value = false }
}
async function copy() { try { await navigator.clipboard.writeText(key.value); message.value = '密钥已复制' } catch { error.value = '复制失败，请显示密钥后手动复制' } }
onMounted(load); onUnmounted(() => abort.abort())
</script>
<template>
  <div class="settings-stack">
    <article class="panel"><div class="settings-card-heading"><h3>分享链接</h3></div><div class="settings-fields">
      <label><span>通用链接有效期（分钟）</span><input v-model.number="model.shareTtlMinutes" type="number" min="5" max="10080" step="1" /><small>玩家信息与排行榜</small></label>
      <label><span>查日志链接有效期（分钟）</span><input v-model.number="model.logQueryShareTtlMinutes" type="number" min="5" max="10080" step="1" /><small>服务器日志查询结果</small></label>
    </div></article>
    <article class="panel"><div class="settings-card-heading"><div><h3>AstrBot 插件</h3><p>插件密钥用于访问玩家统计接口。</p></div></div>
      <p v-if="error" class="notice error" role="alert">{{ error }}<button v-if="!key" type="button" class="table-action" @click="load">重新加载</button></p><p v-if="message" class="notice success" role="status">{{ message }}</p>
      <SettingsSecretField label="插件密钥" :model-value="key" readonly :placeholder="loading ? '加载中…' : ''" />
      <div class="settings-inline-actions"><button type="button" class="secondary-button" :disabled="!key || loading || resetting" @click="copy"><Copy :size="15" />复制密钥</button><button type="button" class="table-action danger" :disabled="loading || resetting" @click="reset"><RefreshCw :size="15" />{{ resetting ? '重置中…' : '重置密钥' }}</button></div>
    </article>
  </div>
</template>
