// DOM glue: polling, routing, search, opening apps and the add/edit dialog for
// custom apps. The rendering rules are in view.js.
//
// Opening an app: inside the Android shell, window.HomeAppsNative exists and
// takes over — the app opens full screen in its own WebView that stays warm
// for instant switching. In a browser the tile is an ordinary link to a new
// tab, so long-press and middle-click behave as they should.
import { allApps, appsHTML, hostsHTML, iconHTML, iconRequest, normalizeAddress, summary } from './view.js'

const POLL_MS = 15000
// After a save, look again soon: the server fetches a new address's own icon
// in the background, and it is usually there within a few seconds.
const SOON_MS = 3000
// A picked picture is shrunk to this many pixels square before it is sent.
const ICON_PX = 192
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
  editingApps: false, // My apps on the Apps page is in edit mode
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
  if (m.custom.length === 0) state.editingApps = false
  view.innerHTML = tab === 'hosts' ? hostsHTML(m, state.version, Boolean(window.HomeAppsNative?.changeServer)) : appsHTML(m, search.value, state.editingApps)
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
const iconFile = form.querySelector('[data-action="icon-file"]')
const iconAuto = form.querySelector('[data-action="icon-auto"]')
let editing = null // the custom app being edited, or null when adding
let iconChange = null // what iconRequest() takes; null leaves the icon alone

function openDialog(app) {
  editing = app || null
  iconChange = null
  form.reset()
  form.elements.url.value = app?.url || ''
  form.elements.name.value = app?.name || ''
  $('app-dialog-title').textContent = app ? 'Edit app' : 'Add an app'
  $('app-submit').textContent = app ? 'Save' : 'Add'
  removeBtn.hidden = !app
  removeBtn.textContent = 'Remove'
  removeBtn.dataset.armed = ''
  showError('')
  drawIcon()
  dialog.showModal()
  form.elements.url.focus()
}

// drawIcon shows the icon the app will have once saved, as far as the page
// can know it: an address's picture is fetched by the server, not here.
function drawIcon() {
  const name = form.elements.name.value.trim() || editing?.name || '?'
  let icon = ''
  let hint = ''
  switch (iconChange?.kind) {
    case 'image':
      icon = iconChange.dataURL
      break
    case 'url':
      hint = 'HomeApps fetches this picture when you save.'
      break
    case 'auto':
      if (editing?.custom_icon) hint = "The app's own icon, if it has one, shows after saving."
      else icon = editing?.icon || ''
      break
    default:
      icon = editing?.icon || ''
      if (!editing) hint = "Found on the app's page if left alone."
  }
  $('app-icon-preview').innerHTML = iconHTML({ name, icon, color: editing?.color }, true)
  $('app-icon-hint').textContent = hint
  iconAuto.hidden = !(editing?.custom_icon || iconChange?.kind === 'image' || iconChange?.kind === 'url')
}

function showError(msg) {
  $('app-error').hidden = !msg
  $('app-error').textContent = msg
}

// run does one change with the dialog's buttons disabled, then closes the
// dialog and redraws; on failure the dialog stays open with the server's words.
async function run(change) {
  for (const b of form.querySelectorAll('button')) b.disabled = true
  try {
    await change()
    dialog.close()
    await refresh()
    setTimeout(refresh, SOON_MS)
  } catch (err) {
    showError(describe(err))
  } finally {
    for (const b of form.querySelectorAll('button')) b.disabled = false
  }
}

const json = (method, body) => ({ method, headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) })

form.addEventListener('submit', (e) => {
  e.preventDefault()
  const url = normalizeAddress(form.elements.url.value)
  if (!url) {
    showError('Type the address the app opens at.')
    return
  }
  const draft = { name: form.elements.name.value, url }
  run(async () => {
    const app = editing
      ? await api(`/api/custom-apps/${encodeURIComponent(editing.id)}`, json('PUT', draft))
      : await api('/api/custom-apps', json('POST', draft))
    // From here the app exists: if the icon fails, a retry edits it rather
    // than adding it twice.
    editing = app
    $('app-dialog-title').textContent = 'Edit app'
    $('app-submit').textContent = 'Save'
    removeBtn.hidden = false
    const req = iconRequest(iconChange)
    if (req) {
      const path = `/api/custom-apps/${encodeURIComponent(app.id)}/icon`
      await api(path, req.body ? json(req.method, req.body) : { method: req.method })
    }
  })
})

form.elements.name.addEventListener('input', drawIcon)

form.elements.icon_url.addEventListener('input', () => {
  const url = normalizeAddress(form.elements.icon_url.value)
  if (url) iconChange = { kind: 'url', url }
  else if (iconChange?.kind === 'url') iconChange = null
  iconFile.value = ''
  drawIcon()
})

iconFile.addEventListener('change', async () => {
  const file = iconFile.files?.[0]
  if (!file) return
  try {
    iconChange = { kind: 'image', dataURL: await shrink(file) }
    form.elements.icon_url.value = ''
    showError('')
  } catch (err) {
    showError(describe(err))
  }
  drawIcon()
})

iconAuto.addEventListener('click', () => {
  iconChange = editing?.custom_icon ? { kind: 'auto' } : null
  form.elements.icon_url.value = ''
  iconFile.value = ''
  drawIcon()
})

// shrink reads a picked picture and redraws it as an ICON_PX square PNG, the
// whole picture centred: a few KiB to send instead of a phone photo's MiB.
// It goes through a data: URL because the page's CSP allows those, not blob:.
function shrink(file) {
  return new Promise((resolve, reject) => {
    const unusable = () => reject(new Error("That file isn't a picture HomeApps can use."))
    const reader = new FileReader()
    reader.onerror = unusable
    reader.onload = () => {
      const img = new Image()
      img.onerror = unusable
      img.onload = () => {
        const w = img.naturalWidth || ICON_PX
        const h = img.naturalHeight || ICON_PX
        const scale = ICON_PX / Math.max(w, h)
        const canvas = document.createElement('canvas')
        canvas.width = canvas.height = ICON_PX
        canvas.getContext('2d').drawImage(img, (ICON_PX - w * scale) / 2, (ICON_PX - h * scale) / 2, w * scale, h * scale)
        resolve(canvas.toDataURL('image/png'))
      }
      img.src = reader.result
    }
    reader.readAsDataURL(file)
  })
}

// Remove asks twice, in place: a WebView without a chrome client has no confirm().
removeBtn.addEventListener('click', () => {
  if (!editing) return
  if (!removeBtn.dataset.armed) {
    removeBtn.dataset.armed = '1'
    removeBtn.textContent = 'Remove for everyone?'
    return
  }
  const id = editing.id
  run(() => api(`/api/custom-apps/${encodeURIComponent(id)}`, { method: 'DELETE' }))
})

form.querySelector('[data-action="cancel"]').addEventListener('click', () => dialog.close())

view.addEventListener('click', (e) => {
  if (e.target.closest('[data-action="change-server"]')) {
    e.preventDefault()
    window.HomeAppsNative?.changeServer?.()
    return
  }
  if (e.target.closest('[data-action="toggle-edit"]')) {
    state.editingApps = !state.editingApps
    render()
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
