<template>
  <div
    class="glass glass-hover rounded-2xl transition-shadow"
    :class="interactive ? 'cursor-pointer' : ''"
    @click="$emit('click')"
  >
    <slot />
  </div>
</template>

<script setup lang="ts">
defineProps<{ interactive?: boolean }>()
defineEmits<{ click: [] }>()
</script>
