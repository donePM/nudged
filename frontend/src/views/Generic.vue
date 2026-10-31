<script setup lang="ts">
import { Events } from '@wailsio/runtime'
import { computed, onMounted, onUnmounted, ref } from 'vue'
import '../card.css'
import { useFit } from '../panel'

interface Generic { kind: string; tool: string; question: string; project: string }
interface Show { request: { id: string; kind: string }; others: number; generic?: Generic }

const show = ref<Show | null>(null)
const card = ref<HTMLElement | null>(null)
let off = () => {}
useFit(card)

const g = computed(() => show.value?.generic)
const icon: Record<string, string> = { permission: '!', question: '?', done: '✓', waits: '…' }

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
  <main v-if="show" ref="card" class="card click" :class="show.request.kind" @click="jump">
    <span class="dot">{{ icon[show.request.kind] ?? '•' }}</span>
    <span class="t">{{ g?.project || 'Claude' }} · {{ show.request.kind }}</span>
    <span class="when"><span v-if="show.others > 0" class="more">+{{ show.others }}</span></span>
    <span class="s">
      <template v-if="g?.tool || g?.question"><b class="what">{{ g.tool || g.question }}</b> · </template>Click to open the terminal
    </span>
  </main>
</template>

<style>
.what { font-weight: 500; color: var(--ink); }
</style>
