<script setup lang="ts">
import { Events } from '@wailsio/runtime'
import { computed, onMounted, onUnmounted, ref } from 'vue'
import '../card.css'
import Countdown from '../components/Countdown.vue'
import { useFit } from '../panel'

interface Notice { title: string; project: string; session: string; duration: number; since: string }
interface Show { request: { id: string; kind: string }; others: number; notice?: Notice; hide_at?: string; hide_ms?: number }

const show = ref<Show | null>(null)
const card = ref<HTMLElement | null>(null)
let off = () => {}
useFit(card)

const n = computed(() => show.value?.notice ?? null)
const kind = computed(() => show.value?.request.kind ?? '')
const detail = computed(() => {
  if (!n.value) return ''
  if (kind.value === 'waits') {
    return 'since ' + new Date(n.value.since).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
  }
  const s = n.value.duration
  return 'took ' + (s >= 60 ? `${Math.floor(s / 60)}m ${s % 60}s` : `${s}s`)
})

onMounted(() => {
  off = Events.On('panel:show', (e) => { show.value = e.data as Show })
  Events.Emit('panel:ready', null)
})
onUnmounted(() => off())

function jump() {
  if (show.value) Events.Emit('panel:focus', { id: show.value.request.id })
}
</script>

<template>
  <main v-if="show && n" ref="card" class="card click" :class="kind" title="Click to open the terminal" @click="jump">
    <span class="dot">{{ kind === 'done' ? '✓' : '…' }}</span>
    <span class="t">{{ n.project || 'Claude' }} · {{ kind === 'done' ? 'done' : 'waits' }}</span>
    <span class="when"><span v-if="show.others > 0" class="more">+{{ show.others }}</span>{{ detail }}</span>
    <span class="s" :class="{ short: show.hide_at }" :title="n.session">Session {{ n.session.slice(0, 8) }} · Click to open the terminal</span>
    <Countdown v-if="show.hide_at && show.hide_ms" :at="show.hide_at" :total="show.hide_ms" />
  </main>
</template>

<style>
.s.short { grid-column: 2; }
</style>
