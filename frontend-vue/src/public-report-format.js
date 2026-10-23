const dateTime = new Intl.DateTimeFormat('zh-CN', {
  year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit'
})
const time = new Intl.DateTimeFormat('zh-CN', { hour: '2-digit', minute: '2-digit' })

export function formatReportTimeRange(start, end) {
  if (!start || !end) return '-'
  const from = new Date(start)
  const to = new Date(end)
  if (!Number.isFinite(from.getTime()) || !Number.isFinite(to.getTime())) return '-'
  const sameDay = from.getFullYear() === to.getFullYear()
    && from.getMonth() === to.getMonth() && from.getDate() === to.getDate()
  return `${dateTime.format(from)} – ${sameDay ? time.format(to) : dateTime.format(to)}`
}
