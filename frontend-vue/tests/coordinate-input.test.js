import assert from 'node:assert/strict'
import test from 'node:test'
import { parseCoordinateInput, savedCoordinateInput, coordinateQueryParameters } from '../src/admin/coordinate-input.js'

test('coordinate input accepts pasted triples, zeros, negatives and decimals', () => {
  assert.deepEqual(parseCoordinateInput('34 35 523'), [34, 35, 523])
  assert.deepEqual(parseCoordinateInput('  -34, 0，+523.5  '), [-34, 0, 523.5])
  assert.deepEqual(parseCoordinateInput('34\t35\n523'), [34, 35, 523])
})

test('invalid or incomplete coordinates are rejected rather than partially queried', () => {
  for (const value of ['', '34 35', '34 35 523 6', '34 nope 523', '~1 35 523', '0x10 35 523', 'Infinity 0 0', '9'.repeat(400) + ' 0 0']) {
    assert.equal(parseCoordinateInput(value), null, value)
  }
})

test('saved coordinate triples keep existing browser form values including zero', () => {
  assert.equal(savedCoordinateInput({ x1: 34, y1: 0, z1: -523 }, 1), '34 0 -523')
  assert.equal(savedCoordinateInput({ position1: '', x1: 34, y1: 0, z1: -523 }, 1), '')
  assert.equal(savedCoordinateInput({ position2: '1 2 3' }, 2), '1 2 3')
  assert.equal(savedCoordinateInput(undefined, 1), '')
})

test('radius requests need only a center and keep box inputs independent', () => {
  const form = { rangeMode: 'radius', centerPosition: '0 -2 3', radius: 0, position1: '10 20 30', position2: '40 50 60' }
  assert.deepEqual(coordinateQueryParameters(form), { rangeMode: 'radius', x1: 0, y1: -2, z1: 3, radius: 0 })
  assert.deepEqual(coordinateQueryParameters({ ...form, rangeMode: 'box' }), { rangeMode: 'box', x1: 10, y1: 20, z1: 30, x2: 40, y2: 50, z2: 60 })
  assert.equal(form.centerPosition, '0 -2 3')
  assert.equal(coordinateQueryParameters({ ...form, radius: 200 }).radius, 200)
  assert.equal(coordinateQueryParameters({ position1: '1 2 3', position2: '4 5 6' }).rangeMode, 'box')
})

test('radius input rejects incomplete centers and invalid or absent radii', () => {
  for (const radius of ['', ' ', undefined, null, -1, 201, Infinity, NaN]) {
    assert.throws(() => coordinateQueryParameters({ rangeMode: 'radius', centerPosition: '1 2 3', radius }), /半径/)
  }
  assert.throws(() => coordinateQueryParameters({ rangeMode: 'radius', centerPosition: '1 2', radius: 5 }), /中心坐标/)
  assert.throws(() => coordinateQueryParameters({ rangeMode: 'other' }), /范围方式/)
})
