import test from 'node:test'
import assert from 'node:assert/strict'
import { settingsTabs, settingsDraft, validateSettings } from '../src/admin/settings-model.js'

test('settings draft edits never alter saved or active runtime values', () => {
  const response = { saved: { liveCacheSeconds: 120, maxFilesPerRun: 1000, xrayRetainedJobs: 1 }, active: { liveCacheSeconds: 120, maxFilesPerRun: 1000, xrayRetainedJobs: 1 } }
  const draft = settingsDraft('runtime', response)
  draft.liveCacheSeconds = 60
  assert.equal(response.saved.liveCacheSeconds, 120)
  assert.equal(response.active.liveCacheSeconds, 120)
  assert.equal(validateSettings('runtime', draft), '')
  for (const value of [0, 601, 60.5, '', NaN]) assert.ok(validateSettings('runtime', { ...draft, liveCacheSeconds: value }))
})
test('settings tabs separate execution logs from configuration and validate times', () => {
  assert.equal(new Set(settingsTabs.map(tab => tab.id)).size, 5)
  assert.equal(settingsTabs[0].id, 'smb')
  const value = { chat: { time: '00:10' }, tasks: [{ syncTime: '00:20', importTime: '00:30' }] }
  assert.equal(validateSettings('schedules', value), '')
  value.tasks[0].syncTime = '24:00'
  assert.ok(validateSettings('schedules', value))
  assert.ok(validateSettings('sharing', { shareTtlMinutes: 4, logQueryShareTtlMinutes: 60 }))
})
