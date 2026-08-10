<template>
  <teleport to="body">
    <div
      v-if="show"
      class="fixed inset-0 z-50 flex items-center justify-center haze-fade-in"
      @click.self="$emit('close')"
      :style="{ background: 'rgba(8,10,15,0.5)', backdropFilter: 'blur(10px)', WebkitBackdropFilter: 'blur(10px)' }"
    >
      <div
        class="glass haze-pop rounded-2xl p-6 w-full max-w-md mx-4"
        :style="{ background: 'linear-gradient(165deg, rgba(30,34,46,0.92), rgba(20,23,32,0.88))' }"
      >
        <slot />
      </div>
    </div>
  </teleport>
</template>

<script setup lang="ts">
defineProps<{ show: boolean }>()
defineEmits<{ close: [] }>()
</script>
