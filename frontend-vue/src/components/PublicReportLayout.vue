<script setup>
import { Clock, Search } from 'lucide-vue-next'

defineProps({
  ready: Boolean,
  loading: Boolean,
  error: String,
  loadingTitle: { type: String, default: '正在打开日志' },
  loadingText: { type: String, default: '正在读取查询结果' },
  errorTitle: { type: String, default: '链接已过期' }
})
</script>

<template>
  <main class="log-share-page">
    <section v-if="loading" class="share-v2-state" aria-live="polite" aria-busy="true">
      <div class="share-v2-state-mark"><Search :size="24" /></div>
      <div><h1>{{ loadingTitle }}</h1><p>{{ loadingText }}</p></div>
    </section>
    <section v-else-if="error" class="share-v2-state share-v2-state-error" role="alert">
      <div class="share-v2-state-mark"><Clock :size="24" /></div>
      <div><h1>{{ errorTitle }}</h1><p>{{ error }}</p></div>
    </section>
    <template v-else-if="ready">
      <header class="log-share-hero"><slot name="header" /></header>
      <section class="log-share-summary"><slot name="summary" /></section>
      <slot />
      <footer class="log-share-footer"><slot name="footer" /></footer>
    </template>
  </main>
</template>
