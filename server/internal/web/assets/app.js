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

  const banner = $('banner')
  banner.hidden = !state.error
  banner.textContent = state.error ? `${state.error}${state.catalog ? ' Showing the last list we had.' : ''}` : ''

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

view.addEventListener('click', (e) => {
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
