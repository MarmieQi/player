import test from 'node:test'
import assert from 'node:assert/strict'
import { resolveTaskServer, taskLocation, parseTaskLocation } from '../src/admin/task-navigation.js'

const options = [{ serverId: 'main' }, { serverId: 'sub' }]

test('task navigation resolves aggregate and missing scopes to a real server', () => {
  assert.equal(resolveTaskServer('all', 'all', options), 'main')
  assert.equal(resolveTaskServer(undefined, 'sub', options), 'sub')
  assert.equal(resolveTaskServer('sub', 'main', options), 'sub')
  assert.equal(resolveTaskServer('removed', 'all', options), 'main')
  assert.equal(resolveTaskServer('all', 'all', [{ serverId: 'all' }, { serverId: 'custom' }]), 'custom')
  assert.equal(resolveTaskServer('', '', []), 'main')
})

test('CSV sync and parsing routes preserve source when refreshing or navigating back', () => {
  for (const [page, view] of [['sync', 'syncPage'], ['import', 'importPage']]) {
    assert.deepEqual(parseTaskLocation(taskLocation(page, 'sub')), { view, serverId: 'sub' })
    assert.deepEqual(parseTaskLocation(`#/${page}`), { view, serverId: '' })
    assert.equal(resolveTaskServer(parseTaskLocation(`#/${page}?serverId=all`).serverId, 'all', options), 'main')
  }
  assert.equal(parseTaskLocation('#/settings'), null)
  assert.equal(parseTaskLocation('#/group-chat'), null)
  assert.equal(parseTaskLocation('#/constructor'), null)
})
