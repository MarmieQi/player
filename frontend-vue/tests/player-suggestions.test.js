import assert from 'node:assert/strict'
import test from 'node:test'
import { createPlayerSuggestionSearch } from '../src/admin/player-suggestions.js'
const tick = () => new Promise(resolve => setTimeout(resolve, 8))

test('debounces input, caches repeated queries and scopes cache by server', async () => {
  const calls = [], states = []
  let time = 0
  const search = createPlayerSuggestionSearch({ delay: 0, now: () => time, request: async (server, q) => { calls.push([server, q]); return [server + q] }, onChange: state => states.push(state) })
  search.search('main', 'L'); search.search('main', 'Lc'); await tick()
  assert.deepEqual(calls, [['main', 'Lc']])
  search.search('main', 'lc'); assert.equal(calls.length, 1)
  assert.deepEqual(states.at(-1).items, ['mainLc'])
  search.search('server2', 'lc'); await tick(); assert.equal(calls.length, 2)
  time = 61000
  search.search('main', 'lc'); await tick(); assert.equal(calls.length, 3)
  search.cancel()
})

test('stale requests cannot replace a newer server or cleared input', async () => {
  const pending = [], states = []
  const search = createPlayerSuggestionSearch({ delay: 0, request: (server, q, signal) => new Promise(resolve => pending.push({ resolve, signal })), onChange: state => states.push(state) })
  search.search('main', 'Lc'); await tick()
  search.search('server2', 'Alex'); await tick()
  assert.equal(pending[0].signal.aborted, true)
  pending[1].resolve(['Alex']); await tick()
  pending[0].resolve(['Lcmnn']); await tick()
  assert.deepEqual(states.at(-1).items, ['Alex'])
  search.search('main', 'New'); await tick(); search.search('main', '')
  pending[2].resolve(['New']); await tick()
  assert.deepEqual(states.at(-1), { items: [], loading: false, error: '' })
  search.cancel()
})

test('failure leaves a non-blocking status and cancel stops scheduled work', async () => {
  let calls = 0, state
  const search = createPlayerSuggestionSearch({ delay: 0, request: async () => { calls++; throw new Error('offline') }, onChange: value => { state = value } })
  search.search('main', 'Alex'); await tick()
  assert.equal(state.loading, false); assert.match(state.error, /完整玩家名/)
  search.search('main', 'Steve'); search.cancel(); await tick()
  assert.equal(calls, 1)
})
