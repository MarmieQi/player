// Presentation metadata only. Business actions remain with the application.
export const adminNavigation = [
  { label: '数据查看', items: [
    { id: 'dashboard', label: '统计总览', icon: 'overview', description: '服务器行为统计与玩家活跃情况' },
    { id: 'logs', label: '日志查询', icon: 'search', description: '按坐标、玩家和关键词追溯行为记录' },
    { id: 'xray', label: '矿透分析', icon: 'analysis', description: '分析挖矿行为，查看风险与证据' }
  ] },
  { label: '系统管理', items: [
    { id: 'sync', label: 'CSV 同步', icon: 'sync', description: '从远程服务器复制日志文件' },
    { id: 'import', label: '解析入库', icon: 'database', description: '管理 CSV 解析与导入任务' },
    { id: 'settings', label: '系统设置', icon: 'settings', description: '管理数据源、自动任务与接口配置' }
  ] }
]

export function adminPage(id) {
  if (id === 'profile') return { id, label: '个人中心', description: '管理账号信息与登录安全' }
  return adminNavigation.flatMap(group => group.items).find(item => item.id === id)
    || adminNavigation[0].items[0]
}

export function adminNavigationId(view) {
  const pages = { dashboard: 'dashboard', logQueryPage: 'logs', xrayAnalysisPage: 'xray', settingsPage: 'settings', syncPage: 'sync', importPage: 'import', profilePage: 'profile', xrayDetailPage: 'xray' }
  return pages[view] || 'dashboard'
}
