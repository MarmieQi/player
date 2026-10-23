<script setup>
import { Crosshair, Search } from 'lucide-vue-next'
import { LOG_QUERY_TYPE_COORDINATE, LOG_QUERY_TYPE_PLAYER_KEYWORD } from './log-feature-navigation'

defineProps({ mode: { type: String, required: true } })
defineEmits(['change'])
const options = [
  { id: LOG_QUERY_TYPE_COORDINATE, label: '通过坐标查询', icon: Crosshair },
  { id: LOG_QUERY_TYPE_PLAYER_KEYWORD, label: '综合筛选', icon: Search }
]
</script>

<template>
  <nav class="log-feature-tabs" aria-label="日志查询方式">
    <button v-for="option in options" :key="option.id" type="button" :class="{ active: mode === option.id }" :aria-current="mode === option.id ? 'page' : undefined" @click="$emit('change', option.id)">
      <component :is="option.icon" :size="16" /><span>{{ option.label }}</span>
    </button>
  </nav>
</template>
