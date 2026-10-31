<script setup lang="ts">
import { Events } from '@wailsio/runtime'
import { computed, onMounted, onUnmounted, ref } from 'vue'

interface Permission {
  tool: string; command: string; params: string; description: string
  project: string; path: string; term: string; tty: string; session: string; model: string
  started: string; deadline: string; suggestions: string[]
}

const id = ref('')
const p = ref<Permission | null>(null)
const others = ref(0)
const now = ref(Date.now())
let off = () => {}
let timer = 0

function clock(ms: number) {
  const s = Math.max(0, Math.floor(ms / 1000))
  return `${Math.floor(s / 60)}:${String(s % 60).padStart(2, '0')}`
}
const waited = computed(() => p.value ? clock(now.value - Date.parse(p.value.started)) : '')
const left = computed(() => p.value ? clock(Date.parse(p.value.deadline) - now.value) : '')

function answer(behavior: 'allow' | 'deny' | 'always', suggestion = 0) {
  if (id.value) Events.Emit('panel:answer', { id: id.value, behavior, suggestion })
}
function key(e: KeyboardEvent) {
  if (e.key !== 'Enter' && e.key !== 'Escape') return
  if (e.key === 'Enter' && (e.target as HTMLElement).closest('button')) return // the focused button acts
  e.preventDefault()
  if (!e.repeat) answer(e.key === 'Enter' ? 'allow' : 'deny')
}

onMounted(() => {
  off = Events.On('panel:show', (e) => {
    const data = e.data as { request: { id: string }; others: number; permission?: Permission }
    id.value = data.request.id
    p.value = data.permission ?? null
    others.value = data.others
  })
  window.addEventListener('keydown', key)
  timer = window.setInterval(() => { now.value = Date.now() }, 1000)
  Events.Emit('panel:ready', null)
})
onUnmounted(() => {
  off()
  window.removeEventListener('keydown', key)
  clearInterval(timer)
})
</script>

<template>
  <main v-if="p">
    <header>
      <strong>{{ p.tool }}</strong>
      <span v-if="others > 0">+{{ others }} waiting</span>
      <span class="time">waits {{ waited }} · {{ left }} left</span>
    </header>
    <p v-if="p.description" class="desc" :title="p.description">{{ p.description }}</p>
    <pre>{{ p.command || p.params }}</pre>
    <dl>
      <dt>Project</dt><dd :title="p.path">{{ p.project }} <small>{{ p.path }}</small></dd>
      <dt>Terminal</dt><dd>{{ p.term }} {{ p.tty }}</dd>
      <dt>Session</dt><dd :title="p.session">{{ p.session }}<template v-if="p.model"> · {{ p.model }}</template></dd>
    </dl>
    <div v-if="p.suggestions.length" class="always">
      <button v-for="(s, i) in p.suggestions" :key="i" :title="s" @click="answer('always', i)">{{ s }}</button>
    </div>
    <footer>
      <button @click="answer('deny')">Deny <kbd>esc</kbd></button>
      <button class="primary" @click="answer('allow')">Allow <kbd>⏎</kbd></button>
    </footer>
  </main>
</template>

<style>
:root { color-scheme: light dark; }
body { margin: 0; overflow: hidden; font: 12px -apple-system, system-ui, sans-serif; background: #f6f6f6; color: #1d1d1f; }
main { display: flex; flex-direction: column; height: 100vh; padding: 10px; box-sizing: border-box; gap: 5px; }
header { display: flex; gap: 8px; align-items: baseline; }
header span { opacity: 0.7; }
header .time { margin-left: auto; font-variant-numeric: tabular-nums; }
.desc { flex-shrink: 0; margin: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
pre { flex: 1; min-height: 1.5em; margin: 0; overflow: auto; font-size: 11px; background: rgba(0, 0, 0, 0.05); padding: 6px; border-radius: 6px; white-space: pre-wrap; word-break: break-all; }
dl { flex-shrink: 0; display: grid; grid-template-columns: auto 1fr; gap: 1px 8px; margin: 0; font-size: 11px; }
dt { opacity: 0.6; }
dd { margin: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
small { opacity: 0.6; }
.always { display: flex; flex-direction: column; gap: 3px; flex-shrink: 0; max-height: 41px; overflow-y: auto; }
footer { display: flex; justify-content: flex-end; gap: 6px; flex-shrink: 0; }
button { font: inherit; padding: 3px 10px; border-radius: 6px; border: 1px solid rgba(0, 0, 0, 0.2); background: #fff; color: inherit; }
.always button { padding: 2px 8px; text-align: left; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; flex-shrink: 0; }
button.primary { background: #0a84ff; border-color: #0a84ff; color: #fff; }
kbd { font: inherit; opacity: 0.6; }
@media (prefers-color-scheme: dark) {
  body { background: #2a2a2c; color: #f5f5f7; }
  pre { background: rgba(255, 255, 255, 0.07); }
  button { background: #3a3a3c; border-color: rgba(255, 255, 255, 0.2); }
}
</style>
