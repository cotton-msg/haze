<template>
  <input
    :value="modelValue"
    @input="$emit('update:modelValue', ($event.target as HTMLInputElement).value)"
    @focus="focused = true"
    @blur="focused = false"
    class="w-full px-4 py-2.5 rounded-xl text-sm outline-none transition-all duration-200 placeholder:text-[var(--color-text-muted)]"
    :style="{
      background: focused ? 'rgba(255,255,255,0.07)' : 'rgba(255,255,255,0.045)',
      border: `1px solid ${focused ? 'rgba(124,108,240,0.55)' : 'rgba(255,255,255,0.09)'}`,
      color: '#e8eaf0',
      backdropFilter: 'blur(12px)',
      boxShadow: focused ? '0 0 0 3px rgba(124,108,240,0.12)' : 'none',
    }"
    v-bind="$attrs"
  />
</template>

<script setup lang="ts">
import { ref } from "vue"

defineProps<{ modelValue: string }>()
defineEmits<{ "update:modelValue": [value: string] }>()
const focused = ref(false)
</script>
