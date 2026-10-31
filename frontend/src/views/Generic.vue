<script setup lang="ts">
import { Events } from '@wailsio/runtime'
import { onMounted, onUnmounted, ref } from 'vue'

interface Request { id: string; kind: string; [key: string]: unknown }

const request = ref<Request | null>(null)
const others = ref(0)
let off = () => {}

onMounted(() => {
  off = Events.On('panel:show', (e) => {
    const data = e.data as { request: Request; others: number }
    request.value = data.request
    others.value = data.others
  })
  Events.Emit('panel:ready', null)
})
onUnmounted(() => off())

function answer(behavior: 'allow' | 'deny') {
  if (request.value) Events.Emit('panel:answer', { id: request.value.id, behavior })
}
</script>

<template>
  <main v-if="request">
    <header>
      <strong>{{ request.kind }}</strong>
      <span v-if="others > 0">+{{ others }} waiting</span>
    </header>
    <pre>{{ JSON.stringify(request, null, 2) }}</pre>
    <footer v-if="request.kind === 'permission'">
      <button @click="answer('deny')">Deny</button>
      <button class="primary" @click="answer('allow')">Allow</button>
    </footer>
  </main>
</template>

<style>
:root { color-scheme: light dark; }
body { margin: 0; font: 13px -apple-system, system-ui, sans-serif; background: #f6f6f6; color: #1d1d1f; }
main { display: flex; flex-direction: column; height: 100vh; padding: 10px; box-sizing: border-box; gap: 8px; }
header { display: flex; justify-content: space-between; text-transform: capitalize; }
header span { opacity: 0.7; text-transform: none; }
pre { flex: 1; margin: 0; overflow: auto; font-size: 11px; background: rgba(0, 0, 0, 0.05); padding: 6px; border-radius: 6px; }
footer { display: flex; justify-content: flex-end; gap: 8px; }
button { font: inherit; padding: 4px 14px; border-radius: 6px; border: 1px solid rgba(0, 0, 0, 0.2); background: #fff; color: inherit; }
button.primary { background: #0a84ff; border-color: #0a84ff; color: #fff; }
@media (prefers-color-scheme: dark) {
  body { background: #2a2a2c; color: #f5f5f7; }
  pre { background: rgba(255, 255, 255, 0.07); }
  button { background: #3a3a3c; border-color: rgba(255, 255, 255, 0.2); }
}
</style>
