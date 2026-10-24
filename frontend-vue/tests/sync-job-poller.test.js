import test from 'node:test'
import assert from 'node:assert/strict'
import { createSyncJobPoller } from '../src/admin/sync-job-poller.js'

const deferred = () => { let resolve; const promise = new Promise(r => { resolve = r }); return { promise, resolve } }

test('sync polling does not overlap requests and stops on completion', async () => {
  const pending = deferred(), timers = [], states = [], finished = []
  let calls = 0
  const poller = createSyncJobPoller({
    load: async () => ++calls === 1 ? pending.promise : { status: 'FINISHED' },
    onUpdate: job => states.push(job.status), onFinished: job => finished.push(job.status), onError: () => false,
    setTimer: fn => { timers.push(fn); return fn }, clearTimer: () => {}
  })
  const first = poller.start('job')
  assert.equal(calls, 1)
  assert.equal(timers.length, 0)
  pending.resolve({ status: 'RUNNING' }); await first
  assert.equal(timers.length, 1)
  await timers.shift()()
  assert.deepEqual(states, ['RUNNING', 'FINISHED'])
  assert.deepEqual(finished, ['FINISHED'])
  assert.equal(timers.length, 0)
  poller.stop()
})

test('late old responses cannot overwrite a newer completed sync job', async () => {
  const old = deferred(), states = []
  let signal
  const poller = createSyncJobPoller({
    load: (id, value) => { if (id === 'old') { signal = value; return old.promise } return Promise.resolve({ status: 'FINISHED' }) },
    onUpdate: job => states.push(job.status), onFinished: () => {}, onError: () => false
  })
  const first = poller.start('old')
  await poller.start('new')
  assert.equal(signal.aborted, true)
  old.resolve({ status: 'RUNNING' }); await first
  assert.deepEqual(states, ['FINISHED'])
  poller.stop()
})

test('temporary network failures retry while expired jobs stop polling', async () => {
  const timers = []
  let calls = 0
  const poller = createSyncJobPoller({
    load: async () => { throw Object.assign(new Error('network'), { status: ++calls === 1 ? 502 : 404 }) },
    onUpdate: () => {}, onFinished: () => {}, onError: err => err.status !== 404,
    setTimer: fn => { timers.push(fn); return fn }, clearTimer: () => {}
  })
  await poller.start('job')
  assert.equal(timers.length, 1)
  await timers.shift()()
  assert.equal(timers.length, 0)
  poller.stop()
})
