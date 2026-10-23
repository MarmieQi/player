import test from 'node:test'
import assert from 'node:assert/strict'
import { apiGet, apiPost, apiPostJson, apiPutJson } from '../src/api.js'

globalThis.window = { location: { origin: 'http://localhost' } }
globalThis.localStorage = { getItem: () => '', removeItem: () => {} }

test('all query and settings requests forward their cancellation signal', async t => {
  const original = globalThis.fetch
  t.after(() => { globalThis.fetch = original })
  const signal = new AbortController().signal
  globalThis.fetch = async (_url, options) => {
    assert.equal(options.signal, signal)
    return { ok: true, status: 200, json: async () => ({ success: true }) }
  }
  for (const request of [apiGet, apiPost, apiPostJson, apiPutJson]) {
    assert.deepEqual(await request('/api/test', {}, { signal }), { success: true })
  }
})

test('an aborted JSON body is never reported as an empty successful response', async t => {
  const original = globalThis.fetch
  t.after(() => { globalThis.fetch = original })
  globalThis.fetch = async () => ({ ok: true, status: 200, json: async () => { throw new DOMException('aborted', 'AbortError') } })
  await assert.rejects(apiGet('/api/test'), { name: 'AbortError' })
})

test('invalid success responses are rejected and HTTP status remains available', async t => {
  const original = globalThis.fetch
  t.after(() => { globalThis.fetch = original })
  globalThis.fetch = async () => ({ ok: true, status: 200, json: async () => { throw new SyntaxError('invalid') } })
  await assert.rejects(apiGet('/api/test'), /服务器响应格式无效/)
  globalThis.fetch = async () => ({ ok: false, status: 410, json: async () => ({ message: '查询结果已过期' }) })
  await assert.rejects(apiGet('/api/test'), error => error.status === 410 && error.message === '查询结果已过期')
  globalThis.fetch = async () => ({ ok: true, status: 204, json: async () => { throw new SyntaxError('empty') } })
  assert.deepEqual(await apiGet('/api/test'), {})
})
