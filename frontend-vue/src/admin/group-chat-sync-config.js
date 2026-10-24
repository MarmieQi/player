import { apiGet } from '../api.js'

export async function loadChatSyncConfig(options = {}) {
  const settings = await apiGet('/api/admin/settings/smb', {}, options)
  const chat = settings?.chat
  if (!chat || typeof chat.enabled !== 'boolean' || typeof chat.smbDirectory !== 'string' || typeof chat.fileGlob !== 'string') {
    throw new Error('群服聊天配置响应无效，请刷新后重试')
  }
  return chat
}
