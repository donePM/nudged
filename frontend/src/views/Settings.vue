<script setup lang="ts">
import '../controls.css'
import { Browser, Events } from '@wailsio/runtime'
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import DonePM from '../components/DonePM.vue'
import Stepper from '../components/Stepper.vue'

// The field names match the Go struct config.Config.
interface Event { Show: boolean; Sound: boolean; HideSeconds: number }
interface Config {
  Mode: string
  TimeoutMinutes: number
  StartAtLogin: boolean
  CheckUpdates: boolean
  Position: string
  Events: { Permission: Event; Question: Event; Done: Event; Waits: Event }
}
interface Update { version: string; latest?: string; url?: string; update: boolean; checking: boolean; error?: string }

const site = 'https://done.pm'
const timeoutSteps = [1, 2, 5, 10, 15, 20, 30]
const hideSteps = [0, 3, 5, 10, 15, 20, 30, 45, 60, 90, 120]

const pages = ['Notifications', 'General', 'Updates', 'About'] as const
type Page = (typeof pages)[number]

const rows = [
  { key: 'permission', field: 'Permission', label: 'Permission', mark: '!', help: 'Claude Code wants to run a tool and needs your OK.', held: true },
  { key: 'question', field: 'Question', label: 'Question', mark: '?', help: 'Claude Code asks you to pick an option or type an answer.', held: true },
  { key: 'done', field: 'Done', label: 'Session done', mark: '✓', help: 'A session finished its turn.', held: false },
  { key: 'waits', field: 'Waits', label: 'Session waits', mark: '…', help: 'A session is idle and waits for your next prompt.', held: false },
] as const

const corners = [
  { value: 'top-left', label: 'Top left' },
  { value: 'top-right', label: 'Top right' },
  { value: 'bottom-left', label: 'Bottom left' },
  { value: 'bottom-right', label: 'Bottom right' },
] as const

const page = ref<Page>('Notifications')
const cfg = ref<Config | null>(null)
const upd = ref<Update>({ version: '', update: false, checking: false })
const hooks = ref<boolean | null>(null)
const status = ref('')
const failed = ref(false)
const hookMsg = ref<{ ok: boolean; message: string } | null>(null)

// last is the JSON of the config the server knows. Autosave skips a config equal to it.
let last = ''
let timer: ReturnType<typeof setTimeout> | undefined
let flash: ReturnType<typeof setTimeout> | undefined

function show(msg: string, error = false) {
  status.value = msg
  failed.value = error
  clearTimeout(flash)
  if (!error) flash = setTimeout(() => { status.value = '' }, 1500)
}

watch(cfg, (c) => {
  if (!c || JSON.stringify(c) === last) return
  clearTimeout(timer)
  timer = setTimeout(() => Events.Emit('settings:save', c), 300)
}, { deep: true })

const offs: (() => void)[] = []
onMounted(() => {
  offs.push(
    Events.On('settings:config', (e) => { last = JSON.stringify(e.data); cfg.value = e.data as Config }),
    // The saved config is the one the view sent, so the view keeps its own copy and edits made since.
    Events.On('settings:saved', (e) => { last = JSON.stringify(e.data); show('Saved') }),
    Events.On('settings:error', (e) => show(String(e.data), true)),
    Events.On('settings:page', (e) => {
      const p = pages.find((n) => n.toLowerCase() === String(e.data).toLowerCase())
      if (p) page.value = p
    }),
    Events.On('settings:update', (e) => { upd.value = e.data as Update }),
    Events.On('settings:hooks-state', (e) => { hooks.value = Boolean(e.data) }),
    Events.On('settings:hooks-result', (e) => { hookMsg.value = e.data as { ok: boolean; message: string } }),
  )
  Events.Emit('settings:get')
})
onUnmounted(() => {
  offs.forEach((off) => off())
  clearTimeout(timer)
  clearTimeout(flash)
})

function setSound(r: (typeof rows)[number], on: boolean) {
  if (!cfg.value) return
  cfg.value.Events[r.field].Sound = on
  if (on) Events.Emit('settings:sound', r.key)
}

const hookAction = (a: 'install' | 'remove') => { hookMsg.value = null; Events.Emit('settings:hooks', a) }
const open = (url: string) => Browser.OpenURL(url)
const minutes = (n: number) => `${n} min`
const seconds = (n: number) => (n === 0 ? 'Never' : `${n} s`)
const dev = computed(() => !/^\d+\.\d+\.\d+$/.test(upd.value.version))
</script>

<template>
  <div class="win">
    <aside class="side">
      <div class="drag top" />
      <div class="drag app">
        <div class="icon small" aria-hidden="true"><span /></div>
        <span>nudged</span>
      </div>
      <nav aria-label="Settings pages">
        <button v-for="p in pages" :key="p" type="button" class="nav" :aria-current="page === p ? 'page' : undefined"
          @click="page = p">
          {{ p }}
          <span v-if="p === 'Updates' && upd.update" class="dot" aria-label="update available" />
        </button>
      </nav>
      <div class="brand foot">
        <DonePM />
        <span>a service by <button type="button" class="link" @click="open(site)">donePM</button></span>
      </div>
    </aside>

    <main class="main">
      <header class="drag bar">
        <h1>{{ page }}</h1>
        <span class="status" :class="{ error: failed }" role="status">{{ status }}</span>
      </header>

      <div v-if="!cfg" class="body"><p class="hint">Loading…</p></div>

      <div v-else-if="page === 'Notifications'" class="body">
        <section class="group">
          <div class="item">
            <div class="lbl">Mode<small>Answer permissions and questions in the panel, or only get notified and answer in the terminal.</small></div>
            <span class="seg" role="radiogroup" aria-label="Mode">
              <button type="button" role="radio" :aria-checked="cfg.Mode === 'answer'" @click="cfg.Mode = 'answer'">Answer</button>
              <button type="button" role="radio" :aria-checked="cfg.Mode === 'notify'" @click="cfg.Mode = 'notify'">Notify</button>
            </span>
          </div>
          <div class="item">
            <div class="lbl">Position<small>Where the cards show up on your screen.</small></div>
            <div class="screen" role="radiogroup" aria-label="Position">
              <button v-for="c in corners" :key="c.value" type="button" role="radio" :class="['corner', c.value]"
                :aria-checked="cfg.Position === c.value" :aria-label="c.label" :title="c.label"
                @click="cfg.Position = c.value" />
            </div>
          </div>
        </section>

        <section class="group events">
          <div class="item head" aria-hidden="true">
            <span class="lbl" />
            <span class="col">Show</span><span class="col">Sound</span><span class="col wide">Hide after</span>
          </div>
          <div v-for="r in rows" :key="r.key" class="item">
            <span :class="['sq', `k-${r.key}`]" aria-hidden="true">{{ r.mark }}</span>
            <div class="lbl">{{ r.label }}<small>{{ r.help }}</small></div>
            <span class="col">
              <button type="button" class="sw" role="switch" :aria-checked="cfg.Events[r.field].Show"
                :aria-label="`Show ${r.label}`" @click="cfg.Events[r.field].Show = !cfg.Events[r.field].Show" />
            </span>
            <span class="col">
              <button type="button" class="sw" role="switch" :aria-checked="cfg.Events[r.field].Sound"
                :aria-label="`${r.label} sound`" @click="setSound(r, !cfg.Events[r.field].Sound)" />
            </span>
            <span class="col wide">
              <span v-if="r.held" class="hint">until answered</span>
              <Stepper v-else v-model="cfg.Events[r.field].HideSeconds" :steps="hideSteps"
                :label="`${r.label} hide after`" :format="seconds" />
            </span>
          </div>
        </section>
        <p class="hint legend">Show: put a card in the corner you picked. Sound: play a sound, click to hear it. Hide after: close the card by itself, 0 keeps it.</p>
      </div>

      <div v-else-if="page === 'General'" class="body">
        <section class="group">
          <div class="item">
            <div class="lbl">Timeout<small>How long nudged holds a request before Claude Code asks in the terminal.</small></div>
            <Stepper v-model="cfg.TimeoutMinutes" :steps="timeoutSteps" label="timeout" :format="minutes" />
          </div>
          <div class="item">
            <div class="lbl">Start at login<small>Opens nudged in the menu bar when you log in to your Mac.</small></div>
            <button type="button" class="sw" role="switch" :aria-checked="cfg.StartAtLogin" aria-label="Start at login"
              @click="cfg.StartAtLogin = !cfg.StartAtLogin" />
          </div>
        </section>
        <section class="group">
          <div class="item">
            <div class="lbl">Hooks<small>Adds nudged to ~/.claude/settings.json so Claude Code sends events.</small></div>
            <span v-if="hooks !== null" :class="['pill', hooks ? 'ok' : 'bad']">{{ hooks ? 'Installed' : 'Not installed' }}</span>
          </div>
          <div class="item">
            <span class="lbl"><small :class="{ error: hookMsg && !hookMsg.ok }" role="status">{{ hookMsg?.message }}</small></span>
            <button v-if="hooks" type="button" class="btn" @click="hookAction('remove')">Remove</button>
            <button type="button" class="btn pri" @click="hookAction('install')">{{ hooks ? 'Reinstall' : 'Install' }}</button>
          </div>
        </section>
      </div>

      <div v-else-if="page === 'Updates'" class="body">
        <section class="group">
          <div class="item">
            <div class="lbl">nudged {{ upd.version }}
              <small v-if="upd.checking">Checking…</small>
              <small v-else-if="upd.error" class="error">{{ upd.error }}</small>
              <small v-else-if="upd.update">Version {{ upd.latest }} is out. Installs with Homebrew, then nudged restarts.</small>
              <small v-else-if="upd.latest">You have the newest version.</small>
              <small v-else-if="dev">Dev build: updates go through Homebrew releases only.</small>
              <small v-else>Not checked yet.</small>
            </div>
            <span v-if="upd.update" class="pill new">New</span>
          </div>
          <div class="item">
            <span class="lbl" />
            <button v-if="upd.url" type="button" class="link" @click="open(upd.url)">Release notes</button>
            <button type="button" class="btn" :disabled="upd.checking" @click="Events.Emit('settings:update-check')">Check now</button>
            <button v-if="upd.update" type="button" class="btn pri" :disabled="upd.checking"
              @click="Events.Emit('settings:update-install')">Update</button>
          </div>
        </section>
        <section class="group">
          <div class="item">
            <div class="lbl">Check for updates<small>Looks for a new nudged release once a day. Nothing installs until you click Update.</small></div>
            <button type="button" class="sw" role="switch" :aria-checked="cfg.CheckUpdates" aria-label="Check for updates"
              @click="cfg.CheckUpdates = !cfg.CheckUpdates" />
          </div>
        </section>
      </div>

      <div v-else class="body about">
        <div class="icon" aria-hidden="true"><span /></div>
        <h2>nudged</h2>
        <p class="hint">Version {{ upd.version || '—' }}</p>
        <p>Answers and notices for Claude Code, in your menu bar.</p>
        <div class="brand big">
          <DonePM />
          <span>nudged is a service by <button type="button" class="link" @click="open(site)">donePM</button></span>
        </div>
      </div>
    </main>
  </div>
</template>

<style>
body { background: var(--win); }
</style>

<style scoped>
.win { display: flex; height: 100vh; overflow: hidden; }
.side { width: 180px; flex: none; background: var(--side); box-shadow: inset -0.5px 0 0 var(--sep); display: flex; flex-direction: column; }
.top { height: 52px; flex: none; }
.app { display: flex; align-items: center; gap: 8px; padding: 0 20px 12px; font-size: 14px; font-weight: 700; }
.icon.small { width: 22px; height: 22px; border-radius: 6px; box-shadow: 0 1px 3px rgba(0, 0, 0, 0.25); }
.icon.small span { width: 7px; height: 7px; box-shadow: 0 0 0 3px rgba(255, 255, 255, 0.14); }
nav { display: grid; gap: 2px; padding: 0 10px; }
.nav { display: flex; align-items: center; justify-content: space-between; border: 0; background: none; color: var(--ink);
  font: 13px var(--sys); text-align: left; padding: 5px 10px; border-radius: 6px; cursor: pointer; }
.nav[aria-current="page"] { background: var(--sel); font-weight: 600; }
.dot { width: 7px; height: 7px; border-radius: 50%; background: var(--accent); }
.foot { margin-top: auto; padding: 14px 16px; }

.main { flex: 1; min-width: 0; display: flex; flex-direction: column; }
.bar { height: 52px; flex: none; display: flex; align-items: center; justify-content: space-between; padding: 0 20px; }
h1 { font-size: 15px; font-weight: 700; margin: 0; }
.status { font-size: 11px; color: var(--muted); }
.error { color: var(--error) !important; }
.body { flex: 1; overflow: auto; padding: 0 20px 20px; display: grid; gap: 14px; align-content: start; }

.screen { position: relative; width: 72px; height: 46px; border-radius: 5px; background: var(--ctl); box-shadow: inset 0 0 0 0.5px var(--sep); flex: none; }
.corner { position: absolute; width: 22px; height: 12px; border: 0; padding: 0; border-radius: 3px; background: var(--track); cursor: pointer; }
.corner[aria-checked="true"] { background: var(--accent); }
.top-left { top: 4px; left: 4px; } .top-right { top: 4px; right: 4px; }
.bottom-left { bottom: 4px; left: 4px; } .bottom-right { bottom: 4px; right: 4px; }

.col { width: 44px; flex: none; display: flex; justify-content: center; }
.col.wide { width: 104px; }
.head { padding-top: 6px; padding-bottom: 6px; min-height: 0; }
.head .col { font-size: 11px; color: var(--muted); }
.legend { margin: -6px 2px 0; }

.about { justify-items: center; text-align: center; padding-top: 24px; gap: 6px; }
.about h2 { margin: 8px 0 0; font-size: 20px; }
.about p { margin: 0; }
.icon { width: 64px; height: 64px; border-radius: 15px; background: linear-gradient(160deg, #3a3a42, #121216);
  display: grid; place-items: center; box-shadow: 0 4px 14px rgba(0, 0, 0, 0.25); }
.icon span { width: 20px; height: 20px; border-radius: 50%; background: #fff; box-shadow: 0 0 0 9px rgba(255, 255, 255, 0.14); }
.brand.big { margin-top: 18px; font-size: 12px; }
.brand.big svg { width: 20px; height: 20px; }
</style>
