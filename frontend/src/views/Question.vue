<script setup lang="ts">
import { Events } from '@wailsio/runtime'
import { computed, onMounted, onUnmounted, ref } from 'vue'
import '../card.css'
import { base, useFit } from '../panel'

interface Option { label: string; description?: string }
interface Question { question: string; header: string; options?: Option[]; multiSelect?: boolean }
interface Request { id: string; session_id: string; payload?: { cwd?: string; tool_input?: { questions?: Question[] } } }
interface Draft { picked: string[]; text: string; skipped: boolean }

const request = ref<Request | null>(null)
const others = ref(0)
const tab = ref(0)
const drafts = ref<Draft[]>([])
const card = ref<HTMLElement | null>(null)
let off = () => {}
useFit(card)

const questions = computed(() => request.value?.payload?.tool_input?.questions ?? [])
const q = computed(() => questions.value[tab.value])
const d = computed(() => drafts.value[tab.value])
const last = computed(() => tab.value >= questions.value.length - 1)

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
  if (!last.value) tab.value++
  else send()
}

// grow fits the text field to its text, up to its max height.
function grow(e: Event) {
  const el = e.target as HTMLTextAreaElement
  el.style.height = 'auto'
  el.style.height = `${el.scrollHeight + 2}px`
}

function terminal() {
  if (request.value) Events.Emit('question:terminal', { session_id: request.value.session_id })
}
</script>

<template>
  <main v-if="request && q && d" ref="card" class="card question">
    <span class="dot">?</span>
    <span class="t">{{ base(request.payload?.cwd) || 'Claude' }} · {{ q.header || 'Question' }}</span>
    <span class="when">
      <span v-if="others > 0" class="more">+{{ others }}</span>
      <template v-if="questions.length > 1">{{ tab + 1 }} / {{ questions.length }}</template>
    </span>
    <p class="wide ask">{{ q.question }}</p>
    <nav v-if="questions.length > 1" class="wide tabs">
      <button v-for="(item, i) in questions" :key="i" :class="{ pri: i === tab }" @click="tab = i">{{ item.header }}</button>
    </nav>
    <div class="wide options">
      <label v-for="o in q.options ?? []" :key="o.label" :class="{ on: d.picked.includes(o.label) }">
        <input
          :type="q.multiSelect ? 'checkbox' : 'radio'"
          :name="'q' + tab"
          :checked="d.picked.includes(o.label)"
          @change="pick(o.label, ($event.target as HTMLInputElement).checked)"
        />
        <span><b>{{ o.label }}</b><small v-if="o.description">{{ o.description }}</small></span>
      </label>
      <textarea v-model="d.text" class="text" rows="2" placeholder="Other answer (⌘⏎ sends)" @input="grow" @keydown.meta.enter.prevent="next(false)"></textarea>
    </div>
    <div class="acts">
      <button @click="terminal">Open in terminal</button>
      <span class="gap"></span>
      <button @click="next(true)">Skip</button>
      <button class="pri" @click="next(false)">{{ last ? 'Send' : 'Next' }}</button>
    </div>
  </main>
</template>

<style>
.ask { margin: 4px 0 0; color: var(--ink); line-height: 1.45; overflow-wrap: anywhere; }
.tabs { display: flex; gap: 6px; margin-top: 10px; overflow-x: auto; }
.tabs button { padding: 2px 8px; font-size: 11px; flex-shrink: 0; }
.options { display: flex; flex-direction: column; gap: 8px; margin-top: 12px; max-height: 340px; overflow-y: auto; padding: 2px; }
.options label { display: flex; gap: 10px; align-items: flex-start; padding: 9px 12px; border-radius: 9px; background: var(--btn); cursor: pointer; }
.options label.on { outline: 1.5px solid var(--accent); outline-offset: -1.5px; background: color-mix(in srgb, var(--accent) 14%, transparent); }
.options input[type='radio'], .options input[type='checkbox'] { margin: 2px 0 0; accent-color: var(--accent); }
.options b { font-weight: 500; }
.options small { display: block; margin-top: 2px; color: var(--muted); font-size: 11px; }
textarea.text {
  box-sizing: border-box; width: 100%; min-height: 52px; max-height: 120px; margin-top: 4px; padding: 8px 12px;
  font: inherit; line-height: 1.4; border-radius: 9px; border: 1px solid var(--line); background: transparent; color: inherit;
  resize: none; overflow-y: auto; -webkit-user-select: text; user-select: text;
}
</style>
