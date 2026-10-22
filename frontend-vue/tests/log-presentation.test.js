import assert from 'node:assert/strict'
import test from 'node:test'
import {
  isBucketLogAction,
  logQueryActionDisplay,
  logQueryDetailDisplay,
  logQuerySummaryDetailText,
  logShareActionClass
} from '../src/log-presentation.js'

test('public rows distinguish bucket placement and collection while retaining legacy labels', () => {
  for (const action of ['使用桶放置', '使用桶收取', '使用桶交互']) {
    assert.equal(isBucketLogAction(` ${action} `), true)
    assert.equal(logQueryActionDisplay(action), action)
  }
  assert.equal(isBucketLogAction('放置方块'), false)
  assert.equal(isBucketLogAction(undefined), false)
  assert.equal(logQueryActionDisplay(undefined), '-')
  assert.equal(logQueryActionDisplay('破坏方块'), '破坏方块')
})

test('container rows display item and count without changing bucket or block colors', () => {
  for (const action of ['放入容器', '从容器取出']) {
    const row = Object.freeze({ action, detail1: 'minecraft:lava_bucket', detail1Display: '熔岩桶', detail2: 'x1' })
    assert.equal(logQueryActionDisplay(action), action)
    assert.equal(isBucketLogAction(action), false)
    assert.equal(logQuerySummaryDetailText(row), '熔岩桶 ×1')
    assert.equal(row.detail2, 'x1')
    assert.equal(logQuerySummaryDetailText({ ...row, detail1Display: '', detail1: 'custom:item', detail2: 'x64' }), 'custom:item ×64')
    assert.equal(logQuerySummaryDetailText({ ...row, detail2: '-' }), '熔岩桶')
  }
  assert.equal(logShareActionClass('放入容器'), 'is-placed')
  assert.equal(logShareActionClass('从容器取出'), 'is-broken')
  assert.equal(logShareActionClass('使用桶放置'), 'is-broken')
  assert.equal(logShareActionClass('使用桶收取'), 'is-placed')
  assert.equal(logShareActionClass('放置方块'), 'is-placed')
  assert.equal(logShareActionClass('破坏方块'), 'is-broken')
})

test('native water/lava summaries use the intended detail field without altering API rows', () => {
  for (const [block, blockDisplay, bucket, bucketDisplay] of [
    ['minecraft:water', '水', 'minecraft:water_bucket', '水桶'],
    ['minecraft:lava', '熔岩', 'minecraft:lava_bucket', '熔岩桶']
  ]) {
    const placement = Object.freeze({ action: '使用桶放置', detail1: '-', detail2: bucket, detail2Display: bucketDisplay })
    const collection = Object.freeze({ action: '使用桶收取', detail1: block, detail1Display: blockDisplay, detail2: '-' })
    assert.equal(logQuerySummaryDetailText(placement), bucketDisplay)
    assert.equal(logQuerySummaryDetailText(collection), blockDisplay)
    assert.equal(collection.detail1, block)
    assert.equal(placement.detail2, bucket)
    // Collection prioritizes the affected liquid even when an old producer supplies both details.
    assert.equal(logQuerySummaryDetailText({ ...collection, detail2: 'minecraft:bucket', detail2Display: '桶' }), blockDisplay)
  }
})

test('legacy summaries, untranslated IDs and ordinary block logs retain their fallback behavior', () => {
  assert.equal(logQuerySummaryDetailText({ action: '使用桶交互', detail1: 'minecraft:stone', detail2: 'minecraft:bucket', detail2Display: '桶' }), '桶')
  assert.equal(logQuerySummaryDetailText({ action: '使用桶放置', detail1: 'minecraft:stone', detail2: '-' }), 'minecraft:stone')
  assert.equal(logQuerySummaryDetailText({ action: '使用桶收取', detail1: 'custom:liquid', detail2: '-' }), 'custom:liquid')
  assert.equal(logQuerySummaryDetailText({ action: '破坏方块', detail1: 'minecraft:stone', detail1Display: '石头', detail2: 'other' }), '石头')
  assert.equal(logQueryDetailDisplay({ detail1: 'raw' }, 'detail1'), 'raw')
  assert.equal(logQuerySummaryDetailText({}), '-')
})
