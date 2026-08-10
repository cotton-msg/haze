<template>
  <div
    v-if="isTauriEnv"
    class="h-10 shrink-0 flex items-center justify-between select-none"
    :style="{
      background: 'rgba(16,19,27,0.72)',
      borderBottom: '1px solid rgba(255,255,255,0.07)',
      backdropFilter: 'blur(18px) saturate(140%)',
      WebkitBackdropFilter: 'blur(18px) saturate(140%)',
      WebkitUserSelect: 'none',
    }"
    data-tauri-drag-region
  >
    <div class="flex items-center gap-2 pl-4 pointer-events-none" data-tauri-drag-region>
      <span
        class="w-3.5 h-3.5 rounded-[5px] flex items-center justify-center text-[9px] font-bold"
        :style="{ background: 'linear-gradient(135deg, #6d5ee8, #2dd4bf)', color: '#fff' }"
      >H</span>
      <span class="text-xs font-medium tracking-wide" :style="{ color: 'rgba(255,255,255,0.75)' }">Haze</span>
    </div>

    <div class="flex h-full">
      <button
        v-for="btn in controls"
        :key="btn.label"
        class="w-11 h-full flex items-center justify-center text-sm transition-colors"
        :class="btn.action === 'close' ? 'hover:bg-[#e5484d] hover:text-white' : 'hover:bg-[rgba(255,255,255,0.08)]'"
        :style="{ color: 'rgba(255,255,255,0.7)' }"
        :aria-label="btn.label"
        @click="onControl(btn.action)"
      >
        {{ btn.icon }}
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from "vue"
import { isTauri, invoke } from "@tauri-apps/api/core"

const controls = [
  { action: "minimize", icon: "—", label: "Свернуть" },
  { action: "maximize", icon: "▢", label: "Развернуть" },
  { action: "close", icon: "✕", label: "Закрыть" },
] as const

const isTauriEnv = isTauri()

async function onControl(action: "minimize" | "maximize" | "close") {
  try {
    await invoke(action === "maximize" ? "toggle_maximize_window" : action === "close" ? "close_window" : "minimize_window")
  } catch {
    /* ignore */
  }
}
</script>
