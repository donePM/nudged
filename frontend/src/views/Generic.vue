<script setup lang="ts">
import { Events } from '@wailsio/runtime'
import { onMounted, onUnmounted, ref } from 'vue'

interface Generic { kind: string; tool: string; question: string; project: string }
interface Request { id: string; kind: string }

const request = ref<Request | null>(null)
const generic = ref<Generic | null>(null)
const others = ref(0)
let off = () => {}

onMounted(() => {
  off = Events.On('panel:show', (e) => {
    const data = e.data as { request: Request; others: number; generic?: Generic }
    request.value = data.request
    generic.value = data.generic ?? null
    others.value = data.others
  })
  Events.Emit('panel:ready', null)
})
onUnmounted(() => off())

function jump() {
  if (request.value) Events.Emit('panel:focus', { id: request.value.id })
}
</script>

<template>
  <main v-if="request" @click="jump">
    <header>
      <strong>{{ request.kind }}</strong>
      <span v-if="others > 0">+{{ others }} waiting</span>
    </header>
    <p v-if="generic?.tool || generic?.question" class="what">{{ generic.tool || generic.question }}</p>
    <p v-if="generic?.project" class="project">{{ generic.project }}</p>
    <p class="hint">Click to open the terminal</p>
  </main>
</template>

<style>
:root { color-scheme: light dark; }
body { margin: 0; font: 13px -apple-system, system-ui, sans-serif; background: #f6f6f6; color: #1d1d1f; }
main { display: flex; flex-direction: column; height: 100vh; padding: 10px; box-sizing: border-box; gap: 8px; cursor: pointer; }
header { display: flex; justify-content: space-between; text-transform: capitalize; }
header span { opacity: 0.7; text-transform: none; }
p { margin: 0; }
.what { font-size: 15px; overflow-wrap: anywhere; overflow: hidden; display: -webkit-box; -webkit-box-orient: vertical; -webkit-line-clamp: 4; line-clamp: 4; }
.project, .hint { opacity: 0.7; }
@media (prefers-color-scheme: dark) {
  body { background: #2a2a2c; color: #f5f5f7; }
}
</style>
