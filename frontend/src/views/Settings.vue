<script setup lang="ts">
import { Events } from '@wailsio/runtime'
import { onMounted, onUnmounted, ref } from 'vue'

// The field names match the Go struct config.Config.
interface Event { Show: boolean; Sound: boolean; HideSeconds: number }
interface Config {
  Mode: string
  TimeoutMinutes: number
  StartAtLogin: boolean
  Events: { Permission: Event; Question: Event; Done: Event; Waits: Event }
}

const rows = [
  { key: 'permission', field: 'Permission', label: 'Permission' },
  { key: 'question', field: 'Question', label: 'Question' },
  { key: 'done', field: 'Done', label: 'Session done' },
  { key: 'waits', field: 'Waits', label: 'Session waits' },
] as const

const cfg = ref<Config | null>(null)
const status = ref('')
const failed = ref(false)

function show(msg: string, error = false) {
  status.value = msg
  failed.value = error
}

const offs: (() => void)[] = []
onMounted(() => {
  offs.push(
    Events.On('settings:config', (e) => { cfg.value = e.data as Config }),
    Events.On('settings:saved', (e) => { cfg.value = e.data as Config; show('Settings saved.') }),
    Events.On('settings:error', (e) => show(String(e.data), true)),
    Events.On('settings:hooks-result', (e) => {
      const r = e.data as { ok: boolean; message: string }
      show(r.message, !r.ok)
    }),
  )
  Events.Emit('settings:get')
})
onUnmounted(() => offs.forEach((off) => off()))

function save() {
  if (cfg.value) Events.Emit('settings:save', cfg.value)
}
</script>

<template>
  <form v-if="cfg" class="settings" @submit.prevent="save">
    <label>Mode
      <select v-model="cfg.Mode">
        <option value="answer">Answer directly</option>
        <option value="notify">Notify only</option>
      </select>
    </label>
    <label>Timeout (minutes)
      <input v-model.number="cfg.TimeoutMinutes" type="number" min="1" required>
    </label>
    <table>
      <thead>
        <tr><th>Event</th><th>Show</th><th>Sound</th><th></th><th>Hide after (s)</th></tr>
      </thead>
      <tbody>
        <tr v-for="r in rows" :key="r.key">
          <td>{{ r.label }}</td>
          <td><input v-model="cfg.Events[r.field].Show" type="checkbox" :aria-label="`Show ${r.label}`"></td>
          <td><input v-model="cfg.Events[r.field].Sound" type="checkbox" :aria-label="`Sound ${r.label}`"></td>
          <td><button type="button" @click="Events.Emit('settings:sound', r.key)">Test</button></td>
          <td>
            <input v-model.number="cfg.Events[r.field].HideSeconds" type="number" min="0" required
              :aria-label="`Hide ${r.label} after seconds`">
          </td>
        </tr>
      </tbody>
    </table>
    <div class="buttons">
      <button type="button" @click="Events.Emit('settings:hooks', 'install')">Install hooks</button>
      <button type="button" @click="Events.Emit('settings:hooks', 'remove')">Remove hooks</button>
      <button type="submit">Save</button>
    </div>
    <p class="status" :class="{ error: failed }" role="status">{{ status }}</p>
  </form>
  <p v-else class="status" :class="{ error: failed }" role="status">{{ status || 'Loading…' }}</p>
</template>

<style>
:root { color-scheme: light dark; font: 13px -apple-system, system-ui, sans-serif; }
body { margin: 0; background: Canvas; color: CanvasText; }
</style>

<style scoped>
.settings { display: flex; flex-direction: column; gap: 12px; padding: 16px; }
label { display: flex; justify-content: space-between; align-items: center; gap: 12px; }
table { border-collapse: collapse; }
th, td { padding: 4px 6px; text-align: left; }
input[type='number'] { width: 5em; }
.buttons { display: flex; gap: 8px; }
.buttons button[type='submit'] { margin-left: auto; }
.status { min-height: 1.2em; margin: 0; padding: 0 16px 16px; }
.settings .status { padding: 0; }
.error { color: #d70015; }
@media (prefers-color-scheme: dark) { .error { color: #ff6961; } }
</style>
