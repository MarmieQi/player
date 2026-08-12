const milestones = [
  ['first_log', '第一次来到这里', '2026-01-18T19:42:16', '旅程的起点'],
  ['first_wood', '获得第一块原木', '2026-01-18T19:48:03', '橡木原木'],
  ['first_coal', '挖到第一块煤矿', '2026-01-18T20:06:41', '煤矿石'],
  ['first_iron', '挖到第一块铁矿', '2026-01-18T20:24:09', '铁矿石'],
  ['first_diamond', '发现第一颗钻石', '2026-01-20T22:17:35', '钻石矿石'],
  ['first_debris', '发现第一块远古残骸', '2026-02-03T21:09:52', '远古残骸']
].map(([type, label, firstSeenAt, detail]) => ({ type, label, firstSeenAt, detail }))

const ores = [
  ['coal_ore', '煤矿', 12846], ['iron_ore', '铁矿', 8632], ['copper_ore', '铜矿', 4275],
  ['gold_ore', '金矿', 1864], ['redstone_ore', '红石矿', 7931], ['lapis_ore', '青金石矿', 1127],
  ['diamond_ore', '钻石矿', 386], ['emerald_ore', '绿宝石矿', 48], ['nether_quartz_ore', '下界石英矿', 2553],
  ['ancient_debris', '远古残骸', 67]
].map(([type, label, count]) => ({ type, label, count }))

const woods = [
  ['oak_log', '橡木原木', 9531], ['spruce_log', '云杉原木', 6820], ['birch_log', '白桦原木', 2187],
  ['jungle_log', '丛林原木', 764], ['acacia_log', '金合欢原木', 523], ['dark_oak_log', '深色橡木原木', 1450],
  ['mangrove_log', '红树原木', 318], ['cherry_log', '樱花原木', 896]
].map(([type, label, count]) => ({ type, label, count }))

const saplings = [
  ['oak_sapling', '橡树树苗', 684], ['spruce_sapling', '云杉树苗', 492], ['birch_sapling', '白桦树苗', 183],
  ['jungle_sapling', '丛林树苗', 46], ['acacia_sapling', '金合欢树苗', 31], ['dark_oak_sapling', '深色橡树树苗', 128],
  ['mangrove_propagule', '红树胎生苗', 25], ['cherry_sapling', '樱花树苗', 73]
].map(([type, label, count]) => ({ type, label, count }))

function server(serverId, serverName, factor, firstSeenAt) {
  const brokenCount = Math.round(186420 * factor)
  const placedCount = Math.round(82460 * factor)
  return {
    serverId,
    serverName,
    playerName: 'CloudCat',
    brokenCount,
    placedCount,
    totalCount: brokenCount + placedCount,
    firstSeenAt,
    latestLogDate: '2026-08-11',
    milestones: milestones.map((item, index) => factor < 0.5 && index === 5
      ? { ...item, firstSeenAt: null, missingText: '尚未发现远古残骸' }
      : { ...item }),
    ores: ores.map((item) => ({ ...item, count: Math.round(item.count * factor) })),
    woods: woods.map((item) => ({ ...item, count: Math.round(item.count * factor) })),
    saplings: saplings.map((item) => ({ ...item, count: Math.round(item.count * factor) }))
  }
}

export function createPlayerSharePreviewData() {
  return {
    playerName: 'CloudCat',
    latestLogDate: '2026-08-11',
    expiresAt: new Date(Date.now() + 2 * 60 * 60 * 1000).toISOString(),
    ttlMinutes: 120,
    servers: [
      server('main', '主服', 1, '2026-01-18T19:42:16'),
      server('sub', '2服', 0.38, '2026-04-06T20:15:28')
    ]
  }
}
