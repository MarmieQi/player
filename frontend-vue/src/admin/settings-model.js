export const settingsTabs = [
  { id: 'smb', label: 'SMB 与目录', title: 'SMB 与目录', description: '连接共享服务器，配置 CSV 和 TXT 日志来源。' },
  { id: 'schedules', label: '自动任务', title: '自动任务', description: '设置每日同步与解析计划。' },
  { id: 'sharing', label: '分享与插件', title: '分享与插件', description: '管理分享链接和 AstrBot 接入。' },
  { id: 'runtime', label: '运行参数', title: '运行参数', description: '调整查询缓存与任务处理上限。' },
  { id: 'logs', label: '运行日志', title: '运行日志', description: '查看自动任务的执行记录。' }
]

export const runtimeFields = [
  { key: 'liveCacheSeconds', label: '当天日志缓存时长', unit: '秒', min: 10, max: 600, hint: 'CSV 和 TXT 共用。到期后，下次查询重新获取当天文件。' },
  { key: 'maxFilesPerRun', label: '每轮文件处理上限', unit: '个', min: 1, max: 100000, hint: '限制每个日志源单轮同步或解析处理的文件数量。' },
  { key: 'xrayRetainedJobs', label: '矿透任务保留数量', unit: '份 / 服务器', min: 1, max: 20, hint: '每个服务器在数据库保留的分析任务数量，至少保留 1 份。' }
]

export function settingsDraft(id, response) {
  return structuredClone(id === 'runtime' ? response.saved : response)
}

export function validateSettings(id, draft) {
  if (id === 'runtime') {
    for (const field of runtimeFields) {
      const value = draft[field.key]
      if (!Number.isInteger(value) || value < field.min || value > field.max) return `${field.label}必须为 ${field.min}～${field.max} 的整数`
    }
  }
  if (id === 'sharing') {
    for (const key of ['shareTtlMinutes', 'logQueryShareTtlMinutes']) {
      if (!Number.isInteger(draft[key]) || draft[key] < 5 || draft[key] > 10080) return '分享有效期必须为 5～10080 分钟的整数'
    }
  }
  if (id === 'schedules') {
    const valid = value => /^([01]\d|2[0-3]):[0-5]\d$/.test(value)
    if (!valid(draft.chat.time) || draft.tasks.some(task => !valid(task.syncTime) || !valid(task.importTime))) return '执行时间必须为有效的 HH:mm'
  }
  return ''
}
