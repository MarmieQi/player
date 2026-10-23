import test from 'node:test'
import assert from 'node:assert/strict'
import { logFeatureLocation, normalizeLogQueryMode, parseLogFeatureLocation } from '../src/admin/log-feature-navigation.js'

test('logs and analysis have separate reloadable locations and server scope', () => {
  for (const [mode, view] of [['coordinate', 'logQueryPage'], ['playerKeyword', 'logQueryPage'], ['xrayAnalysis', 'xrayAnalysisPage']]) {
    const route = parseLogFeatureLocation(logFeatureLocation('sub', mode))
    assert.deepEqual(route, { view, serverId: 'sub', mode, playerName: '' })
  }
  assert.equal(normalizeLogQueryMode('xrayAnalysis'), 'coordinate')
  assert.equal(parseLogFeatureLocation('#/log-query?mode=xrayAnalysis').view, 'logQueryPage')
  assert.equal(parseLogFeatureLocation('#/log-query?mode=xrayAnalysis').mode, 'coordinate')
})

test('analysis detail links retain server/player and accept the legacy URL', () => {
  const playerName = '玩家 A&B'
  const expected = { view: 'xrayDetailPage', serverId: 'sub', mode: 'xrayAnalysis', playerName }
  assert.deepEqual(parseLogFeatureLocation(logFeatureLocation('sub', 'xrayAnalysis', playerName)), expected)
  assert.deepEqual(parseLogFeatureLocation(`#/log-query/xray-detail?serverId=sub&player=${encodeURIComponent(playerName)}`), expected)
  assert.equal(parseLogFeatureLocation('#/settings'), null)
  assert.equal(parseLogFeatureLocation('#/log-query').mode, 'coordinate')
})
