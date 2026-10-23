// Advance the server's business date without using the browser's local timezone.
export function serverDateAt(serverTime, receivedAt, now = Date.now()) {
  const timestamp = Date.parse(serverTime)
  if (!Number.isFinite(timestamp)) throw new Error('服务器时间无效，请刷新页面')
  const zone = serverTime.match(/([+-])(\d{2}):(\d{2})$/)
  const offset = zone ? (zone[1] === '+' ? 1 : -1) * (Number(zone[2]) * 60 + Number(zone[3])) * 60000 : 0
  return new Date(timestamp + (now - receivedAt) + offset).toISOString().slice(0, 10)
}
