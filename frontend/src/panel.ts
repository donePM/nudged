import { Events } from '@wailsio/runtime'
import { onUnmounted, watch, type Ref } from 'vue'

// useFit reports the height of the card el to the panel window, which then fits itself to the card.
export function useFit(el: Ref<HTMLElement | null>) {
  const ro = new ResizeObserver(() => {
    if (el.value) Events.Emit('panel:size', { height: Math.ceil(el.value.getBoundingClientRect().height) })
  })
  watch(el, (now, old) => {
    if (old) ro.unobserve(old)
    if (now) ro.observe(now)
  }, { flush: 'post' })
  onUnmounted(() => ro.disconnect())
}

// base is the last part of the path p.
export function base(p?: string): string {
  return p ? p.replace(/\/+$/, '').split('/').pop() ?? '' : ''
}
