export function createLogSharePreviewData() {
  // Development-only public page preview using the supplied native PlayerLogger examples.
  const samples = [
    ['03:26:17', '使用桶收取', '59', '62', 'minecraft:water', '-', '水', ''],
    ['03:26:35', '使用桶放置', '58', '62', '-', 'minecraft:lava_bucket', '', '熔岩桶'],
    ['03:26:40', '使用桶放置', '58', '63', '-', 'minecraft:water_bucket', '', '水桶'],
    ['03:26:42', '使用桶收取', '59', '63', 'minecraft:water', '-', '水', ''],
  ]
  const rows = samples.map(([time, action, x, y, detail1, detail2, detail1Display, detail2Display], index) => ({
    fileName: 'player_actions_2026-10-23.csv',
    lineNumber: index + 1,
    date: '2026-10-23',
    time,
    playerName: 'Lcmnn',
    action,
    x,
    y,
    z: '52',
    dimension: '主世界',
    // The API exposes the bucket position in position 2 for a consistent query-page contract.
    x2: x,
    y2: y,
    z2: '52',
    dimension2: '主世界',
    detail1,
    detail2,
    detail1Display,
    detail2Display,
  }))

  for (const [time, action, x, lineNumber] of [
    ['03:29:26', '放入容器', '64', 5],
    ['03:29:28', '从容器取出', '65', 6],
  ]) {
    rows.push({
      fileName: 'player_actions_2026-10-23.csv',
      lineNumber,
      date: '2026-10-23',
      time,
      playerName: 'Lcmnn',
      action,
      x, y: '64', z: '55', dimension: '主世界',
      x2: '66', y2: '64', z2: '56', dimension2: '主世界',
      detail1: 'minecraft:lava_bucket', detail1Display: '熔岩桶', detail2: 'x1',
    })
  }
  rows.sort((a, b) => a.time.localeCompare(b.time))

  return {
    serverId: 'main',
    serverName: '主服',
    x: 62,
    y: 63,
    z: 54,
    radius: 5,
    days: 1,
    fromDate: '2026-10-23',
    toDate: '2026-10-23',
    scannedFiles: 1,
    scannedRows: rows.length,
    matchedRows: rows.length,
    displayedRows: rows.length,
    failedFiles: 0,
    message: `匹配到 ${rows.length} 条日志`,
    rows,
    expiresAt: new Date(Date.now() + 2 * 60 * 60 * 1000).toISOString(),
    ttlMinutes: 120,
  }
}
