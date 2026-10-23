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
