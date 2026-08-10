<template>
  <div
    v-if="show"
    class="glass haze-pop absolute bottom-16 left-4 rounded-2xl p-3 w-72 z-40"
    :style="{ background: 'linear-gradient(165deg, rgba(30,34,46,0.92), rgba(20,23,32,0.88))' }"
  >
    <div class="grid grid-cols-8 gap-1">
      <button
        v-for="emoji in emojis"
        :key="emoji"
        class="w-8 h-8 flex items-center justify-center rounded-lg text-lg hover:scale-110 transition-transform hover:bg-[rgba(124,108,240,0.18)]"
        @click="$emit('select', emoji)"
      >
        {{ emoji }}
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
defineProps<{ show: boolean }>()
defineEmits<{ select: [emoji: string] }>()

const emojis = [
  "👍", "❤️", "😂", "😮", "😢", "🔥", "🎉", "💯",
  "🙏", "😊", "🥰", "😍", "🤔", "😎", "💪", "✨",
  "👀", "🙌", "🫡", "😭", "🥺", "😤", "👋", "🤝",
  "⭐", "🏆", "💀", "☠️", "🎶", "💡", "📌", "🗿",
]
</script>
