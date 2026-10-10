// DOM glue: polling, routing, search, opening apps and the add/edit dialog for
// custom apps. The rendering rules are in view.js.
//
// Opening an app: inside the Android shell, window.HomeAppsNative exists and
// takes over — the app opens full screen in its own WebView that stays warm
// for instant switching. In a browser the tile is an ordinary link to a new
// tab, so long-press and middle-click behave as they should.
import { allApps, appsHTML, hostsHTML, normalizeAddress, summary } from './view.js'

const POLL_MS = 15000
const STORE_KEY = 'homeapps.catalog'
const CUSTOM_KEY = 'homeapps.custom'

const $ = (id) => document.getElementById(id)
const view = $('view')
const search = $('search')

// catalog and custom load separately: custom apps don't come from HostMan, so
// they show, and open, while HostMan is down.
const state = {
  catalog: readStored(STORE_KEY),
  custom: readStored(CUSTOM_KEY),
  version: '',
  error: '',
}

function readStored(key) {
  try {
    return JSON.parse(localStorage.getItem(key)) || null
  } catch {
    return null
  }
}

function store(key, value) {
  try {
    localStorage.setItem(key, JSON.stringify(value))
  } catch {
    // Private mode or full storage: the launcher still works, just not offline.
  }
}

// What view.js draws: the catalog with the custom apps beside it.
function model() {
  return { hosts: [], ...state.catalog, custom: state.custom || [] }
}

function route() {
  return location.hash === '#/hosts' ? 'hosts' : 'apps'
}

function render() {
  const tab = route()
  for (const a of document.querySelectorAll('.tabs__tab')) {
    if (a.dataset.tab === tab) a.setAttribute('aria-current', 'page')
    else a.removeAttribute('aria-current')
  }
  $('title').textContent = tab === 'hosts' ? 'Hosts' : 'Home'
  $('search-wrap').hidden = tab !== 'apps'

  const banner = $('banner')
  banner.hidden = !state.error
  banner.textContent = state.error ? `${state.error}${state.catalog ? ' Showing the last list we had.' : ''}` : ''

  if (!state.catalog && !state.custom) {
    $('summary').innerHTML = '<span class="dot dot--unknown"></span>Loading…'
    view.innerHTML = state.error ? '' : '<p class="empty">Loading…</p>'
    return
  }
  const m = model()
  const s = summary(m)
  $('summary').innerHTML = `<span class="dot dot--${s.health}"></span>`
  $('summary').append(s.text)
  view.innerHTML = tab === 'hosts' ? hostsHTML(m, state.version, Boolean(window.HomeAppsNative?.changeServer)) : appsHTML(m, search.value)
}

// api returns the body of a 2xx answer and throws the server's own words
// otherwise.
async function api(path, init) {
  const res = await fetch(path, { cache: 'no-store', ...init })
  const body = res.status === 204 ? null : await res.json().catch(() => ({}))
  if (!res.ok) throw new Error(body?.error || `HomeApps answered ${res.status}.`)
  return body
}

function describe(err) {
  return err instanceof TypeError ? "Can't reach HomeApps." : String(err.message || err)
}

async function refresh() {
  const [catalog, custom] = await Promise.allSettled([api('/api/catalog'), api('/api/custom-apps')])
  if (catalog.status === 'fulfilled') {
    state.catalog = catalog.value
    store(STORE_KEY, catalog.value)
  }
  if (custom.status === 'fulfilled') {
    state.custom = custom.value.apps
    store(CUSTOM_KEY, custom.value.apps)
  }
  const failed = [catalog, custom].find((r) => r.status === 'rejected')
  state.error = failed ? describe(failed.reason) : ''
  render()
}

async function loadVersion() {
  try {
    state.version = (await (await fetch('/api/health')).json()).version || ''
  } catch {
    // Shown on the Hosts page only; leaving it blank is fine.
  }
}

function openApp(id) {
  const app = allApps(model()).find((a) => a.id === id)
  const native = window.HomeAppsNative
  if (!app || !native || typeof native.open !== 'function') return false
  // Positional strings: the shell's @JavascriptInterface takes them as-is,
  // with no JSON to parse on the other side. It validates every one.
  native.open(app.id, app.name, app.url, app.color || '', app.icon ? new URL(app.icon, location.href).href : '')
  return true
}

// The add/edit dialog. It lives outside #view, so a poll redrawing the list
// never wipes what someone is typing.
const dialog = $('app-dialog')
const form = $('app-form')
const removeBtn = form.querySelector('[data-action="remove"]')
let editing = null // the custom app being edited, or null when adding

function openDialog(app) {
  editing = app || null
  form.reset()
  form.elements.url.value = app?.url || ''
  form.elements.name.value = app?.name || ''
  $('app-dialog-title').textContent = app ? 'Edit app' : 'Add an app'
  $('app-submit').textContent = app ? 'Save' : 'Add'
  removeBtn.hidden = !app
  removeBtn.textContent = 'Remove'
  removeBtn.dataset.armed = ''
  showError('')
  dialog.showModal()
  form.elements.url.focus()
}

function showError(msg) {
  $('app-error').hidden = !msg
  $('app-error').textContent = msg
}

async function save(path, init) {
  for (const b of form.querySelectorAll('button')) b.disabled = true
  try {
    await api(path, init)
    dialog.close()
    await refresh()
  } catch (err) {
    showError(describe(err))
  } finally {
    for (const b of form.querySelectorAll('button')) b.disabled = false
  }
}

form.addEventListener('submit', (e) => {
  e.preventDefault()
  const url = normalizeAddress(form.elements.url.value)
  if (!url) {
    showError('Type the address the app opens at.')
    return
  }
  const body = JSON.stringify({ name: form.elements.name.value, url })
  const init = { method: editing ? 'PUT' : 'POST', headers: { 'Content-Type': 'application/json' }, body }
  save(editing ? `/api/custom-apps/${encodeURIComponent(editing.id)}` : '/api/custom-apps', init)
})

// Remove asks twice, in place: a WebView without a chrome client has no confirm().
removeBtn.addEventListener('click', () => {
  if (!editing) return
  if (!removeBtn.dataset.armed) {
    removeBtn.dataset.armed = '1'
    removeBtn.textContent = 'Remove for everyone?'
    return
  }
  save(`/api/custom-apps/${encodeURIComponent(editing.id)}`, { method: 'DELETE' })
})

form.querySelector('[data-action="cancel"]').addEventListener('click', () => dialog.close())

view.addEventListener('click', (e) => {
  if (e.target.closest('[data-action="change-server"]')) {
    e.preventDefault()
    window.HomeAppsNative?.changeServer?.()
    return
  }
  if (e.target.closest('[data-action="add-app"]')) {
    openDialog(null)
    return
  }
  const edit = e.target.closest('[data-action="edit-app"]')
  if (edit) {
    const app = (state.custom || []).find((a) => a.id === edit.dataset.id)
    if (app) openDialog(app)
    return
  }
  const link = e.target.closest('a[data-app]')
  if (link && !e.metaKey && !e.ctrlKey && !e.shiftKey && openApp(link.dataset.app)) e.preventDefault()
})

search.addEventListener('input', render)
search.addEventListener('keydown', (e) => {
  if (e.key !== 'Enter') return
  const first = view.querySelector('a[data-app]')
  if (first) first.click()
})

window.addEventListener('hashchange', () => {
  render()
  view.scrollTop = 0
})

// Poll while visible; refresh at once when the launcher comes back to the
// front, which is exactly when someone is about to look at it.
let timer = 0
function schedule() {
  clearInterval(timer)
  if (document.visibilityState === 'visible') timer = setInterval(refresh, POLL_MS)
}
document.addEventListener('visibilitychange', () => {
  if (document.visibilityState === 'visible') refresh()
  schedule()
})

if ('serviceWorker' in navigator && window.isSecureContext && !window.HomeAppsNative) {
  navigator.serviceWorker.register('/sw.js').catch(() => {})
}

render()
loadVersion().then(render)
refresh()
schedule()
