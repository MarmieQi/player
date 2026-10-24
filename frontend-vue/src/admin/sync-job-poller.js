// Schedule the next read only after the previous read/callback has finished.
// Navigation and job changes abort the old read and invalidate late responses.
export function createSyncJobPoller({ load, onUpdate, onFinished, onError, setTimer = setTimeout, clearTimer = clearTimeout }) {
  let revision = 0
  let timer
  let controller
  function stop() {
    revision++
    clearTimer(timer)
    controller?.abort()
    timer = undefined
    controller = undefined
  }
  function start(id) {
    stop()
    const version = revision
    async function tick() {
      const request = new AbortController()
      controller = request
      const isCurrent = () => version === revision && !request.signal.aborted
      try {
        const job = await load(id, request.signal)
        if (!isCurrent()) return
        onUpdate(job)
        if (['PENDING', 'RUNNING'].includes(job.status)) timer = setTimer(tick, 1000)
        else await onFinished(job, { signal: request.signal, isCurrent })
      } catch (err) {
        if (!isCurrent() || err.name === 'AbortError') return
        if (onError(err) !== false && isCurrent()) timer = setTimer(tick, 3000)
      } finally {
        if (controller === request) controller = undefined
      }
    }
    return tick()
  }
  return { start, stop }
}
