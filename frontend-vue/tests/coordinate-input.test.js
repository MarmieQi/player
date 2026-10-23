import assert from 'node:assert/strict'
import test from 'node:test'
import { parseCoordinateInput, savedCoordinateInput } from '../src/admin/coordinate-input.js'

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
