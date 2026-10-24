const taskViews = new Map([['sync', 'syncPage'], ['import', 'importPage']])

// Tasks require one concrete source. Dashboard's aggregate scope is never valid.
export function resolveTaskServer(requested, current, options) {
  const ids = options.map(option => option.serverId).filter(id => typeof id === 'string' && id && id !== 'all')
  if (ids.includes(requested)) return requested
  if (ids.includes(current)) return current
  return ids.includes('main') ? 'main' : ids[0] || 'main'
}

export function taskLocation(page, serverId) {
  if (!taskViews.has(page)) throw new Error('未知任务页面')
  return `#/${page}?${new URLSearchParams({ serverId })}`
}

export function parseTaskLocation(hash) {
  const [path, query = ''] = hash.replace(/^#\//, '').split('?')
  const view = taskViews.get(path)
  return view ? { view, serverId: new URLSearchParams(query).get('serverId') || '' } : null
}
