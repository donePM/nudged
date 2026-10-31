<script setup lang="ts">
import '../controls.css'
import { Events } from '@wailsio/runtime'
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useFit } from '../panel'

interface Session {
  project: string
  path: string
  status: string
  term: string
  model: string
  effort: string
  context_pct: number
  session_id: string
  last_activity: string
  pending_kind?: string
  pending_text?: string
}

interface Payload {
  sessions: Session[]
  usage: { tokens: number; cost_usd: number }
  paused: boolean
}

const data = ref<Payload>({ sessions: [], usage: { tokens: 0, cost_usd: 0 }, paused: false })
const now = ref(Date.now())
const root = ref<HTMLElement | null>(null)
useFit(root, 'sessions:size')

let off = () => {}
let tick: ReturnType<typeof setInterval> | undefined
onMounted(() => {
  off = Events.On('sessions:update', (e) => { data.value = e.data as Payload; now.value = Date.now() })
  tick = setInterval(() => { now.value = Date.now() }, 30_000)
  Events.Emit('sessions:ready')
})
onUnmounted(() => { off(); clearInterval(tick) })

// group sorts a session: 0 needs you, 1 working, 2 idle or done.
function group(s: Session): number {
  if (s.pending_kind || s.status === 'waiting') return 0
  return s.status === 'working' ? 1 : 2
}

const sorted = computed(() =>
  [...data.value.sessions].sort((a, b) => group(a) - group(b) || b.last_activity.localeCompare(a.last_activity)))
const count = (g: number) => data.value.sessions.filter((s) => group(s) === g).length

function kind(s: Session): string {
  if (s.pending_kind === 'question') return 'question'
  if (group(s) === 0) return 'permission'
  return s.status === 'working' ? 'working' : 'idle'
}

const ring = 50.3
function ringColor(p: number) {
  return p >= 85 ? 'var(--permission)' : p >= 60 ? 'var(--waits)' : 'var(--muted)'
}

function ago(iso: string): string {
  const s = Math.max(0, (now.value - Date.parse(iso)) / 1000)
  if (!Number.isFinite(s) || s < 60) return 'now'
  if (s < 3600) return `${Math.floor(s / 60)}m`
  if (s < 86400) return `${Math.floor(s / 3600)}h`
  return `${Math.floor(s / 86400)}d`
}

function tokens(n: number): string {
  if (n >= 1e6) return `${(n / 1e6).toFixed(1)} M`
  if (n >= 1e3) return `${Math.round(n / 1e3)} k`
  return String(n)
}

const meta = (s: Session) => [s.model, s.effort, s.term].filter(Boolean).join(' · ')
const focus = (id: string) => Events.Emit('sessions:focus', id)
</script>

<template>
  <div ref="root" class="pop">
    <header>
      <div class="title">
        <h1>Sessions</h1>
        <span class="hint">{{ data.sessions.length }} running</span>
      </div>
      <div v-if="data.sessions.length" class="chips">
        <span v-if="count(0)" class="chip need">{{ count(0) }} needs you</span>
        <span v-if="count(1)" class="chip work">{{ count(1) }} working</span>
        <span v-if="count(2)" class="chip">{{ count(2) }} idle</span>
      </div>
    </header>

    <ul v-if="sorted.length" class="list">
      <li v-for="s in sorted" :key="s.session_id" class="row" tabindex="0" :title="s.path"
        @click="focus(s.session_id)" @keydown.enter="focus(s.session_id)" @keydown.space.prevent="focus(s.session_id)">
        <span :class="['dot', kind(s)]" aria-hidden="true" />
        <div class="lbl">
          <span class="name">{{ s.project }} <em v-if="s.pending_text">{{ s.pending_text }}</em></span>
          <small>{{ meta(s) || s.status }}</small>
        </div>
        <svg class="ring" viewBox="0 0 20 20" role="img" :aria-label="`Context ${s.context_pct} %`">
          <title>Context {{ s.context_pct }} %</title>
          <circle cx="10" cy="10" r="8" fill="none" stroke="var(--track)" stroke-width="2.5" />
          <circle cx="10" cy="10" r="8" fill="none" :stroke="ringColor(s.context_pct)" stroke-width="2.5"
            stroke-linecap="round" :stroke-dasharray="ring"
            :stroke-dashoffset="ring * (1 - Math.min(100, s.context_pct) / 100)" transform="rotate(-90 10 10)" />
        </svg>
        <span class="time">{{ ago(s.last_activity) }}</span>
      </li>
    </ul>
    <p v-else class="empty">No Claude Code sessions.<br>Start <code>claude</code> in a terminal.</p>

    <footer>
      <span class="hint">Today {{ tokens(data.usage.tokens) }} tokens · ${{ data.usage.cost_usd.toFixed(2) }}</span>
      <span class="icons">
        <button type="button" class="ico" :aria-pressed="data.paused" :title="data.paused ? 'Resume' : 'Pause'"
          :aria-label="data.paused ? 'Resume nudged' : 'Pause nudged'" @click="Events.Emit('sessions:pause')">
          <svg v-if="data.paused" viewBox="0 0 16 16"><path d="M5 3.5v9l7.5-4.5z" fill="currentColor" /></svg>
          <svg v-else viewBox="0 0 16 16"><rect x="4" y="3.5" width="2.6" height="9" rx="1" fill="currentColor" /><rect x="9.4" y="3.5" width="2.6" height="9" rx="1" fill="currentColor" /></svg>
        </button>
        <button type="button" class="ico" title="Settings" aria-label="Settings" @click="Events.Emit('sessions:settings')">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M10 8h4" /><path d="M12 21v-9" /><path d="M12 8V3" /><path d="M17 16h4" /><path d="M19 12V3" /><path d="M19 21v-5" /><path d="M3 14h4" /><path d="M5 10V3" /><path d="M5 21v-7" /></svg>
        </button>
      </span>
    </footer>
  </div>
</template>

<style>
body { background: var(--win); }
</style>

<style scoped>
.pop { display: flex; flex-direction: column; }
header { padding: 12px 14px 8px; display: grid; gap: 8px; }
.title { display: flex; align-items: baseline; justify-content: space-between; }
h1 { margin: 0; font-size: 14px; font-weight: 700; }
.chips { display: flex; gap: 6px; flex-wrap: wrap; }
.chip { font-size: 11px; font-weight: 600; padding: 2px 8px; border-radius: 999px; background: var(--ctl); color: var(--muted); }
.chip.need { background: color-mix(in srgb, var(--permission) 15%, transparent); color: var(--permission); }
.chip.work { background: color-mix(in srgb, var(--accent) 15%, transparent); color: var(--accent); }

.list { list-style: none; margin: 0; padding: 0 6px 6px; display: grid; gap: 1px; max-height: 520px; overflow: auto; }
.row { display: flex; align-items: center; gap: 10px; padding: 7px 8px; border-radius: 8px; cursor: pointer; }
.row:hover, .row:focus-visible { background: var(--sel); outline: none; }
.dot { width: 10px; height: 10px; border-radius: 3px; flex: none; background: var(--faint); }
.dot.permission { background: var(--permission); }
.dot.question { background: var(--question); }
.dot.working { background: var(--accent); animation: pulse 1.4s ease-in-out infinite; }
@keyframes pulse { 50% { opacity: 0.35; } }
@media (prefers-reduced-motion: reduce) { .dot.working { animation: none; } }
.lbl { flex: 1; min-width: 0; display: grid; gap: 1px; }
.name { font-weight: 600; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.name em { font-style: normal; font-weight: 400; color: var(--muted); }
.lbl small { color: var(--muted); font-size: 11px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.ring { width: 18px; height: 18px; flex: none; }
.time { width: 28px; text-align: right; font-size: 11px; color: var(--faint); font-variant-numeric: tabular-nums; flex: none; }

.empty { margin: 0; padding: 18px 14px 22px; text-align: center; color: var(--muted); }
.empty code { font: 12px ui-monospace, Menlo, monospace; }

footer { display: flex; align-items: center; justify-content: space-between; padding: 7px 10px 7px 14px; box-shadow: inset 0 0.5px 0 var(--sep); }
.icons { display: flex; gap: 2px; }
.ico { width: 26px; height: 26px; border: 0; border-radius: 6px; background: none; color: var(--muted); cursor: pointer; display: grid; place-items: center; }
.ico:hover { background: var(--sel); color: var(--ink); }
.ico[aria-pressed="true"] { color: var(--accent); }
.ico svg { width: 15px; height: 15px; }
</style>
