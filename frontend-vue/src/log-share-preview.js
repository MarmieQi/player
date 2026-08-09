export function createLogSharePreviewData() {
  const players = ['Steve', 'Alex', 'CloudCat']
  const actions = ['破坏方块', '放置方块']
  const details = ['minecraft:stone', 'minecraft:oak_planks']
  const rows = Array.from({ length: 286 }, (_, index) => ({
    fileName: 'player_actions_2026-08-10.csv',
    lineNumber: index + 1,
    date: '2026-08-10',
    time: `20:${String(59 - (index % 60)).padStart(2, '0')}:12`,
    playerName: players[index % players.length],
    action: actions[index % actions.length],
    x2: String(-191 + (index % 5)),
    y2: '-34',
    z2: String(750 + (index % 4)),
    dimension2: '主世界',
    detail1: details[index % details.length],
    detail2: '',
  }))

  return {
    serverId: 'main',
    serverName: '主服',
    x: -191,
    y: -34,
    z: 750,
    radius: 5,
    days: 7,
    fromDate: '2026-08-04',
    toDate: '2026-08-10',
    scannedFiles: 7,
    scannedRows: 182430,
    matchedRows: 286,
    displayedRows: rows.length,
    failedFiles: 0,
    message: '匹配到 286 条日志',
    rows,
    expiresAt: new Date(Date.now() + 2 * 60 * 60 * 1000).toISOString(),
    ttlMinutes: 120,
  }
}
