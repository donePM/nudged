<script setup lang="ts">
import { Events } from '@wailsio/runtime'
import { onMounted, onUnmounted, ref } from 'vue'

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
}

interface Payload {
  sessions: Session[]
  usage: { tokens: number; cost_usd: number }
}

const data = ref<Payload>({ sessions: [], usage: { tokens: 0, cost_usd: 0 } })
let off = () => {}

onMounted(() => {
  off = Events.On('sessions:update', (e) => { data.value = e.data as Payload })
  Events.Emit('sessions:ready')
})
onUnmounted(() => off())

const focus = (id: string) => Events.Emit('sessions:focus', id)
</script>

<template>
  <main>
    <table>
      <thead>
        <tr>
          <th>Project</th><th>Path</th><th>Status</th><th>Terminal</th><th>Model</th><th>Effort</th><th>Context</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="s in data.sessions" :key="s.session_id" :class="s.status" tabindex="0"
            @click="focus(s.session_id)" @keydown.enter="focus(s.session_id)">
          <td>{{ s.project }}</td>
          <td class="path" :title="s.path">{{ s.path }}</td>
          <td>{{ s.status }}</td>
          <td>{{ s.term }}</td>
          <td>{{ s.model }}</td>
          <td>{{ s.effort }}</td>
          <td>{{ s.context_pct }} %</td>
        </tr>
        <tr v-if="!data.sessions.length"><td colspan="7" class="empty">No sessions</td></tr>
      </tbody>
    </table>
    <footer>
      Today: {{ data.usage.tokens.toLocaleString() }} tokens · ≈ ${{ data.usage.cost_usd.toFixed(2) }}
    </footer>
  </main>
</template>

<style scoped>
main {
  font: 12px -apple-system, BlinkMacSystemFont, sans-serif;
  color: #1d1d1f;
  background: #f5f5f7;
  height: 100vh;
  display: flex;
  flex-direction: column;
  margin: 0;
}
table { width: 100%; border-collapse: collapse; overflow: auto; }
th, td { text-align: left; padding: 4px 8px; white-space: nowrap; }
th { font-weight: 600; color: #6e6e73; border-bottom: 1px solid #d2d2d7; }
tbody tr { cursor: pointer; }
tbody tr:hover, tbody tr:focus { background: rgba(0, 0, 0, 0.06); outline: none; }
tr.waiting td { font-weight: 600; color: #bf4800; }
.path { max-width: 200px; overflow: hidden; text-overflow: ellipsis; }
.empty { color: #6e6e73; text-align: center; cursor: default; }
footer { margin-top: auto; padding: 6px 8px; border-top: 1px solid #d2d2d7; color: #6e6e73; }
@media (prefers-color-scheme: dark) {
  main { color: #f5f5f7; background: #1e1e1e; }
  th, .empty, footer { color: #a1a1a6; }
  th, footer { border-color: #3a3a3c; }
  tbody tr:hover, tbody tr:focus { background: rgba(255, 255, 255, 0.08); }
  tr.waiting td { color: #ff9f0a; }
}
</style>
