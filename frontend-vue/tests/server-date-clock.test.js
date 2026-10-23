import test from 'node:test'
import assert from 'node:assert/strict'
import { serverDateAt } from '../src/admin/server-date-clock.js'

test('chat date rolls over in server timezone while the browser remains open', () => {
  const received = 100000
  const clock = '2026-10-24T23:59:30+08:00'
  assert.equal(serverDateAt(clock, received, received + 29000), '2026-10-24')
  assert.equal(serverDateAt(clock, received, received + 30000), '2026-10-25')
  assert.equal(serverDateAt('2026-10-31T23:59:30-05:00', received, received + 60000), '2026-11-01')
  assert.equal(serverDateAt('2026-12-31T23:59:30Z', received, received + 60000), '2027-01-01')
})
