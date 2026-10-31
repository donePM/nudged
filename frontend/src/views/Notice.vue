<script setup lang="ts">
import { Events } from '@wailsio/runtime'
import { computed, onMounted, onUnmounted, ref } from 'vue'

interface Notice { title: string; project: string; session: string; duration: number; since: string }

const id = ref('')
const kind = ref('')
const n = ref<Notice | null>(null)
const others = ref(0)
let off = () => {}

const detail = computed(() => {
  if (!n.value) return ''
  if (kind.value === 'waits') {
    return 'since ' + new Date(n.value.since).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
  }
  const s = n.value.duration
  return 'took ' + (s >= 60 ? `${Math.floor(s / 60)}m ${s % 60}s` : `${s}s`)
})

onMounted(() => {
  off = Events.On('panel:show', (e) => {
    const data = e.data as { request: { id: string; kind: string }; others: number; notice?: Notice }
    id.value = data.request.id
    kind.value = data.request.kind
    n.value = data.notice ?? null
    others.value = data.others
  })
  Events.Emit('panel:ready', null)
})
onUnmounted(() => off())

function jump() {
  if (id.value) Events.Emit('panel:focus', { id: id.value })
}
</script>

<template>
  <main v-if="n" @click="jump">
    <header>
      <strong>{{ n.title }}</strong>
      <span v-if="others > 0">+{{ others }} waiting</span>
    </header>
    <p class="project">{{ n.project }}</p>
    <p :title="n.session">Session {{ n.session.slice(0, 8) }}</p>
    <p class="detail">{{ detail }}</p>
  </main>
</template>

<style>
:root { color-scheme: light dark; }
body { margin: 0; overflow: hidden; font: 13px -apple-system, system-ui, sans-serif; background: #f6f6f6; color: #1d1d1f; }
main { display: flex; flex-direction: column; height: 100vh; padding: 10px; box-sizing: border-box; gap: 6px; cursor: pointer; }
header { display: flex; justify-content: space-between; }
header span { opacity: 0.7; }
p { margin: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.project { font-size: 15px; font-weight: 600; }
.detail { opacity: 0.7; font-variant-numeric: tabular-nums; }
@media (prefers-color-scheme: dark) {
  body { background: #2a2a2c; color: #f5f5f7; }
}
</style>
