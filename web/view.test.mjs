// Tests for the web client's pure view logic: `node --test web/`.
import { test } from 'node:test'
import assert from 'node:assert/strict'
import { allApps, appsHTML, esc, filter, groups, hostmanApp, hostsHTML, hue, monogram, summary } from '../server/internal/web/assets/view.js'

const catalog = {
  hosts: [
    { id: 1, name: 'nakedpi', address: 'nakedpi.local', online: true, cpu_pct: 6.4, mem_pct: null, apps: [
      { id: 'hm-1', name: 'CountRoster', url: 'http://nakedpi.local:8787/', port: 8787, health: 'up', icon: '/api/icons/ab', color: '#5eead4' },
      { id: 'hm-2', name: 'Thought Mesh', url: 'http://nakedpi.local:8881/', port: 8881, health: 'up', icon: '', color: '' },
    ] },
    { id: 2, name: 'pi5', address: 'pi5.local', online: true, cpu_pct: null, mem_pct: null, apps: [
      { id: 'hm-3', name: 'SAND Vault', url: 'http://pi5.local:8123/', port: 8123, health: 'down', detail: 'connection refused', icon: '', color: '' },
    ] },
    { id: 3, name: 'empty', address: 'e.local', online: false, apps: [] },
  ],
  source: { kind: 'hostman', url: 'http://nakedpi.local:8899' },
}

test('monogram', () => {
  assert.equal(monogram('Thought Mesh'), 'TM')
  assert.equal(monogram('CountRoster'), 'CR')
  assert.equal(monogram('HostMan'), 'HM')
  assert.equal(monogram('BIP39 Explorer'), 'BE')
  assert.equal(monogram('tickers'), 'Ti')
  assert.equal(monogram(''), '?')
})

test('hue is stable and in range', () => {
  assert.equal(hue('SAND Vault'), hue('SAND Vault'))
  assert.ok(hue('x') >= 0 && hue('x') < 360)
})

test('summary names a single down app and counts only hosts with apps', () => {
  assert.deepEqual(summary(catalog), { text: '3 apps · 2 hosts · SAND Vault not responding', health: 'down' })
  const healthy = { hosts: [{ name: 'a', apps: [{ name: 'x', health: 'up' }] }] }
  assert.deepEqual(summary(healthy), { text: '1 app · 1 host · all healthy', health: 'up' })
  assert.equal(summary({ hosts: [] }).text, 'No apps yet')
})

test('filter matches every word against app and host name', () => {
  const names = (gs) => gs.flatMap((g) => g.apps.map((a) => a.name))
  assert.deepEqual(names(filter(catalog, 'pi5 va')), ['SAND Vault'])
  assert.deepEqual(names(filter(catalog, 'MESH')), ['Thought Mesh'])
  assert.deepEqual(names(filter(catalog, '')), ['CountRoster', 'Thought Mesh', 'SAND Vault'])
  assert.equal(filter(catalog, 'nothing').length, 0)
})

test('apps view: real icon when known, monogram otherwise, health per host', () => {
  const html = appsHTML(catalog, '')
  assert.match(html, /<img src="\/api\/icons\/ab"/)
  assert.match(html, /--tile-bg:#5eead4/)
  assert.match(html, />TM<span class="dot dot--up">/)
  assert.match(html, /0 of 1 healthy/)
  assert.doesNotMatch(html, /empty/, 'a host with no apps has no section in the grid')
  assert.match(appsHTML(catalog, 'zzz'), /Nothing matches “zzz”/)
})

test('everything from the catalog is escaped', () => {
  const evil = { hosts: [{ name: '<b>h</b>', apps: [{ id: '"x', name: '<img src=x onerror=alert(1)>', url: 'javascript:"', health: 'up', color: 'red;background:url(x)' }] }] }
  const html = appsHTML(evil, '')
  assert.doesNotMatch(html, /<img src=x/)
  assert.doesNotMatch(html, /<b>h<\/b>/)
  assert.doesNotMatch(html, /red;background/, 'a non-hex colour never reaches a style attribute')
  assert.equal(esc(`<&"'>`), '&lt;&amp;&quot;&#39;&gt;')
})

test('hosts view shows metrics, the down detail and the source', () => {
  const html = hostsHTML(catalog, 'v2026.10.3')
  assert.match(html, /CPU 6% · Mem –/)
  assert.match(html, /connection refused/)
  assert.match(html, /Offline/)
  assert.match(html, /HomeApps v2026\.10\.3/)
  assert.doesNotMatch(html, /href="http:\/\/nakedpi\.local:8899"/, 'no HostMan tile: say HostMan, link nowhere')
})

test('the change-server link only appears inside the shell', () => {
  assert.doesNotMatch(hostsHTML(catalog, 'v1'), /change-server/)
  assert.match(hostsHTML(catalog, 'v1', true), /data-action="change-server"/)
})

test('where to add apps: HostMan, opened through its own tile', () => {
  const withHostMan = structuredClone(catalog)
  withHostMan.source.url = 'http://127.0.0.1:8899' // HomeApps beside HostMan: useless from a phone
  withHostMan.hosts[0].apps.push({ id: 'hm-9', name: 'HostMan', url: 'http://nakedpi.local:8899/', port: 8899, health: 'up' })
  assert.equal(hostmanApp(withHostMan).id, 'hm-9')
  const html = appsHTML(withHostMan, '')
  assert.match(html, /deploy one to a host in <a href="http:\/\/nakedpi\.local:8899\/" data-app="hm-9"/)
  assert.doesNotMatch(html, /127\.0\.0\.1/, 'the loopback source address must never become a link')
  assert.match(hostsHTML(withHostMan, 'v1'), /apps from <a href="http:\/\/nakedpi\.local:8899\/" data-app="hm-9"/)
  assert.doesNotMatch(appsHTML(withHostMan, 'count'), /Add an app by its address/, 'no footer while searching')
  assert.equal(hostmanApp(catalog), null)
  assert.match(appsHTML({ hosts: [], source: catalog.source }, ''), /deploy one in HostMan and it shows up here/)
})

const withMine = {
  ...catalog,
  my_apps: [
    { id: 'my-a1', name: 'Router', url: 'http://router.lan/', port: 80, health: 'up', icon: '', color: '' },
    { id: 'my-b2', name: 'NAS <b>', url: 'https://nas.lan/admin', port: 443, health: 'down', icon: '', color: '' },
  ],
}

test('apps added by address come first, as "My apps"', () => {
  const gs = groups(withMine)
  assert.deepEqual(gs.map((g) => g.name), ['My apps', 'nakedpi', 'pi5', 'empty'])
  assert.equal(gs[0].mine, true)
  assert.equal(allApps(withMine).length, 5)
  assert.deepEqual(summary(withMine), { text: '5 apps · 2 hosts · 2 not responding', health: 'down' })
  const html = appsHTML(withMine, '')
  assert.ok(html.indexOf('My apps') < html.indexOf('nakedpi'), 'My apps section before the hosts')
  assert.match(html, /1 of 2 healthy/)
  assert.match(html, /data-action="add-app"/)
  assert.deepEqual(filter(withMine, 'router').map((g) => g.name), ['My apps'])
  assert.deepEqual(filter(withMine, 'my').flatMap((g) => g.apps.map((a) => a.id)), ['my-a1', 'my-b2'], 'the group name is searchable')
})

test('only added apps, no HostMan', () => {
  const solo = { my_apps: withMine.my_apps, hosts: [], source: null, hostman_error: '' }
  assert.deepEqual(summary(solo), { text: '2 apps · NAS <b> not responding', health: 'down' })
  const html = appsHTML(solo, '')
  assert.doesNotMatch(html, /HostMan/, 'no HostMan configured: never mention it')
  assert.match(appsHTML({ my_apps: [], hosts: [], source: null }, ''), /No apps yet.*data-action="add-app"/s)
})

test('hosts view lists added apps with an Edit button, escaped', () => {
  const html = hostsHTML(withMine, 'v1')
  assert.match(html, /<h2>My apps<\/h2>/)
  assert.match(html, /data-action="edit-app" data-id="my-a1"/)
  assert.match(html, /NAS &lt;b&gt;/)
  assert.doesNotMatch(html, /NAS <b>/)
  assert.match(hostsHTML({ hosts: [] }, 'v1'), /None yet\./)
})
