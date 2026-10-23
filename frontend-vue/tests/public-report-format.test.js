import test from 'node:test'
import assert from 'node:assert/strict'
import { formatReportTimeRange } from '../src/public-report-format.js'

test('same-day ranges retain the date once and both times', () => {
  assert.equal(formatReportTimeRange(new Date(2026, 9, 23, 2, 10), new Date(2026, 9, 23, 2, 48)), '2026/10/23 02:10 – 02:48')
})
test('cross-day and cross-year ranges retain both dates', () => {
  assert.equal(formatReportTimeRange(new Date(2026, 11, 31, 23, 50), new Date(2027, 0, 1, 0, 10)), '2026/12/31 23:50 – 2027/01/01 00:10')
})
test('missing or invalid ranges render the empty marker', () => {
  assert.equal(formatReportTimeRange(null, new Date()), '-')
  assert.equal(formatReportTimeRange('invalid', 'invalid'), '-')
})
