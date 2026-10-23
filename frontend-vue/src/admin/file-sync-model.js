// File synchronization presentation shared by CSV and TXT. No API knowledge here.
export function fileNameFromPath(path) { return String(path || '').split(/[\\/]/).pop() || '-' }
export function fileCompareKey(file) { return `${file.serverId}\n${file.fileName || fileNameFromPath(file.remotePath)}` }
export function localFileKey(file) { return `${file.serverId}\n${file.remotePath}` }
export function fileSize(value) {
  const bytes = Number(value) || 0
  if (bytes < 1024) return `${bytes} B`
  return bytes < 1048576 ? `${(bytes / 1024).toFixed(1)} KB` : `${(bytes / 1048576).toFixed(1)} MB`
}
export function mergeSyncFiles(remoteFiles, jobFiles = [], localFiles = []) {
  const pending = new Map(jobFiles.map(file => [fileCompareKey(file), file]))
  const rows = remoteFiles.map(remote => {
    const key = fileCompareKey(remote)
    const job = pending.get(key)
    pending.delete(key)
    return job ? { ...remote, ...job, remotePath: remote.remotePath || job.remotePath, fileName: remote.fileName || job.fileName, fileSize: remote.fileSize ?? job.fileSize, lastModified: remote.lastModified } : remote
  })
  const merged = [...rows, ...pending.values()]
  const known = new Set(merged.map(fileCompareKey))
  for (const local of localFiles) {
    if (known.has(fileCompareKey(local))) continue
    merged.push({ ...local, localOnly: true, status: 'LOCAL', fileSize: null })
    known.add(fileCompareKey(local))
  }
  return merged
}
export function compareLocalCopy(remote, local) {
  if (!local) return { label: '未同步', className: 'pending', fileSize: null, message: '本地未找到' }
  if (remote.localOnly) return { label: '仅本地', className: 'imported', fileSize: Number(local.fileSize) || 0, message: '远端列表中没有此文件' }
  const remoteSize = Number(remote.fileSize) || 0
  const localSize = Number(local.fileSize) || 0
  if (remoteSize !== localSize) return { label: '大小不一致', className: 'failed', fileSize: localSize, message: `本地 ${fileSize(localSize)}，远程 ${fileSize(remoteSize)}` }
  const remoteTime = new Date(remote.lastModified || '').getTime()
  const localTime = new Date(local.lastModified || '').getTime()
  if (Number.isFinite(remoteTime) && Number.isFinite(localTime) && remoteTime !== localTime) return { label: '时间不一致', className: 'pending', fileSize: localSize, message: '远程修改时间与本地副本不同' }
  return { label: '已在本地', className: 'imported', fileSize: localSize, message: `本地 ${fileSize(localSize)}` }
}
export function syncStatusLabel(status = 'READY') { return { READY: '就绪', PENDING: '待同步', RUNNING: '同步中', FINISHED: '已完成', FINISHED_WITH_ERRORS: '完成有失败', COPIED: '已同步', IMPORTED: '已在本地', SKIPPED: '跳过', SKIPPED_TODAY: '今日跳过', CHANGED: '已变化', FAILED: '失败', REMOTE: '远程' }[status] || status }
export function syncStatusClass(status) { return { imported: ['COPIED', 'IMPORTED', 'FINISHED'].includes(status), running: status === 'RUNNING', pending: ['PENDING', 'CHANGED', 'REMOTE'].includes(status), skipped: ['SKIPPED', 'SKIPPED_TODAY'].includes(status), failed: ['FAILED', 'FINISHED_WITH_ERRORS'].includes(status) } }
