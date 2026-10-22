// Shared by administrator queries and public log pages. Never rewrite the raw API row.
const containerQuantityPattern = /^x([1-9]\d*)$/

function bucketPrimaryDetail(action) {
  switch (String(action ?? '').trim()) {
    case '使用桶放置':
    case '使用桶交互': // Historical logs keep their original label and bucket-first summary.
      return 'detail2'
    case '使用桶收取':
      return 'detail1'
    default:
      return null
  }
}

export function isBucketLogAction(action) {
  return bucketPrimaryDetail(action) !== null
}

export function logQueryActionDisplay(action) {
  return String(action ?? '').trim() || '-'
}

export function logShareActionClass(action) {
  const value = String(action ?? '').trim()
  if (value === '使用桶放置') return 'is-broken'
  if (value === '使用桶收取') return 'is-placed'
  if (value === '放入容器') return 'is-placed'
  return value.includes('放置') ? 'is-placed' : 'is-broken'
}

export function logQueryDetailDisplay(row, field) {
  const display = String(row?.[`${field}Display`] ?? '').trim()
  const raw = String(row?.[field] ?? '').trim()
  return display || raw || '-'
}

export function logQuerySummaryDetailText(row) {
  const action = String(row?.action ?? '').trim()
  if (action === '放入容器' || action === '从容器取出') {
    const item = logQueryDetailDisplay(row, 'detail1')
    const quantity = String(row?.detail2 ?? '').trim()
    if (item === '-') return quantity || '-'
    return containerQuantityPattern.test(quantity) ? `${item} ×${quantity.slice(1)}` : item
  }
  const primary = bucketPrimaryDetail(action) || 'detail1'
  const detail = logQueryDetailDisplay(row, primary)
  return detail !== '-' ? detail : logQueryDetailDisplay(row, primary === 'detail1' ? 'detail2' : 'detail1')
}
