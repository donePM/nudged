<script setup lang="ts">
// Countdown is a pie that empties counterclockwise from 12 o'clock. It is empty at the time at.
import { computed } from 'vue'

const props = defineProps<{ at: string; total: number }>()
// The animation runs for total ms and starts in the past by the time that is already gone.
const style = computed(() => {
  const gone = Math.min(props.total, Math.max(0, props.total - (Date.parse(props.at) - Date.now())))
  return { animationDuration: `${props.total}ms`, animationDelay: `-${gone}ms` }
})
</script>

<template>
  <svg :key="at" class="countdown" viewBox="0 0 20 20" aria-hidden="true">
    <circle cx="10" cy="10" r="9" class="rim" />
    <circle cx="10" cy="10" r="4" class="pie" :style="style" transform="rotate(-90 10 10)" />
  </svg>
</template>

<style>
.countdown { grid-column: 3; justify-self: end; align-self: end; width: 16px; height: 16px; color: var(--kind, var(--muted)); }
.countdown .rim { fill: none; stroke: currentColor; stroke-width: 1.5; opacity: 0.35; }
.countdown .pie {
  fill: none; stroke: currentColor; stroke-width: 8; stroke-dasharray: 25.1327 25.1327; /* 2π·4 */
  animation-name: countdown-drain; animation-timing-function: linear; animation-fill-mode: both;
}
@keyframes countdown-drain { from { stroke-dashoffset: 0; } to { stroke-dashoffset: 25.1327; } }
</style>
