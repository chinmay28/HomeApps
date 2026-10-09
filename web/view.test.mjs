// Tests for the web client's pure view logic: `node --test web/`.
import { test } from 'node:test'
import assert from 'node:assert/strict'
import { appsHTML, esc, filter, hostsHTML, hue, monogram, summary } from '../server/internal/web/assets/view.js'

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
  assert.match(html, /href="http:\/\/nakedpi\.local:8899"/)
})

test('the change-server link only appears inside the shell', () => {
  assert.doesNotMatch(hostsHTML(catalog, 'v1'), /change-server/)
  assert.match(hostsHTML(catalog, 'v1', true), /data-action="change-server"/)
})
