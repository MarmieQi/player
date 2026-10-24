import test from 'node:test'
import assert from 'node:assert/strict'
import { loadChatSyncConfig } from '../src/admin/group-chat-sync-config.js'

globalThis.window = { location: { origin: 'http://localhost' } }
globalThis.localStorage = { getItem: () => 'fixture-token', removeItem: () => {} }

test('TXT sync reads the same saved source as settings and forwards cancellation', async t => {
  const original = globalThis.fetch
  t.after(() => { globalThis.fetch = original })
  const signal = new AbortController().signal
  const chat = { enabled: true, smbDirectory: 'Cat66Hub/logs/chat', fileGlob: '*.txt' }
  globalThis.fetch = async (url, options) => {
    assert.equal(new URL(url).pathname, '/api/admin/settings/smb')
    assert.equal(options.signal, signal)
    assert.equal(options.headers.Authorization, 'Bearer fixture-token')
    return { ok: true, status: 200, json: async () => ({ host: 'fixture', sources: [], chat }) }
  }
  assert.deepEqual(await loadChatSyncConfig({ signal }), chat)
  chat.enabled = false
  assert.deepEqual(await loadChatSyncConfig({ signal }), chat)
})

test('failed or malformed settings do not become a disabled TXT configuration', async t => {
  const original = globalThis.fetch
  t.after(() => { globalThis.fetch = original })
  globalThis.fetch = async () => ({ ok: false, status: 503, json: async () => ({ message: 'settings unavailable' }) })
  await assert.rejects(loadChatSyncConfig(), error => error.status === 503)
  for (const body of [{}, { chat: null }, { enabled: true }, { chat: { enabled: 'true' } }]) {
    globalThis.fetch = async () => ({ ok: true, status: 200, json: async () => body })
    await assert.rejects(loadChatSyncConfig(), /配置响应无效/)
  }
})
