<script setup lang="ts">
import { computed } from 'vue'

// Stepper moves a number through the steps list. A value between two steps moves to the next step on that side.
const props = defineProps<{ modelValue: number; steps: number[]; label: string; format: (n: number) => string }>()
const emit = defineEmits<{ 'update:modelValue': [n: number] }>()

const less = computed(() => [...props.steps].reverse().find((s) => s < props.modelValue))
const more = computed(() => props.steps.find((s) => s > props.modelValue))
</script>

<template>
  <span class="step" role="group" :aria-label="label">
    <button type="button" :aria-label="`Less ${label}`" :disabled="less === undefined" @click="emit('update:modelValue', less!)">−</button>
    <span aria-live="polite">{{ format(modelValue) }}</span>
    <button type="button" :aria-label="`More ${label}`" :disabled="more === undefined" @click="emit('update:modelValue', more!)">+</button>
  </span>
</template>
