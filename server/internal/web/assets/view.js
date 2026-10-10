// Pure view logic: catalog in, HTML strings and summaries out. No DOM, no
// fetch, so it runs under `node --test` (web/view.test.mjs). Every value from
// the catalog goes through esc(): names and URLs come from HostMan, from what
// people typed, and from other apps' manifests.

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

// The launcher's sections: the apps added by URL first ("My apps"), then
// each of HostMan's hosts. A host is a group; so is the list of added apps.
export function groups(catalog) {
  const mine = catalog?.my_apps || []
  const out = mine.length ? [{ key: 'mine', name: 'My apps', mine: true, apps: mine }] : []
  return out.concat((catalog?.hosts || []).map((h) => ({ ...h, key: `host-${h.id}`, mine: false })))
}

export function allApps(catalog) {
  return groups(catalog).flatMap((g) => g.apps.map((a) => ({ ...a, host: g.mine ? '' : g.name })))
}

const plural = (n, one, many) => `${n} ${n === 1 ? one : many}`

// The line under the title: how much there is and whether anything is wrong.
export function summary(catalog) {
  const apps = allApps(catalog)
  const hosts = (catalog?.hosts || []).filter((h) => h.apps.length > 0).length
  const down = apps.filter((a) => a.health === 'down')
  const base = plural(apps.length, 'app', 'apps') + (hosts ? ` · ${plural(hosts, 'host', 'hosts')}` : '')
  if (apps.length === 0) return { text: 'No apps yet', health: 'unknown' }
  if (down.length === 1) return { text: `${base} · ${down[0].name} not responding`, health: 'down' }
  if (down.length > 1) return { text: `${base} · ${down.length} not responding`, health: 'down' }
  if (apps.every((a) => a.health === 'up')) return { text: `${base} · all healthy`, health: 'up' }
  return { text: base, health: 'unknown' }
}

// Search matches app or host names, every word anywhere, case-insensitively,
// so "pi5 va" finds SAND Vault on pi5.
// Returns the groups with at least one match.
export function filter(catalog, query) {
  const words = String(query || '').toLowerCase().split(/\s+/).filter(Boolean)
  return groups(catalog)
    .map((g) => ({
      ...g,
      apps: g.apps.filter((a) => {
        const hay = `${a.name} ${g.name}`.toLowerCase()
        return words.every((w) => hay.includes(w))
      }),
    }))
    .filter((g) => g.apps.length > 0)
}

function iconHTML(app, small) {
  const cls = small ? ' icon--sm' : ''
  const dot = `<span class="dot dot--${esc(app.health)}"></span>`
  if (app.icon) {
    const bg = HEX.test(app.color || '') ? ` style="--tile-bg:${app.color}"` : ''
    return `<span class="icon icon--img${cls}"${bg}><img src="${esc(app.icon)}" alt="" loading="lazy">${dot}</span>`
  }
  return `<span class="icon${cls}" style="--hue:${hue(app.name)}" aria-hidden="true">${esc(monogram(app.name))}${dot}</span>`
}

const healthWord = { up: 'healthy', down: 'not responding', unknown: 'status unknown' }

function tileHTML(app) {
  return `<a class="tile" href="${esc(app.url)}" data-app="${esc(app.id)}" target="_blank" rel="noopener"
    aria-label="${esc(app.name)}, ${healthWord[app.health] || healthWord.unknown}">${iconHTML(app, false)}<span class="tile__name">${esc(app.name)}</span></a>`
}

const addButton = '<button type="button" class="button" data-action="add-app">Add an app</button>'

export function appsHTML(catalog, query) {
  const shown = filter(catalog, query)
  if (shown.length === 0) {
    if (query) return `<p class="empty">Nothing matches “${esc(query)}”.</p>`
    const fromHostMan = catalog?.source ? `<br>Or deploy one in ${hostmanLink(catalog, 'HostMan')} and it shows up here.` : ''
    return `<div class="empty">No apps yet. Add one by its address, like <code>pi5.local:8123</code>.${fromHostMan}<p>${addButton}</p></div>`
  }
  const sections = shown.map((g) => {
    const up = g.apps.filter((a) => a.health === 'up').length
    const countCls = up === g.apps.length ? ' section__count--ok' : g.apps.some((a) => a.health === 'down') ? ' section__count--bad' : ''
    return `<section class="section"><div class="section__head"><h2>${esc(g.name)}</h2>
      <span class="section__count${countCls}">${up} of ${g.apps.length} healthy</span></div>
      <div class="grid">${g.apps.map(tileHTML).join('')}</div></section>`
  }).join('')
  if (query) return sections
  const viaHostMan = catalog?.source ? `, or deploy one to a host in ${hostmanLink(catalog, 'HostMan')}` : ''
  return sections + `<p class="about">Tap <a href="#" data-action="add-app">+</a> to add an app by its address${viaHostMan}.</p>`
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

// The added apps, each with an Edit button (removal lives in the edit dialog,
// behind a second tap, so a slip can't delete one).
function myAppsCardHTML(catalog) {
  const mine = catalog?.my_apps || []
  const rows = mine.map((a) => `<div class="row row--mine">
      <a class="row__open" href="${esc(a.url)}" data-app="${esc(a.id)}" target="_blank" rel="noopener">${iconHTML(a, true)}
        <span class="row__text"><span class="row__name">${esc(a.name)}</span><span class="row__sub${a.health === 'down' ? ' row__sub--down' : ''}">${esc(a.url)}</span></span>
        <span class="dot dot--${esc(a.health)}"></span></a>
      <button type="button" class="row__edit" data-action="edit-app" data-id="${esc(a.id)}" aria-label="Edit ${esc(a.name)}">Edit</button>
    </div>`).join('')
  return `<section class="card"><div class="card__head"><div><h2>My apps</h2>
    <span class="card__meta">added by address</span></div>
    <button type="button" class="chip chip--button" data-action="add-app">Add</button></div>${rows || '<p class="card__empty">None yet.</p>'}</section>`
}

export function hostsHTML(catalog, version, canChangeServer = false) {
  const hosts = catalog?.hosts || []
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
  return myAppsCardHTML(catalog) + cards + about
}
