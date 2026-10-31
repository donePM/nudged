<script setup lang="ts">
import { Events } from '@wailsio/runtime'
import { computed, onMounted, onUnmounted, ref } from 'vue'
import '../card.css'
import { useFit } from '../panel'

interface Permission {
  tool: string; command: string; params: string; description: string
  project: string; path: string; term: string; tty: string; session: string; model: string
  started: string; deadline: string; suggestions: string[]
}

const id = ref('')
const p = ref<Permission | null>(null)
const others = ref(0)
const now = ref(Date.now())
const card = ref<HTMLElement | null>(null)
let off = () => {}
let timer = 0
useFit(card)

function clock(ms: number) {
  const s = Math.max(0, Math.floor(ms / 1000))
  return `${Math.floor(s / 60)}:${String(s % 60).padStart(2, '0')}`
}
const waited = computed(() => p.value ? clock(now.value - Date.parse(p.value.started)) : '')
const left = computed(() => p.value ? clock(Date.parse(p.value.deadline) - now.value) : '')
const where = computed(() => {
  if (!p.value) return ''
  return [`${p.value.term} ${p.value.tty}`.trim(), `session ${p.value.session.slice(0, 8)}`, p.value.model].filter(Boolean).join(' · ')
})

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
  <main v-if="p" ref="card" class="card permission">
    <span class="dot">!</span>
    <span class="t" :title="p.path">{{ p.project || 'Claude' }} · Permission</span>
    <span class="when" :title="`waits ${waited}, ${left} left`">
      <span v-if="others > 0" class="more">+{{ others }}</span>{{ waited }} · {{ left }} left
    </span>
    <span class="s" :title="p.description"><b class="tool">{{ p.tool }}</b><template v-if="p.description"> · {{ p.description }}</template></span>
    <pre class="wide">{{ p.command || p.params }}</pre>
    <span class="wide meta" :title="p.session">{{ where }}</span>
    <div v-if="p.suggestions.length" class="wide always">
      <button v-for="(s, i) in p.suggestions" :key="i" :title="s" @click="answer('always', i)">{{ s }}</button>
    </div>
    <div class="acts">
      <span class="gap"></span>
      <button @click="answer('deny')">Deny <kbd>esc</kbd></button>
      <button class="pri" @click="answer('allow')">Allow <kbd>⏎</kbd></button>
    </div>
  </main>
</template>

<style>
.tool { font-weight: 500; color: var(--ink); }
pre {
  margin: 10px 0 0; max-height: 7.5em; overflow: auto; padding: 8px 10px; border-radius: 9px; background: var(--code);
  font: 11px/1.45 var(--mono); white-space: pre-wrap; word-break: break-all; -webkit-user-select: text; user-select: text;
}
.meta { margin-top: 8px; font-size: 11px; color: var(--muted); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.always { display: flex; flex-direction: column; gap: 6px; margin-top: 10px; max-height: 110px; overflow-y: auto; }
.always button {
  text-align: left; padding: 5px 10px; flex-shrink: 0; font-weight: 400; font-size: 11px; line-height: 1.35;
  background: transparent; border: 1px solid var(--line); color: var(--muted); overflow-wrap: anywhere;
}
.always button:hover { color: var(--ink); background: var(--btn); }
</style>
