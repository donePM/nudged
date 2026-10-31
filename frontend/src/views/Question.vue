<script setup lang="ts">
import { Events } from '@wailsio/runtime'
import { computed, onMounted, onUnmounted, ref } from 'vue'

interface Option { label: string; description?: string }
interface Question { question: string; header: string; options?: Option[]; multiSelect?: boolean }
interface Request { id: string; session_id: string; payload?: { tool_input?: { questions?: Question[] } } }
interface Draft { picked: string[]; text: string; skipped: boolean }

const request = ref<Request | null>(null)
const others = ref(0)
const tab = ref(0)
const drafts = ref<Draft[]>([])
let off = () => {}

const questions = computed(() => request.value?.payload?.tool_input?.questions ?? [])
const q = computed(() => questions.value[tab.value])
const d = computed(() => drafts.value[tab.value])

onMounted(() => {
  off = Events.On('panel:show', (e) => {
    const data = e.data as { request: Request; others: number }
    others.value = data.others
    if (request.value?.id === data.request.id) return
    request.value = data.request
    tab.value = 0
    drafts.value = questions.value.map(() => ({ picked: [], text: '', skipped: false }))
  })
  Events.Emit('panel:ready', null)
})
onUnmounted(() => off())

function pick(label: string, checked: boolean) {
  if (!q.value.multiSelect) d.value.picked = [label]
  else d.value.picked = checked ? [...d.value.picked, label] : d.value.picked.filter((l) => l !== label)
}

// Free text is one more value. In single select it replaces the option.
function values(question: Question, draft: Draft): string[] {
  const text = draft.text.trim()
  if (draft.skipped) return []
  if (!question.multiSelect) return text ? [text] : draft.picked.slice(0, 1)
  return text ? [...draft.picked, text] : draft.picked
}

function send() {
  if (!request.value) return
  const answers: Record<string, string[]> = {}
  questions.value.forEach((question, i) => {
    const v = values(question, drafts.value[i])
    if (v.length) answers[question.question] = v
  })
  Events.Emit('question:answer', { id: request.value.id, answers })
}

function next(skip: boolean) {
  d.value.skipped = skip
  if (tab.value < questions.value.length - 1) tab.value++
  else send()
}

function terminal() {
  if (request.value) Events.Emit('question:terminal', { session_id: request.value.session_id })
}
</script>

<template>
  <main v-if="request && q && d">
    <header>
      <nav>
        <button v-for="(item, i) in questions" :key="i" :class="{ active: i === tab }" @click="tab = i">{{ item.header }}</button>
      </nav>
      <span v-if="others > 0">+{{ others }} waiting</span>
    </header>
    <section>
      <p>{{ q.question }}</p>
      <label v-for="o in q.options ?? []" :key="o.label">
        <input
          :type="q.multiSelect ? 'checkbox' : 'radio'"
          :name="'q' + tab"
          :checked="d.picked.includes(o.label)"
          @change="pick(o.label, ($event.target as HTMLInputElement).checked)"
        />
        <span><strong>{{ o.label }}</strong> <small v-if="o.description">{{ o.description }}</small></span>
      </label>
      <input v-model="d.text" class="text" type="text" placeholder="Other answer" />
    </section>
    <footer>
      <button @click="terminal">Open in terminal</button>
      <span></span>
      <button @click="next(true)">Skip</button>
      <button class="primary" @click="next(false)">Next</button>
    </footer>
  </main>
</template>

<style>
:root { color-scheme: light dark; }
body { margin: 0; font: 13px -apple-system, system-ui, sans-serif; background: #f6f6f6; color: #1d1d1f; }
main { display: flex; flex-direction: column; height: 100vh; padding: 10px; box-sizing: border-box; gap: 8px; }
header { display: flex; justify-content: space-between; align-items: center; gap: 8px; }
header span { opacity: 0.7; white-space: nowrap; }
nav { display: flex; gap: 4px; overflow-x: auto; }
nav button { padding: 2px 8px; }
nav button.active { background: #0a84ff; border-color: #0a84ff; color: #fff; }
section { flex: 1; overflow: auto; display: flex; flex-direction: column; gap: 4px; }
section p { margin: 0 0 2px; font-weight: 600; }
label { display: flex; gap: 6px; align-items: flex-start; }
small { opacity: 0.7; }
input.text { font: inherit; padding: 3px 6px; border-radius: 6px; border: 1px solid rgba(0, 0, 0, 0.2); background: #fff; color: inherit; }
footer { display: flex; gap: 8px; }
footer span { flex: 1; }
button { font: inherit; padding: 4px 14px; border-radius: 6px; border: 1px solid rgba(0, 0, 0, 0.2); background: #fff; color: inherit; }
button.primary { background: #0a84ff; border-color: #0a84ff; color: #fff; }
@media (prefers-color-scheme: dark) {
  body { background: #2a2a2c; color: #f5f5f7; }
  button, input.text { background: #3a3a3c; border-color: rgba(255, 255, 255, 0.2); }
}
</style>
