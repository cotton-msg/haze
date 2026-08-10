<template>
  <GlassModal :show="show" @close="$emit('close')">
    <div class="flex flex-col gap-4">
      <h2 class="text-lg font-semibold">Профиль</h2>

      <div class="flex items-center gap-4">
        <label class="relative cursor-pointer group">
          <Avatar :alt="displayName" :size="64" :src="avatarUrl" />
          <div
            class="absolute inset-0 rounded-full flex items-center justify-center text-xs opacity-0 group-hover:opacity-100 transition-opacity"
            :style="{ background: 'rgba(0,0,0,0.5)', color: '#fff' }"
          >
            Изменить
          </div>
          <input type="file" accept="image/*" class="hidden" @change="onAvatarPick" />
        </label>
        <div class="text-sm">
          <p class="font-medium">{{ store.user?.display_name }}</p>
          <p class="text-xs" :style="{ color: 'var(--color-text-muted)' }">@{{ store.user?.username }}</p>
        </div>
      </div>

      <div>
        <label class="text-xs" :style="{ color: 'var(--color-text-muted)' }">Имя</label>
        <GlassInput v-model="displayName" class="mt-1" placeholder="Как вас зовут" />
      </div>

      <div>
        <label class="text-xs" :style="{ color: 'var(--color-text-muted)' }">Username</label>
        <GlassInput v-model="username" class="mt-1" placeholder="username" />
      </div>

      <div>
        <label class="text-xs" :style="{ color: 'var(--color-text-muted)' }">О себе</label>
        <textarea
          v-model="bio"
          rows="3"
          class="glass-soft w-full mt-1 rounded-xl px-3 py-2 text-sm resize-none outline-none"
          placeholder="Расскажите о себе"
        />
      </div>

      <div class="flex gap-2.5 pt-1">
        <GlassButton class="flex-1" :disabled="saving" @click="save">
          {{ saving ? "Сохраняем…" : "Сохранить" }}
        </GlassButton>
        <GlassButton class="flex-1" variant="secondary" @click="$emit('close')">Отмена</GlassButton>
      </div>
    </div>
  </GlassModal>
</template>

<script setup lang="ts">
import { ref, computed, watch } from "vue"
import { useUserStore } from "../stores/userStore"
import GlassModal from "./GlassModal.vue"
import GlassInput from "./GlassInput.vue"
import GlassButton from "./GlassButton.vue"
import Avatar from "./Avatar.vue"

const props = defineProps<{ show: boolean }>()
const emit = defineEmits<{ close: [] }>()

const store = useUserStore()

const displayName = ref("")
const username = ref("")
const bio = ref("")
const avatarUrl = ref("")
const saving = ref(false)

watch(
  () => props.show,
  (show) => {
    if (show && store.user) {
      displayName.value = store.user.display_name || ""
      username.value = store.user.username || ""
      bio.value = store.user.bio || ""
      avatarUrl.value = store.user.avatar_url || ""
    }
  },
)

async function onAvatarPick(e: Event) {
  const input = e.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return
  saving.value = true
  const ok = await store.uploadAvatar(file)
  if (ok && store.user) avatarUrl.value = store.user.avatar_url
  saving.value = false
}

async function save() {
  saving.value = true
  const ok = await store.updateProfile({
    display_name: displayName.value,
    username: username.value || undefined,
    bio: bio.value,
  })
  saving.value = false
  if (ok) emit("close")
}
</script>
