// CSV behavior names emitted by PlayerLogger; labels never rewrite stored logs.
export const logActionOptions = [
  { value: '', label: '全部行为（清空选择）' },
  ...['破坏方块', '放置方块', '使用桶放置', '使用桶收取', '放入容器', '从容器取出', '发送聊天', '执行命令', '修改告示牌', '使用桶交互'].map(value => ({ value, label: value === '使用桶交互' ? '使用桶交互（旧日志兼容）' : value }))
]
