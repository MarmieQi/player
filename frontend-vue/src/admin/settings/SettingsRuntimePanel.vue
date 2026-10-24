<script setup>
import { runtimeFields } from '../settings-model.js'
defineProps({ model: Object, response: Object })
</script>
<template>
  <article class="panel settings-runtime-panel">
    <div class="settings-card-heading"><div><h3>运行调优</h3><p>保存后重启后端生效。</p></div><span class="settings-state" :class="{ pending: response.restartRequired }">{{ response.restartRequired ? '待重启' : '已生效' }}</span></div>
    <div class="settings-runtime-columns"><span>参数</span><span>已保存值 / 编辑</span><span>当前运行值</span></div>
    <div v-for="field in runtimeFields" :key="field.key" class="settings-runtime-row">
      <div><label :for="`runtime-${field.key}`">{{ field.label }}</label><p>{{ field.hint }}</p></div>
      <div class="settings-number-with-unit"><input :id="`runtime-${field.key}`" v-model.number="model[field.key]" type="number" :min="field.min" :max="field.max" step="1" /><span>{{ field.unit }}</span></div>
      <div class="settings-active-value"><small>当前运行</small><strong>{{ response.active[field.key] }}</strong><span>{{ field.unit }}</span></div>
    </div>
    <p class="settings-footnote">重启命令：<code>docker compose restart backend</code></p>
  </article>
</template>
