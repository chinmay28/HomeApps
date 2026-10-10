// Tests for the web client's pure view logic: `node --test web/`.
import { test } from 'node:test'
import assert from 'node:assert/strict'
import { allApps, appsHTML, esc, filter, hostmanApp, hostsHTML, hue, monogram, normalizeAddress, summary } from '../server/internal/web/assets/view.js'

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
  assert.deepEqual(summary(catalog), { text: '3 apps on 2 hosts · SAND Vault not responding', health: 'down' })
  const healthy = { hosts: [{ name: 'a', apps: [{ name: 'x', health: 'up' }] }] }
  assert.deepEqual(summary(healthy), { text: '1 app on 1 host · all healthy', health: 'up' })
  assert.equal(summary({ hosts: [] }).text, 'No apps yet')
})

test('filter matches every word against app and host name', () => {
  const names = (c) => c.hosts.flatMap((h) => h.apps.map((a) => a.name))
  assert.deepEqual(names(filter(catalog, 'pi5 va')), ['SAND Vault'])
  assert.deepEqual(names(filter(catalog, 'MESH')), ['Thought Mesh'])
  assert.deepEqual(names(filter(catalog, '')), ['CountRoster', 'Thought Mesh', 'SAND Vault'])
  assert.equal(filter(catalog, 'nothing').hosts.length, 0)
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
  assert.match(html, /To add an app, deploy it to a host in <a href="http:\/\/nakedpi\.local:8899\/" data-app="hm-9"/)
  assert.doesNotMatch(html, /127\.0\.0\.1/, 'the loopback source address must never become a link')
  assert.match(hostsHTML(withHostMan, 'v1'), /apps from <a href="http:\/\/nakedpi\.local:8899\/" data-app="hm-9"/)
  assert.doesNotMatch(appsHTML(withHostMan, 'count'), /To add an app/, 'no footer while searching')
  assert.equal(hostmanApp(catalog), null)
  assert.match(appsHTML({ hosts: [], source: catalog.source }, ''), /Deploy one in HostMan and it shows up here/)
})

const withCustom = {
  ...catalog,
  custom: [
    { id: 'ca-1', name: 'Router', url: 'http://192.168.1.1/', icon: '', color: '' },
    { id: 'ca-2', name: 'NAS', url: 'https://nas.local:5001/', icon: '/api/icons/cd', color: '#112233' },
  ],
}

test('custom apps are counted, but health stays HostMan\'s', () => {
  assert.deepEqual(summary(withCustom), { text: '3 apps on 2 hosts + 2 custom · SAND Vault not responding', health: 'down' })
  assert.deepEqual(summary({ hosts: [], custom: withCustom.custom.slice(0, 1) }), { text: '1 custom app', health: 'unknown' })
  const healthy = { hosts: [{ name: 'a', apps: [{ name: 'x', health: 'up' }] }], custom: withCustom.custom }
  assert.equal(summary(healthy).health, 'up', 'a custom app has no health, so it cannot make the summary unknown')
})

test('custom apps open like any other and come first', () => {
  assert.deepEqual(allApps(withCustom).map((a) => a.id), ['ca-1', 'ca-2', 'hm-1', 'hm-2', 'hm-3'])
  const html = appsHTML(withCustom, '')
  assert.ok(html.indexOf('My apps') < html.indexOf('nakedpi'), 'the hand-picked apps lead')
  assert.match(html, /href="http:\/\/192\.168\.1\.1\/" data-app="ca-1"/)
  assert.match(html, /aria-label="Router">/, 'no health words for an app without health')
  assert.doesNotMatch(html.slice(0, html.indexOf('nakedpi')), /dot--/, 'and no dot')
  assert.match(html, /data-action="add-app"/)
})

test('search finds custom apps by name and by address', () => {
  assert.deepEqual(filter(withCustom, 'router').custom.map((a) => a.id), ['ca-1'])
  assert.deepEqual(filter(withCustom, 'nas.local').custom.map((a) => a.id), ['ca-2'])
  assert.equal(filter(withCustom, 'mesh').custom.length, 0)
  assert.doesNotMatch(appsHTML(withCustom, 'mesh'), /My apps/, 'no empty section')
  assert.match(appsHTML({ hosts: [], custom: withCustom.custom }, 'router'), /data-app="ca-1"/)
})

test('with nothing at all, both ways to add an app are offered', () => {
  const html = appsHTML({ hosts: [], custom: [] }, '')
  assert.match(html, /Deploy one in HostMan/)
  assert.match(html, /<button type="button" class="link" data-action="add-app">add any web address<\/button>/)
})

test('hosts view lists custom apps with an Edit button each', () => {
  const html = hostsHTML(withCustom, 'v1')
  assert.match(html, /<h2>My apps<\/h2>/)
  assert.match(html, /data-action="edit-app" data-id="ca-2"/)
  assert.match(html, /https:\/\/nas\.local:5001\//)
  assert.doesNotMatch(hostsHTML(catalog, 'v1'), /My apps/, 'no card without custom apps')
})

test('custom apps are escaped too', () => {
  const evil = { hosts: [], custom: [{ id: '"><x', name: '<script>', url: 'http://a/"onmouseover="x', icon: '', color: '' }] }
  for (const html of [appsHTML(evil, ''), hostsHTML(evil, 'v1')]) {
    assert.doesNotMatch(html, /<script>|"><x|"onmouseover/)
  }
})

test('normalizeAddress reads a bare host the way an address bar does', () => {
  assert.equal(normalizeAddress('pi5.local:8080'), 'http://pi5.local:8080')
  assert.equal(normalizeAddress('  192.168.1.1  '), 'http://192.168.1.1')
  assert.equal(normalizeAddress('//nas.local'), 'http://nas.local')
  assert.equal(normalizeAddress('https://nas.local'), 'https://nas.local')
  assert.equal(normalizeAddress('HTTP://x'), 'HTTP://x')
  assert.equal(normalizeAddress('ftp://x'), 'ftp://x', 'a scheme is left for the server to refuse')
  assert.equal(normalizeAddress('javascript:alert(1)'), 'javascript:alert(1)')
  assert.equal(normalizeAddress('localhost:3000/x'), 'http://localhost:3000/x')
  assert.equal(normalizeAddress('   '), '')
})
