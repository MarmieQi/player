// Parse only three finite decimal coordinates; never silently drop invalid tokens.
export function parseCoordinateInput(value) {
  const parts = String(value ?? '').trim().split(/[\s,，]+/)
  if (parts.length !== 3 || !parts.every(part => /^[+-]?(?:\d+(?:\.\d*)?|\.\d+)$/.test(part))) return null
  const coordinates = parts.map(Number)
  return coordinates.every(Number.isFinite) ? coordinates : null
}

export function savedCoordinateInput(form, index) {
  return form?.[`position${index}`] ?? ['x', 'y', 'z'].map(axis => form?.[`${axis}${index}`] ?? '').join(' ').trim()
}

export const coordinateRangeOptions = [
  { value: 'radius', label: '中心半径' },
  { value: 'box', label: '起始坐标' }
]

// Both coordinate modes use the same query endpoint and interaction coordinates.
export function coordinateQueryParameters(form) {
  const rangeMode = form.rangeMode || 'box'
  if (rangeMode !== 'box' && rangeMode !== 'radius') throw new Error('不支持的坐标范围方式')
  const start = parseCoordinateInput(rangeMode === 'radius' ? form.centerPosition : form.position1)
  if (!start) throw new Error(rangeMode === 'radius' ? '请输入完整的中心坐标，例如：34 35 523' : '请输入完整的起点坐标，例如：34 35 523')
  if (rangeMode === 'radius') {
    const raw = String(form.radius ?? '').trim()
    const radius = Number(raw)
    if (!raw || !Number.isFinite(radius) || radius < 0 || radius > 200) throw new Error('查询半径必须在 0-200 格之间')
    return { rangeMode, x1: start[0], y1: start[1], z1: start[2], radius }
  }
  const end = parseCoordinateInput(form.position2)
  if (!end) throw new Error('请输入完整的终点坐标，例如：40 40 530')
  return { rangeMode, x1: start[0], y1: start[1], z1: start[2], x2: end[0], y2: end[1], z2: end[2] }
}
