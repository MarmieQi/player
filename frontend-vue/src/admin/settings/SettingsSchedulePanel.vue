<script setup>
defineProps({ model: Object })
</script>
<template>
  <div class="settings-stack">
    <article class="panel settings-policy"><label class="toggle-label"><input v-model="model.skipToday" type="checkbox" /><strong>默认跳过当天文件</strong></label><p>CSV 自动同步、解析和 TXT 自动同步共用，默认开启。</p></article>
    <article class="panel settings-schedule-panel">
      <div class="settings-card-heading"><h3>每日执行计划</h3><span class="settings-subtle">北京时间</span></div>
      <div v-for="task in model.tasks" :key="task.serverId" class="settings-plan-row">
        <div class="settings-plan-name"><strong>{{ task.serverName }}</strong><span>CSV 日志</span></div>
        <div class="settings-plan-control"><label class="toggle-label"><input v-model="task.syncEnabled" type="checkbox" role="switch" />自动同步 CSV</label><input v-model="task.syncTime" type="time" :aria-label="`${task.serverName}同步时间`" :disabled="!task.syncEnabled" /></div>
        <div class="settings-plan-control"><label class="toggle-label"><input v-model="task.importEnabled" type="checkbox" role="switch" />自动解析入库</label><input v-model="task.importTime" type="time" :aria-label="`${task.serverName}解析时间`" :disabled="!task.importEnabled" /></div>
      </div>
      <div class="settings-plan-row"><div class="settings-plan-name"><strong>群服聊天</strong><span>TXT 日志</span></div><div class="settings-plan-control"><label class="toggle-label"><input v-model="model.chat.enabled" type="checkbox" role="switch" />自动同步 TXT</label><input v-model="model.chat.time" type="time" aria-label="群服聊天同步时间" :disabled="!model.chat.enabled" /></div></div>
      <p class="settings-footnote">CSV 同步与解析建议间隔 5～10 分钟；TXT 只同步文件。</p>
    </article>
  </div>
</template>
