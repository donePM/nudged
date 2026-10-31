import '@wailsio/runtime'
import { createApp, defineAsyncComponent, h, shallowRef, type Component } from 'vue'

const views = import.meta.glob<Component>('./views/*.vue', { import: 'default' })

// view returns the view for the hash "#/<Name>", or the placeholder for an empty or unknown hash.
function view(): Component {
  const load = views[`./views/${location.hash.replace(/^#\/?/, '')}.vue`] ?? views['./views/Placeholder.vue']
  return defineAsyncComponent(load)
}

const current = shallowRef(view())
window.addEventListener('hashchange', () => { current.value = view() })
createApp({ render: () => h(current.value) }).mount('#app')
