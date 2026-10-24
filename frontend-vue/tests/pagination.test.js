import test from 'node:test'
import assert from 'node:assert/strict'
import { orderedPageRows } from '../src/admin/pagination.js'
import { createLatestRequestScope } from '../src/admin/latest-request.js'

test('mining rows sort globally before pagination without mutating analysis data', () => {
  const rows = Object.freeze(Array.from({ length: 601 }, (_, i) => Object.freeze({ id: i })))
  for (const order of ['asc', 'desc', 'asc']) {
    for (const size of [50, 200, 500]) {
      const result = []
      for (let page = 1; result.length < rows.length; page++) result.push(...orderedPageRows(rows, page, size, order))
      const expected = order === 'desc' ? [...rows].reverse() : [...rows]
      assert.deepEqual(result, expected)
    }
  }
  assert.equal(rows[0].id, 0)
  assert.deepEqual(orderedPageRows([], 1, 200, 'desc'), [])
  assert.deepEqual(orderedPageRows(rows, 99, 200, 'desc'), [rows[0]])
})

test('replaced, aborted and disposed reads cannot overwrite the current query', () => {
  const scope = createLatestRequestScope()
  const first = scope.begin('main:logs')
  const other = scope.begin('sub:logs')
  const latest = scope.begin('main:logs')
  assert.equal(first.signal.aborted, true)
  assert.equal(scope.isCurrent('main:logs', first), false)
  scope.finish('main:logs', first)
  assert.equal(scope.isCurrent('main:logs', latest), true)
  assert.equal(scope.isCurrent('sub:logs', other), true)
  scope.finish('main:logs', latest)
  assert.equal(scope.isCurrent('main:logs', latest), false)
  const pending = scope.begin('main:logs')
  scope.dispose()
  assert.equal(pending.signal.aborted, true)
  assert.equal(other.signal.aborted, true)
  assert.equal(scope.isCurrent('main:logs', pending), false)
})
