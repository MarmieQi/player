// Each mounted picker owns its requests and bounded cache.
export function createPlayerSuggestionSearch({ request, onChange, delay = 250, now = Date.now }) {
  const cache = new Map()
  let timer, controller, version = 0
  function cancel() {
    version++
    clearTimeout(timer)
    controller?.abort()
    controller = undefined
  }
  function search(serverId, input) {
    cancel()
    const query = String(input ?? '').trim()
    if (!serverId || !query || [...query].length > 64) {
      onChange({ items: [], loading: false, error: '' })
      return
    }
    const key = JSON.stringify([serverId, query.toLowerCase()])
    const cached = cache.get(key)
    if (cached && cached.expires > now()) {
      onChange({ items: cached.items, loading: false, error: '' })
      return
    }
    onChange({ items: [], loading: true, error: '' })
    const current = version
    timer = setTimeout(async () => {
      controller = new AbortController()
      try {
        const result = await request(serverId, query, controller.signal)
        if (version !== current) return
        const items = Array.isArray(result) ? result.filter(item => typeof item === 'string').slice(0, 10) : []
        cache.delete(key)
        cache.set(key, { items, expires: now() + 60000 })
        while (cache.size > 32) cache.delete(cache.keys().next().value)
        onChange({ items, loading: false, error: '' })
      } catch (error) {
        if (version === current) onChange({ items: [], loading: false, error: '候选暂不可用，仍可输入完整玩家名查询' })
      }
    }, delay)
  }
  return { search, cancel }
}
