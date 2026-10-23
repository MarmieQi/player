import test from 'node:test'
import assert from 'node:assert/strict'
import { mergeSyncFiles, compareLocalCopy, fileCompareKey } from '../src/admin/file-sync-model.js'

test('CSV and TXT share metadata comparison including empty local files', () => {
  for (const extension of ['csv', 'txt']) {
    const remote = {serverId:'main',fileName:`2026-10-23.${extension}`,fileSize:15,lastModified:'2026-10-23T00:00:00Z'}
    assert.equal(compareLocalCopy(remote, null).label, '未同步')
    assert.equal(compareLocalCopy(remote, {...remote,fileSize:0}).label, '大小不一致')
    assert.equal(compareLocalCopy(remote, {...remote,lastModified:'2026-10-23T01:00:00Z'}).label, '时间不一致')
    assert.equal(compareLocalCopy(remote, {...remote,lastModified:'2026-10-23T08:00:00+08:00'}).label, '已在本地')
  }
})
test('job state merges by source and filename while remote metadata retains zero-byte size', () => {
  const remote = {serverId:'main',fileName:'test.csv',fileSize:0,lastModified:'2026-10-23T00:00:00Z',remotePath:'logs/test.csv',status:'REMOTE'}
  const job = {...remote,remotePath:'/data/main/test.csv',fileSize:99,status:'COPIED'}
  const other = {...job,serverId:'sub'}
  const rows = mergeSyncFiles([remote],[job,other])
  assert.equal(rows.length,2)
  assert.equal(rows[0].status,'COPIED')
  assert.equal(rows[0].fileSize,0)
  assert.equal(rows[0].remotePath,'logs/test.csv')
  assert.notEqual(fileCompareKey(rows[0]),fileCompareKey(rows[1]))
  assert.equal(remote.status,'REMOTE')
})

test('local archives remain manageable when their remote files no longer exist', () => {
  const local = { serverId: 'group-chat', fileName: '2026-10-20.txt', remotePath: '/data/chat/2026-10-20.txt', fileSize: 123 }
  const rows = mergeSyncFiles([], [], [local])
  assert.equal(rows.length, 1)
  assert.equal(rows[0].localOnly, true)
  assert.equal(rows[0].remotePath, local.remotePath)
  assert.equal(compareLocalCopy(rows[0], local).fileSize, 123)
  assert.equal(mergeSyncFiles([local], [], [local]).length, 1)
})
