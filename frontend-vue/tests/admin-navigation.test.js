import test from 'node:test'
import assert from 'node:assert/strict'
import { adminNavigation, adminNavigationId, adminPage } from '../src/admin/navigation.js'

test('admin page identity depends on the page, not shared task mode', () => {
  assert.equal(adminNavigationId('dashboard'), 'dashboard')
  assert.equal(adminNavigationId('logQueryPage', 'coordinate'), 'logs')
  assert.equal(adminNavigationId('logQueryPage', 'playerKeyword'), 'logs')
  assert.equal(adminNavigationId('logQueryPage', 'xrayAnalysis'), 'logs')
  assert.equal(adminNavigationId('xrayAnalysisPage', 'coordinate'), 'xray')
  assert.equal(adminNavigationId('xrayDetailPage'), 'xray')
  assert.equal(adminNavigationId('syncPage'), 'sync')
  assert.equal(adminNavigationId('importPage'), 'import')
  assert.equal(adminNavigationId('settingsPage'), 'settings')
  assert.equal(adminNavigationId('profilePage'), 'profile')
  assert.equal(adminNavigationId('unknown'), 'dashboard')
})

test('navigation metadata has unique IDs and supports non-menu pages', () => {
  const items = adminNavigation.flatMap(group => group.items)
  assert.equal(new Set(items.map(item => item.id)).size, items.length)
  for (const item of items) assert.equal(adminPage(item.id), item)
  assert.equal(adminPage('profile').label, '个人中心')
  assert.equal(adminPage('unknown').id, 'dashboard')
})
