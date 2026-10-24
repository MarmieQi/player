// One cancellable request per query scope; completed requests retain no payloads.
export function createLatestRequestScope() {
  const active = new Map()
  function cancel(key) {
    active.get(key)?.abort()
    active.delete(key)
  }
  return {
    begin(key) {
      cancel(key)
      const controller = new AbortController()
      active.set(key, controller)
      return controller
    },
    isCurrent: (key, controller) => active.get(key) === controller && !controller.signal.aborted,
    finish(key, controller) { if (active.get(key) === controller) active.delete(key) },
    cancel,
    dispose() { for (const key of active.keys()) cancel(key) }
  }
}
