<script setup>
import { ref, useId } from 'vue'
import { Eye, EyeOff } from 'lucide-vue-next'
defineProps({ label: String, modelValue: String, placeholder: String, readonly: Boolean, autocomplete: { type: String, default: 'off' } })
defineEmits(['update:modelValue'])
const visible = ref(false)
const inputId = useId()
</script>
<template>
  <div class="settings-secret-field">
    <label :for="inputId">{{ label }}</label>
    <div class="settings-secret-control">
      <input :id="inputId" :type="visible ? 'text' : 'password'" :value="modelValue" :placeholder="placeholder" :readonly="readonly" :autocomplete="autocomplete" @input="$emit('update:modelValue', $event.target.value)" />
      <button type="button" class="settings-secret-toggle" :aria-label="`${visible ? '隐藏' : '显示'}${label}`" :aria-controls="inputId" :aria-pressed="visible" @click="visible = !visible"><EyeOff v-if="visible" :size="16" /><Eye v-else :size="16" /></button>
    </div>
  </div>
</template>
