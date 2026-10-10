// Pure view logic: catalog in, HTML strings and summaries out. No DOM, no
// fetch, so it runs under `node --test` (web/view.test.mjs). Every value from
// the catalog goes through esc() — names and URLs come from HostMan, from
// whoever added a custom app, and from other apps' manifests, not from us.
//
// The catalog here is /api/catalog's body plus `custom`: the apps from
// /api/custom-apps, which have no host and no health.

const HEX = /^#(?:[0-9a-f]{3}|[0-9a-f]{6})$/i

export function esc(s) {
  return String(s ?? '').replace(/[&<>"']/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[c])
}

// Two letters for a tile without an icon: the initials of the first two
// words, where a camel-case word counts as several ("Thought Mesh" -> "TM",
// "CountRoster" -> "CR"), else the first two letters ("tickers" -> "Ti").
export function monogram(name) {
  let words = String(name || '?').trim().split(/[\s_-]+/).filter(Boolean)
  if (words.length === 1) words = words[0].split(/(?<=[a-z0-9])(?=[A-Z])/)
  if (words.length >= 2) return (words[0][0] + words[1][0]).toUpperCase()
  const w = words[0] || '?'
  return w[0].toUpperCase() + (w[1] || '').toLowerCase()
}

// A stable hue per name, so a monogram tile keeps its colour across reloads.
export function hue(name) {
  let h = 0
  for (const ch of String(name)) h = (h * 31 + ch.codePointAt(0)) >>> 0
  return h % 360
}

// The heading of the custom apps' section and card.
export const CUSTOM_TITLE = 'My apps'

function hostedApps(catalog) {
  return (catalog?.hosts || []).flatMap((h) => h.apps.map((a) => ({ ...a, host: h.name })))
}

export function allApps(catalog) {
  return [...(catalog?.custom || []).map((a) => ({ ...a, host: '' })), ...hostedApps(catalog)]
}

function plural(n, word) {
  return `${n} ${n === 1 ? word : word + 's'}`
}

// The line under the title: how much there is and whether anything is wrong.
// Health is HostMan's; custom apps are counted but have none.
export function summary(catalog) {
  const apps = hostedApps(catalog)
  const custom = (catalog?.custom || []).length
  const hosts = (catalog?.hosts || []).filter((h) => h.apps.length > 0).length
  const down = apps.filter((a) => a.health === 'down')
  if (apps.length === 0) {
    return { text: custom ? plural(custom, 'custom app') : 'No apps yet', health: 'unknown' }
  }
  const base = `${plural(apps.length, 'app')} on ${plural(hosts, 'host')}${custom ? ` + ${custom} custom` : ''}`
  if (down.length === 1) return { text: `${base} · ${down[0].name} not responding`, health: 'down' }
  if (down.length > 1) return { text: `${base} · ${down.length} not responding`, health: 'down' }
  if (apps.every((a) => a.health === 'up')) return { text: `${base} · all healthy`, health: 'up' }
  return { text: base, health: 'unknown' }
}

// Search matches app or host names, every word anywhere, case-insensitively,
// so "pi5 va" finds SAND Vault on pi5.
// A custom app matches on its name and its address's host.
export function filter(catalog, query) {
  const words = String(query || '').toLowerCase().split(/\s+/).filter(Boolean)
  const matches = (hay) => words.every((w) => hay.toLowerCase().includes(w))
  const hosts = (catalog?.hosts || []).map((h) => ({
    ...h,
    apps: h.apps.filter((a) => matches(`${a.name} ${h.name}`)),
  }))
  const custom = (catalog?.custom || []).filter((a) => matches(`${a.name} ${hostOf(a.url)}`))
  return { ...catalog, hosts: hosts.filter((h) => h.apps.length > 0), custom }
}

function hostOf(url) {
  try {
    return new URL(url).host
  } catch {
    return ''
  }
}

// What someone typed into the address field, made into the absolute URL the
// server wants: "pi5.local:8080" means http://pi5.local:8080, the way a
// browser's address bar reads it. Anything with a scheme ("https://…",
// "javascript:…", but not the "host:port" of a bare address) is left for the
// server to accept or refuse.
export function normalizeAddress(input) {
  const s = String(input || '').trim()
  if (s === '' || /^[a-z][a-z0-9+.-]*:(?!\d)/i.test(s)) return s
  return 'http://' + s.replace(/^\/+/, '')
}

function iconHTML(app, small) {
  const cls = small ? ' icon--sm' : ''
  const dot = app.health ? `<span class="dot dot--${esc(app.health)}"></span>` : ''
  if (app.icon) {
    const bg = HEX.test(app.color || '') ? ` style="--tile-bg:${app.color}"` : ''
    return `<span class="icon icon--img${cls}"${bg}><img src="${esc(app.icon)}" alt="" loading="lazy">${dot}</span>`
  }
  return `<span class="icon${cls}" style="--hue:${hue(app.name)}" aria-hidden="true">${esc(monogram(app.name))}${dot}</span>`
}

const healthWord = { up: 'healthy', down: 'not responding', unknown: 'status unknown' }

function tileHTML(app) {
  const label = app.health ? `${app.name}, ${healthWord[app.health] || healthWord.unknown}` : app.name
  return `<a class="tile" href="${esc(app.url)}" data-app="${esc(app.id)}" target="_blank" rel="noopener"
    aria-label="${esc(label)}">${iconHTML(app, false)}<span class="tile__name">${esc(app.name)}</span></a>`
}

const addButton = (label) => `<button type="button" class="link" data-action="add-app">${label}</button>`

function customSectionHTML(apps) {
  if (apps.length === 0) return ''
  return `<section class="section"><div class="section__head"><h2>${CUSTOM_TITLE}</h2>${addButton('Add')}</div>
    <div class="grid">${apps.map(tileHTML).join('')}</div></section>`
}

export function appsHTML(catalog, query) {
  const shown = filter(catalog, query)
  if (shown.hosts.length === 0 && shown.custom.length === 0) {
    return query
      ? `<p class="empty">Nothing matches “${esc(query)}”.</p>`
      : `<p class="empty">No apps with an address to open yet.<br>Deploy one in ${hostmanLink(catalog, 'HostMan')} and it shows up here,<br>or ${addButton('add any web address')} yourself.</p>`
  }
  return customSectionHTML(shown.custom) + shown.hosts.map((h) => {
    const up = h.apps.filter((a) => a.health === 'up').length
    const countCls = up === h.apps.length ? ' section__count--ok' : h.apps.some((a) => a.health === 'down') ? ' section__count--bad' : ''
    return `<section class="section"><div class="section__head"><h2>${esc(h.name)}</h2>
      <span class="section__count${countCls}">${up} of ${h.apps.length} healthy</span></div>
      <div class="grid">${h.apps.map(tileHTML).join('')}</div></section>`
  }).join('') + (query ? '' : `<p class="about">To add an app, deploy it to a host in ${hostmanLink(catalog, 'HostMan')}.<br>It appears here once it has a port.<br>Or ${addButton('add any web address')}.</p>`)
}

// HostMan's own tile, if HostMan manages itself: the one app on the port
// HomeApps reads it from. Its URL is the address a phone can reach, unlike the
// source URL, which is often 127.0.0.1 because HomeApps runs beside HostMan.
export function hostmanApp(catalog) {
  const src = catalog?.source?.url
  if (!src) return null
  let port
  try {
    port = Number(new URL(src).port || 80)
  } catch {
    return null
  }
  const apps = allApps(catalog).filter((a) => a.port === port)
  return apps.find((a) => /hostman/i.test(a.name)) || apps[0] || null
}

// Where new apps come from, as a link that opens HostMan the way any app
// opens (in the shell or a new tab), or plain words when there is no tile.
function hostmanLink(catalog, label) {
  const hm = hostmanApp(catalog)
  if (!hm) return esc(label)
  return `<a href="${esc(hm.url)}" data-app="${esc(hm.id)}" target="_blank" rel="noopener">${esc(label)}</a>`
}

function pct(v) {
  return v == null ? '–' : `${Math.round(v)}%`
}

// The custom apps as rows, each with its own Edit button: the one place they
// are changed or removed.
function customCardHTML(apps) {
  if (apps.length === 0) return ''
  const rows = apps.map((a) => `<div class="row"><a class="row__link" href="${esc(a.url)}" data-app="${esc(a.id)}" target="_blank" rel="noopener">${iconHTML(a, true)}
      <span class="row__text"><span class="row__name">${esc(a.name)}</span><span class="row__sub">${esc(a.url)}</span></span></a>
      <button type="button" class="link" data-action="edit-app" data-id="${esc(a.id)}">Edit</button></div>`).join('')
  return `<section class="card"><div class="card__head"><div><h2>${CUSTOM_TITLE}</h2>
    <span class="card__meta">added by hand</span></div>${addButton('Add')}</div>${rows}</section>`
}

export function hostsHTML(catalog, version, canChangeServer = false) {
  const hosts = catalog?.hosts || []
  const custom = customCardHTML(catalog?.custom || [])
  const cards = hosts.map((h) => {
    const status = h.online ? 'Online' : 'Offline'
    const rows = h.apps.map((a) => {
      const sub = a.health === 'down' ? (a.detail || 'Not responding') : [healthWord[a.health], a.version].filter(Boolean).join(' · ')
      return `<a class="row" href="${esc(a.url)}" data-app="${esc(a.id)}" target="_blank" rel="noopener">${iconHTML(a, true)}
        <span class="row__text"><span class="row__name">${esc(a.name)}</span><span class="row__sub${a.health === 'down' ? ' row__sub--down' : ''}">${esc(sub)}</span></span>
        <span class="row__port">${a.port ? ':' + esc(a.port) : ''}</span><span class="dot dot--${esc(a.health)}"></span></a>`
    }).join('')
    return `<section class="card"><div class="card__head"><div><h2>${esc(h.name)}</h2>
      <span class="card__meta">${esc(h.address)} · CPU ${pct(h.cpu_pct)} · Mem ${pct(h.mem_pct)}</span></div>
      <span class="chip">${status}</span></div>${rows}</section>`
  }).join('')
  const src = catalog?.source?.url
  const about = `<p class="about">HomeApps ${esc(version || '')}${src ? `<br>apps from ${hostmanLink(catalog, 'HostMan')}` : ''}${canChangeServer ? '<br><a href="#" data-action="change-server">Change HomeApps server</a>' : ''}</p>`
  return custom + (cards || '<p class="empty">No hosts.</p>') + about
}
