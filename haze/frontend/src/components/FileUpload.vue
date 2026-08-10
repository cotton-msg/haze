<template>
  <div
    class="relative"
    @dragenter.prevent="dragOver = true"
    @dragover.prevent="dragOver = true"
    @dragleave.prevent="dragOver = false"
    @drop.prevent="onDrop"
  >
    <slot :open="openPicker" />
    <input ref="input" type="file" class="hidden" multiple @change="onPick" />

    <teleport to="body">
      <div
        v-if="dragOver"
        class="fixed inset-0 z-50 flex items-center justify-center haze-fade-in"
        :style="{ background: 'rgba(8,10,15,0.5)', backdropFilter: 'blur(12px)', WebkitBackdropFilter: 'blur(12px)' }"
        @dragleave="dragOver = false"
        @drop.prevent="onDrop"
      >
        <div class="glass haze-pop rounded-3xl p-12 text-center">
          <p class="text-lg font-medium" :style="{ color: '#cdc6ff' }">Перетащите файлы сюда</p>
        </div>
      </div>
    </teleport>

    <div v-if="previews.length" class="flex flex-wrap gap-2 p-2">
      <div
        v-for="(preview, i) in previews"
        :key="i"
        class="relative w-16 h-16 rounded-lg overflow-hidden glass-soft"
      >
        <img v-if="preview.url" :src="preview.url" class="w-full h-full object-cover" />
        <div v-else class="w-full h-full flex items-center justify-center text-xs">{{ preview.name?.slice(0, 4) }}</div>
        <button
          class="absolute top-0.5 right-0.5 w-5 h-5 rounded-full flex items-center justify-center text-xs hover:bg-[rgba(255,255,255,0.2)]"
          :style="{ background: 'rgba(0,0,0,0.55)' }"
          @click="removePreview(i)"
        >✕</button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from "vue"

const emit = defineEmits<{ files: [files: File[]] }>()
const input = ref<HTMLInputElement>()
const dragOver = ref(false)

interface Preview {
  file: File
  url?: string
  name?: string
}

const previews = ref<Preview[]>([])

function openPicker() {
  input.value?.click()
}

function onPick(e: Event) {
  const files = (e.target as HTMLInputElement).files
  if (files) addFiles(Array.from(files))
}

function onDrop(e: DragEvent) {
  dragOver.value = false
  const files = e.dataTransfer?.files
  if (files) addFiles(Array.from(files))
}

function addFiles(files: File[]) {
  const items: Preview[] = files.map((f) => ({
    file: f,
    url: f.type.startsWith("image/") ? URL.createObjectURL(f) : undefined,
    name: f.name,
  }))
  previews.value.push(...items)
  emit("files", files)
}

function removePreview(i: number) {
  if (previews.value[i]?.url) URL.revokeObjectURL(previews.value[i].url!)
  previews.value.splice(i, 1)
}
</script>
