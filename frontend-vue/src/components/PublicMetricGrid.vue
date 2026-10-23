<script setup>
import { Blocks, Clock, Database, Eye, FileText, Hammer, Layers, Pickaxe } from 'lucide-vue-next'

defineProps({ items: { type: Array, required: true } })
const icons = { blocks: Blocks, clock: Clock, database: Database, ratio: Eye, evidence: FileText, broken: Hammer, underground: Layers, ore: Pickaxe }
</script>

<template>
  <div class="share-v2-stat-grid">
    <article v-for="item in items" :key="item.id" class="share-v2-stat-card" :class="`is-${item.tone}`">
      <div class="share-v2-stat-icon"><component :is="icons[item.icon]" :size="19" /></div>
      <div><span>{{ item.label }}</span><strong>{{ item.value }}</strong></div>
    </article>
  </div>
</template>
