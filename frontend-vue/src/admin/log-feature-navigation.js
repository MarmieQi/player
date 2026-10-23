export const LOG_QUERY_TYPE_COORDINATE = 'coordinate'
export const LOG_QUERY_TYPE_PLAYER_KEYWORD = 'playerKeyword'
export const LOG_QUERY_TYPE_XRAY = 'xrayAnalysis'

export function normalizeLogQueryMode(mode) {
  return mode === LOG_QUERY_TYPE_PLAYER_KEYWORD ? mode : LOG_QUERY_TYPE_COORDINATE
}

export function logFeatureLocation(serverId, mode, playerName) {
  const analysis = mode === LOG_QUERY_TYPE_XRAY
  const params = new URLSearchParams({ serverId })
  if (analysis && playerName) params.set('player', playerName)
  if (!analysis && normalizeLogQueryMode(mode) !== LOG_QUERY_TYPE_COORDINATE) params.set('mode', normalizeLogQueryMode(mode))
  const path = analysis ? (playerName ? '/xray-analysis/detail' : '/xray-analysis') : '/log-query'
  return `#${path}?${params}`
}

export function parseLogFeatureLocation(hash) {
  const [path, search = ''] = hash.replace(/^#/, '').split('?')
  const params = new URLSearchParams(search)
  const detail = path === '/xray-analysis/detail' || path === '/log-query/xray-detail'
  if (path !== '/log-query' && path !== '/xray-analysis' && !detail) return null
  return {
    view: detail ? 'xrayDetailPage' : path === '/xray-analysis' ? 'xrayAnalysisPage' : 'logQueryPage',
    serverId: params.get('serverId') || '',
    mode: path === '/log-query' ? normalizeLogQueryMode(params.get('mode')) : LOG_QUERY_TYPE_XRAY,
    playerName: detail ? params.get('player') || '' : ''
  }
}
