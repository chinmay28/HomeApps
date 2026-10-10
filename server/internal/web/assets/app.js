// DOM glue: polling, routing, search and opening apps. The rendering rules are
// in view.js.
//
// Opening an app: inside the Android shell, window.HomeAppsNative exists and
// takes over — the app opens full screen in its own WebView that stays warm
// for instant switching. In a browser the tile is an ordinary link to a new
// tab, so long-press and middle-click behave as they should.
import { allApps, appsHTML, hostsHTML, summary } from './view.js'

const POLL_MS = 15000
const STORE_KEY = 'homeapps.catalog'

const $ = (id) => document.getElementById(id)
const view = $('view')
const search = $('search')

const state = {
  catalog: readStored(),
  version: '',
  error: '',
}

function readStored() {
  try {
    return JSON.parse(localStorage.getItem(STORE_KEY)) || null
  } catch {
    return null
  }
}

function store(catalog) {
  try {
    localStorage.setItem(STORE_KEY, JSON.stringify(catalog))
  } catch {
    // Private mode or full storage: the launcher still works, just not offline.
  }
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

  // HomeApps itself unreachable beats HostMan unreachable; either way the
  // last list stays on screen.
  const banner = $('banner')
  const hostmanError = state.catalog?.hostman_error || ''
  const message = state.error
    ? `${state.error}${state.catalog ? ' Showing the last list we had.' : ''}`
    : hostmanError && `${hostmanError}${state.catalog?.hosts?.length ? ' Its apps are from the last list we had.' : ''}`
  banner.hidden = !message
  banner.textContent = message || ''

  if (!state.catalog) {
    $('summary').innerHTML = '<span class="dot dot--unknown"></span>Loading…'
    view.innerHTML = state.error ? '' : '<p class="empty">Loading…</p>'
    return
  }
  const s = summary(state.catalog)
  $('summary').innerHTML = `<span class="dot dot--${s.health}"></span>`
  $('summary').append(s.text)
  view.innerHTML = tab === 'hosts' ? hostsHTML(state.catalog, state.version, Boolean(window.HomeAppsNative?.changeServer)) : appsHTML(state.catalog, search.value)
}

async function refresh() {
  try {
    const res = await fetch('/api/catalog', { cache: 'no-store' })
    const body = await res.json().catch(() => ({}))
    if (!res.ok) throw new Error(body.error || `HomeApps answered ${res.status}.`)
    // HostMan down: HomeApps still answers, with its own apps and no hosts.
    // Keep showing the hosts we last had rather than emptying the screen.
    if (body.hostman_error && !body.hosts?.length && state.catalog?.hosts?.length) body.hosts = state.catalog.hosts
    state.catalog = body
    state.error = ''
    store(body)
  } catch (err) {
    state.error = err instanceof TypeError ? "Can't reach HomeApps." : String(err.message || err)
  }
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
  const app = allApps(state.catalog).find((a) => a.id === id)
  const native = window.HomeAppsNative
  if (!app || !native || typeof native.open !== 'function') return false
  // Positional strings: the shell's @JavascriptInterface takes them as-is,
  // with no JSON to parse on the other side. It validates every one.
  native.open(app.id, app.name, app.url, app.color || '', app.icon ? new URL(app.icon, location.href).href : '')
  return true
}

// --- Adding, editing and removing apps by address ---------------------------

const dialog = $('app-dialog')
const form = $('app-form')
const urlInput = $('app-url')
const nameInput = $('app-name')
const errorText = $('app-error')
const saveButton = $('app-save')
const removeButton = $('app-remove')
let editing = null // the app being edited, or null when adding
let confirmRemove = false

function showError(message) {
  errorText.textContent = message || ''
  errorText.hidden = !message
}

function openDialog(app) {
  editing = app || null
  confirmRemove = false
  $('app-dialog-title').textContent = app ? 'Edit app' : 'Add an app'
  saveButton.textContent = app ? 'Save' : 'Add'
  urlInput.value = app ? app.url : ''
  nameInput.value = app ? app.name : ''
  removeButton.hidden = !app
  removeButton.textContent = 'Remove'
  showError('')
  setBusy(false)
  dialog.showModal()
  urlInput.focus()
}

function setBusy(busy) {
  for (const el of [saveButton, removeButton, urlInput, nameInput]) el.disabled = busy
}

async function call(method, path, body) {
  const res = await fetch(path, {
    method,
    headers: body ? { 'Content-Type': 'application/json' } : {},
    body: body ? JSON.stringify(body) : undefined,
  })
  if (res.ok) return res.status === 204 ? null : res.json()
  const err = await res.json().catch(() => ({}))
  throw new Error(err.error || `HomeApps answered ${res.status}.`)
}

form.addEventListener('submit', async (e) => {
  e.preventDefault()
  const body = { url: urlInput.value, name: nameInput.value }
  if (!body.url.trim()) return showError("Enter the app's address, for example pi5.local:8123.")
  setBusy(true)
  showError('')
  try {
    if (editing) await call('PATCH', `/api/apps/${encodeURIComponent(editing.id)}`, body)
    else await call('POST', '/api/apps', body)
    dialog.close()
    await refresh()
  } catch (err) {
    showError(err instanceof TypeError ? "Can't reach HomeApps." : err.message)
    setBusy(false)
  }
})

// Two taps: the first asks, the second removes. No confirm(): the Android
// shell's launcher WebView has no dialog handler, so it would return false.
removeButton.addEventListener('click', async () => {
  if (!editing) return
  if (!confirmRemove) {
    confirmRemove = true
    removeButton.textContent = 'Tap again to remove'
    return
  }
  setBusy(true)
  try {
    await call('DELETE', `/api/apps/${encodeURIComponent(editing.id)}`)
    dialog.close()
    await refresh()
  } catch (err) {
    showError(err.message)
    setBusy(false)
  }
})

$('app-cancel').addEventListener('click', () => dialog.close())
$('add-app').addEventListener('click', () => openDialog(null))

view.addEventListener('click', (e) => {
  const action = e.target.closest('[data-action]')?.dataset.action
  if (action === 'add-app') {
    e.preventDefault()
    openDialog(null)
    return
  }
  if (action === 'edit-app') {
    const id = e.target.closest('[data-id]').dataset.id
    openDialog((state.catalog?.my_apps || []).find((a) => a.id === id))
    return
  }
  if (e.target.closest('[data-action="change-server"]')) {
    e.preventDefault()
    window.HomeAppsNative?.changeServer?.()
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
