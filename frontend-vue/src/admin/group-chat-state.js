import { reactive } from 'vue'
import { DEFAULT_PAGE_SIZE } from './pagination.js'

export function createGroupChatState() {
  return reactive({
    initialized: false,
    fromDate: '',
    toDate: '',
    player: '',
    keyword: '',
    result: null,
    searched: null,
    pageSize: DEFAULT_PAGE_SIZE,
    sortOrder: 'asc'
  })
}
