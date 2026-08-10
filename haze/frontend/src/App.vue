<template>
  <div class="h-full w-full flex flex-col overflow-hidden">
    <TitleBar />
    <div class="flex-1 min-h-0">
      <router-view />
    </div>
  </div>
</template>

<script setup lang="ts">
import TitleBar from "./components/TitleBar.vue"
</script>
