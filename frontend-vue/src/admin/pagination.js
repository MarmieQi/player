export const DEFAULT_PAGE_SIZE = 200
export const PAGE_SIZE_OPTIONS = [50, 100, 200, 500]

// Input is a stable ascending snapshot. Only the requested page is copied.
export function orderedPageRows(rows, page, pageSize, sortOrder = 'asc') {
  const totalPages = Math.max(1, Math.ceil(rows.length / pageSize))
  const safePage = Math.max(1, Math.min(page, totalPages))
  const start = (safePage - 1) * pageSize
  const end = Math.min(start + pageSize, rows.length)
  const result = []
  for (let i = start; i < end; i++) {
    result.push(rows[sortOrder === 'desc' ? rows.length - 1 - i : i])
  }
  return result
}
