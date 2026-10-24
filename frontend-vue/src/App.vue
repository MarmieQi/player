<script setup>
import { computed, defineAsyncComponent, onMounted, onUnmounted, ref } from 'vue'
import { formatReportTimeRange } from './public-report-format.js'
import { logActionOptions } from './admin/log-action-options.js'
import { DEFAULT_PAGE_SIZE, PAGE_SIZE_OPTIONS, orderedPageRows } from './admin/pagination.js'
import { createLatestRequestScope } from './admin/latest-request.js'
import { createSyncJobPoller } from './admin/sync-job-poller.js'
import { parseCoordinateInput, savedCoordinateInput, coordinateRangeOptions, coordinateQueryParameters } from './admin/coordinate-input.js'
import {
  ArrowLeft,
  ArrowUp,
  Clock,
  Blocks,
  CalendarDays,
  ChevronDown,
  ChevronLeft,
  ChevronRight,
  CheckCircle2,
  CircleSlash,
  Copy,
  Crosshair,
  Crown,
  Database,
  FileText,
  Hammer,
  Layers,
  Play,
  Pickaxe,
  RefreshCw,
  Search,
  Send,
  Sprout,
  Eye,
  EyeOff,
  Save,
  TreePine,
  Trash2,
  XCircle,
  UserRound,
  UsersRound
} from 'lucide-vue-next'
import { apiDelete, apiGet, apiPost, apiPostJson, clearAuthToken, getAuthToken, setAuthToken } from './api'
import { isBucketLogAction, logQueryActionDisplay, logQuerySummaryDetailText, logShareActionClass } from './log-presentation'
import { adminNavigationId } from './admin/navigation'
import { parseTaskLocation, resolveTaskServer, taskLocation } from './admin/task-navigation.js'
import { LOG_QUERY_TYPE_COORDINATE, LOG_QUERY_TYPE_PLAYER_KEYWORD, LOG_QUERY_TYPE_XRAY, normalizeLogQueryMode, logFeatureLocation, parseLogFeatureLocation } from './admin/log-feature-navigation'

const AdminLayout = defineAsyncComponent(() => import('./admin/AdminLayout.vue'))
const PublicReportLayout = defineAsyncComponent(() => import('./components/PublicReportLayout.vue'))
const PublicMetricGrid = defineAsyncComponent(() => import('./components/PublicMetricGrid.vue'))
const AdminLogin = defineAsyncComponent(() => import('./admin/AdminLogin.vue'))
const PlayerNameAutocomplete = defineAsyncComponent(() => import('./admin/PlayerNameAutocomplete.vue'))
const LogActionFilter = defineAsyncComponent(() => import('./admin/LogActionFilter.vue'))
const FileSyncPage = defineAsyncComponent(() => import('./admin/FileSyncPage.vue'))
const GroupChatCopyPage = defineAsyncComponent(() => import('./admin/GroupChatCopyPage.vue'))
const SettingsPage = defineAsyncComponent(() => import('./admin/SettingsPage.vue'))
const settingsPage = ref(null)
const GroupChatPage = defineAsyncComponent(() => import('./admin/GroupChatPage.vue'))
const AdminSelect = defineAsyncComponent(() => import('./admin/AdminSelect.vue'))
const AdminPagination = defineAsyncComponent(() => import('./admin/AdminPagination.vue'))
const LogQueryTabs = defineAsyncComponent(() => import('./admin/LogQueryTabs.vue'))

const loading = ref(false)
const authChecked = ref(false)
const authenticated = ref(Boolean(getAuthToken()))
const loginLoading = ref(false)
const syncingFiles = ref(false)
const syncJob = ref(null)
const syncPageLoading = ref(false)
const remoteFiles = ref([])
const syncLocalFiles = ref([])
const syncSkipToday = ref(true)
const importSkipToday = ref(true)
const importing = ref(false)
const error = ref('')
const loginError = ref('')
const importMessage = ref('')
const currentAdmin = ref('')
const currentAdminExpiresAt = ref(null)
const currentView = ref('dashboard')
const activeAdminPage = computed(() => adminNavigationId(currentView.value))
const shareToken = ref('')
const shareData = ref(null)
const shareLoading = ref(false)
const shareError = ref('')
const xrayShareToken = ref('')
const xrayShareData = ref(null)
const xrayShareLoading = ref(false)
const xrayShareError = ref('')
const xrayShareEvidenceExpanded = ref(false)
const xrayShareEvidenceRowExpandedKeys = ref(new Set())
const xrayShareOrePositionsExpanded = ref(false)
const xrayShareOrePositionPage = ref(1)
const xrayShareOrePositionPageSize = ref(100)
const logShareToken = ref('')
const logShareData = ref(null)
const logShareLoading = ref(false)
const logSharePageLoading = ref(false)
const logShareError = ref('')
const logSharePage = ref(1)
const logSharePageSize = ref(50)
const selectedShareServerId = ref('')
const rankingShareToken = ref('')
const rankingShareData = ref(null)
const rankingShareLoading = ref(false)
const rankingShareError = ref('')
const selectedRankingShareServerId = ref('')
const minecraftLaunchNoticeOpen = ref(false)
const shareNow = ref(Date.now())
const importFiles = ref([])
const importJob = ref(null)
const importPageLoading = ref(false)
const resetRemotePath = ref('')
const deleteRecordJob = ref(null)
const deleteRecordPollError = ref('')
const deletingImportRecords = computed(() => Boolean(resetRemotePath.value) || isActiveJob(deleteRecordJob.value))
const DELETE_JOB_STORAGE_KEY = 'player-stats-last-delete-records-job'
let deleteRecordTimer
let deleteRecordAbort
let deleteRecordVersion = 0
const selectedResetKeys = ref(new Set())
const deletingLocalCsv = ref(false)
const loginForm = ref({
  username: 'admin',
  password: ''
})
const passwordSaving = ref(false)
const showCurrentPassword = ref(false)
const showNewPassword = ref(false)
const showConfirmPassword = ref(false)
const passwordForm = ref({
  currentPassword: '',
  newPassword: '',
  confirmPassword: ''
})
let importJobTimer = null
const syncJobPoller = createSyncJobPoller({
  load: (id, signal) => apiGet(`/api/import/sync-jobs/${encodeURIComponent(id)}`, {}, { signal }),
  onUpdate: job => { syncJob.value = job; syncingFiles.value = isActiveJob(job); error.value = '' },
  onFinished: async (job, request) => {
    importMessage.value = jobSummaryMessage('sync', job)
    await loadSyncLocalFiles(request)
  },
  onError: err => {
    handleApiError(err)
    if (err.status === 401 || err.status === 404) {
      syncingFiles.value = false
      syncJob.value = null
      if (err.status === 404) clearSavedJobId('sync')
      return false
    }
    return true
  }
})
let syncFileLoadVersion = 0
let shareTimer = null
const serverTimeDisplay = ref('')
let serverTimeOffset = 0
let serverTimeTimer = null
const IMPORT_JOB_STORAGE_KEY = 'player-stats-last-import-jobs'
const SYNC_JOB_STORAGE_KEY = 'player-stats-last-sync-jobs'
const LOG_QUERY_FORM_STORAGE_KEY = 'player-stats-log-query-forms'
const LOG_TEXT_QUERY_FORM_STORAGE_KEY = 'player-stats-log-text-query-forms'
const LOG_XRAY_ANALYSIS_FORM_STORAGE_KEY = 'player-stats-log-xray-analysis-forms'
const XRAY_ORE_POSITION_PAGE_SIZE_OPTIONS = PAGE_SIZE_OPTIONS
const LOG_SHARE_PAGE_SIZE_OPTIONS = [50, 100, 200]
const ACTIVE_JOB_STATUSES = ['PENDING', 'RUNNING', 'CANCELLING']

const filters = ref({
  serverId: 'all',
  from: '',
  to: '',
  player: ''
})

const serverOptions = ref([
  { serverId: 'main', serverName: '主服' },
  { serverId: 'sub', serverName: '2服' }
])
const adminServerOptions = computed(() => serverOptions.value.map(server => ({ value: server.serverId, label: server.serverName })))
const serverSummaries = ref([])
const overview = ref(emptyOverview())
const players = ref([])
const daily = ref([])
const imports = ref([])
const logQueryServerId = ref('main')
const logQueryMode = ref(LOG_QUERY_TYPE_COORDINATE)
const lastLogQueryMode = ref(LOG_QUERY_TYPE_COORDINATE)
const logQueryForms = ref(readSavedLogQueryForms())
const logTextQueryForms = ref(readSavedLogTextQueryForms())
const xrayAnalysisForms = ref(readSavedXrayAnalysisForms())
const logQueryResults = ref({})
const xrayAnalysisResults = ref({})
const logQueryLoading = ref({})
const xrayAnalysisLoading = ref({})
const logQueryPages = ref({})
const logQuerySortOrders = ref({})
const logQueryRequests = createLatestRequestScope()
const logQueryPageSizes = ref({})
const selectedLogQueryRow = ref(null)
let logQueryTimer = null

const selectedServerName = computed(() => {
  if (filters.value.serverId === 'all') return '合计'
  return serverOptions.value.find((item) => item.serverId === filters.value.serverId)?.serverName || filters.value.serverId
})

const serverTabs = computed(() => [
  { serverId: 'all', serverName: '合计' },
  ...serverOptions.value
])

const isSharePage = computed(() => Boolean(shareToken.value))
const isXraySharePage = computed(() => Boolean(xrayShareToken.value))
const isLogSharePage = computed(() => Boolean(logShareToken.value))
const isRankingSharePage = computed(() => Boolean(rankingShareToken.value))
const shareServers = computed(() => shareData.value?.servers || [])
const selectedShareServer = computed(() => {
  if (!shareServers.value.length) return null
  return shareServers.value.find((server) => server.serverId === selectedShareServerId.value) || shareServers.value[0]
})
const shareResourceIconAliases = Object.freeze({
  lapis_ore: 'lapis_lazuli_ore',
  deepslate_lapis_ore: 'deepslate_lapis_lazuli_ore',
  quartz_ore: 'nether_quartz_ore'
})
const failedShareResourceIcons = ref(new Set())
function shareResourceIconKey(type) {
  const normalized = String(type || '').trim().toLowerCase().replace(/^minecraft:/, '')
  if (!normalized || !/^[a-z0-9_]+$/.test(normalized)) return ''
  return shareResourceIconAliases[normalized] || normalized
}
function shareResourceIconPath(type) {
  const key = shareResourceIconKey(type)
  if (!key || failedShareResourceIcons.value.has(key)) return ''
  return `/minecraft-icons/${key}.png`
}
function markShareResourceIconFailed(type) {
  const key = shareResourceIconKey(type)
  if (!key || failedShareResourceIcons.value.has(key)) return
  const next = new Set(failedShareResourceIcons.value)
  next.add(key)
  failedShareResourceIcons.value = next
}
const minecraftServers = Object.freeze({
  '主服': { name: 'cat', host: 'cat66.cn', port: 19132 },
  '2服': { name: 'cat生电服', host: 'cat66.cn', port: 19134 }
})
function minecraftJoinUrl(serverName) {
  const server = minecraftServers[serverName]
  if (!server) return '#'
  return `minecraft://?addExternalServer=${encodeURIComponent(server.name)}|${server.host}:${server.port}`
}
const shareMinecraftJoinUrl = computed(() => minecraftJoinUrl(selectedShareServer.value?.serverName))
const xraySharePlayer = computed(() => xrayShareData.value?.player || null)
const xraySharePeriodMetrics = computed(() => {
  const player = xraySharePlayer.value
  return [
    { id: 'breaks', label: '破坏方块', value: formatNumber(xrayAnalysisValue(player, 'analysisBreaks', 'miningSessionBreaks')), icon: 'broken', tone: 'blue' },
    { id: 'underground', label: '地下破坏', value: formatNumber(xrayAnalysisValue(player, 'analysisUndergroundBreaks', 'miningSessionUndergroundBreaks')), icon: 'underground', tone: 'purple' },
    { id: 'rare', label: '稀有矿', value: formatNumber(xrayAnalysisValue(player, 'analysisRareOreBreaks', 'miningSessionRareOreBreaks')), icon: 'ore', tone: 'orange' },
    { id: 'diamond', label: '钻石矿', value: formatNumber(xrayAnalysisValue(player, 'analysisDiamondOreBreaks', 'miningSessionDiamondOreBreaks')), icon: 'blocks', tone: 'blue' },
    { id: 'debris', label: '远古残骸', value: formatNumber(xrayAnalysisValue(player, 'analysisAncientDebrisBreaks', 'miningSessionAncientDebrisBreaks')), icon: 'database', tone: 'orange' },
    { id: 'ratio', label: '地下稀有矿占比', value: xrayAnalysisRareRatioText(player), icon: 'ratio', tone: 'green' },
    { id: 'peak', label: '十分钟挖取峰值', value: formatNumber(xrayAnalysisValue(player, 'analysisPeakRareOreWindowCount', 'peakRareOreWindowCount')), icon: 'clock', tone: 'purple' },
    { id: 'evidence', label: '周期追矿证据', value: formatNumber(xrayAnalysisTrackingEvidenceCount(player)), icon: 'evidence', tone: 'green' }
  ]
})
const xrayShareSessionMetrics = computed(() => {
  const player = xraySharePlayer.value
  return [
    { id: 'breaks', label: '会话破坏方块', value: formatNumber(player?.miningSessionBreaks), icon: 'broken', tone: 'blue' },
    { id: 'underground', label: '地下破坏', value: formatNumber(player?.miningSessionUndergroundBreaks || player?.miningSessionBreaks || 0), icon: 'underground', tone: 'purple' },
    { id: 'rare', label: '稀有矿', value: formatNumber(player?.miningSessionRareOreBreaks), icon: 'ore', tone: 'orange' },
    { id: 'diamond', label: '钻石矿', value: formatNumber(player?.miningSessionDiamondOreBreaks), icon: 'blocks', tone: 'blue' },
    { id: 'debris', label: '远古残骸', value: formatNumber(player?.miningSessionAncientDebrisBreaks), icon: 'database', tone: 'orange' },
    { id: 'ratio', label: '地下稀有矿占比', value: xrayRareRatioText(player), icon: 'ratio', tone: 'green' },
    { id: 'peak', label: '十分钟挖取峰值', value: formatNumber(player?.peakRareOreWindowCount), icon: 'clock', tone: 'purple' },
    { id: 'evidence', label: '会话追矿证据', value: formatNumber(player?.trackingEvidenceCount), icon: 'evidence', tone: 'green' }
  ]
})

const shareRemainingMs = computed(() => {
  if (!shareData.value?.expiresAt) return 0
  return Math.max(0, new Date(shareData.value.expiresAt).getTime() - shareNow.value)
})
const shareRemainingText = computed(() => {
  if (shareError.value) return '链接已过期'
  if (!shareData.value?.expiresAt) {
    return shareData.value?.ttlMinutes ? `链接${durationText(shareData.value.ttlMinutes)}内有效` : '链接有效期内可访问'
  }
  const totalSeconds = Math.ceil(shareRemainingMs.value / 1000)
  if (totalSeconds <= 0) return '链接已过期'
  const minutes = Math.floor(totalSeconds / 60)
  const seconds = totalSeconds % 60
  if (minutes > 0) return `剩余 ${minutes} 分钟`
  return `剩余 ${seconds} 秒`
})
const xrayShareRemainingMs = computed(() => {
  if (!xrayShareData.value?.expiresAt) return 0
  return Math.max(0, new Date(xrayShareData.value.expiresAt).getTime() - shareNow.value)
})
const xrayShareRemainingText = computed(() => {
  if (xrayShareError.value) return '链接已过期'
  if (!xrayShareData.value?.expiresAt) {
    return xrayShareData.value?.ttlMinutes ? `链接 ${durationText(xrayShareData.value.ttlMinutes)}内有效` : '链接有效期内可访问'
  }
  const totalSeconds = Math.ceil(xrayShareRemainingMs.value / 1000)
  if (totalSeconds <= 0) return '链接已过期'
  const minutes = Math.floor(totalSeconds / 60)
  const seconds = totalSeconds % 60
  if (minutes > 0) return `剩余 ${minutes} 分钟`
  return `剩余 ${seconds} 秒`
})
const logShareRows = computed(() => logShareData.value?.rows || [])
const logShareTotalRows = computed(() => Number(logShareData.value?.totalRows ?? logShareData.value?.displayedRows ?? logShareRows.value.length))
const logShareTotalPages = computed(() => Math.max(1, Number(logShareData.value?.totalPages) || Math.ceil(logShareTotalRows.value / logSharePageSize.value)))
const logSharePagedRows = computed(() => logShareRows.value)
const logShareRemainingMs = computed(() => {
  if (!logShareData.value?.expiresAt) return 0
  return Math.max(0, new Date(logShareData.value.expiresAt).getTime() - shareNow.value)
})
const logShareRemainingText = computed(() => {
  if (logShareError.value) return '链接已过期'
  if (!logShareData.value?.expiresAt) return '链接有效期内可访问'
  const totalSeconds = Math.ceil(logShareRemainingMs.value / 1000)
  if (totalSeconds <= 0) return '链接已过期'
  const minutes = Math.floor(totalSeconds / 60)
  return minutes > 0 ? `剩余 ${minutes} 分钟` : `剩余 ${totalSeconds % 60} 秒`
})
const rankingShareServers = computed(() => rankingShareData.value?.servers || [])
const selectedRankingShareServer = computed(() => {
  if (!rankingShareServers.value.length) return null
  return rankingShareServers.value.find((s) => s.serverId === selectedRankingShareServerId.value) || rankingShareServers.value[0]
})
const rankingMinecraftJoinUrl = computed(() => minecraftJoinUrl(selectedRankingShareServer.value?.serverName))
const rankingShareRemainingMs = computed(() => {
  if (!rankingShareData.value?.expiresAt) return 0
  return Math.max(0, new Date(rankingShareData.value.expiresAt).getTime() - shareNow.value)
})
const rankingShareTitle = computed(() => {
  if (rankingShareData.value?.rankingType === 'active') return '活跃榜'
  return '肝帝榜'
})
const isActiveRankingShare = computed(() => rankingShareData.value?.rankingType === 'active')
const rankingSharePeriodDays = computed(() => {
  if (!rankingShareData.value?.fromDate || !rankingShareData.value?.toDate) return 0
  const from = new Date(`${rankingShareData.value.fromDate}T00:00:00`)
  const to = new Date(`${rankingShareData.value.toDate}T00:00:00`)
  if (Number.isNaN(from.getTime()) || Number.isNaN(to.getTime())) return 0
  return Math.max(1, Math.round((to.getTime() - from.getTime()) / 86400000) + 1)
})
const rankingSharePlayers = computed(() => selectedRankingShareServer.value?.players || [])
const rankingShareTotals = computed(() => rankingSharePlayers.value.reduce((totals, player) => ({
  brokenCount: totals.brokenCount + Number(player?.brokenCount || 0),
  placedCount: totals.placedCount + Number(player?.placedCount || 0),
  totalCount: totals.totalCount + Number(player?.totalCount || 0)
}), { brokenCount: 0, placedCount: 0, totalCount: 0 }))

const maxDailyTotal = computed(() => {
  const max = Math.max(...daily.value.map((item) => item.totalCount || 0), 0)
  return max || 1
})

const leadingPlayers = computed(() => players.value.slice(0, 8))
const importJobRunning = computed(() => importJob.value?.status === 'RUNNING' || importJob.value?.status === 'PENDING')
const importPageFiles = computed(() => importJobRunning.value ? (importJob.value?.files || importFiles.value) : importFiles.value)
const syncJobRunning = computed(() => syncJob.value?.status === 'RUNNING' || syncJob.value?.status === 'PENDING')
const selectedLogQueryResult = computed(() => logQueryResultFor(logQueryServerId.value) || null)
const selectedXrayAnalysisResult = computed(() => xrayAnalysisResultFor(logQueryServerId.value) || null)
const selectedLogFeatureResult = computed(() => (
  logQueryMode.value === LOG_QUERY_TYPE_XRAY ? selectedXrayAnalysisResult.value : selectedLogQueryResult.value
))
const selectedLogQueryRows = computed(() => selectedLogQueryResult.value?.rows || [])
const xrayDetailPlayerName = ref('')
const xrayDetailEvidenceExpanded = ref(false)
const xrayDetailEvidenceRowExpandedKeys = ref(new Set())
const xrayDetailOrePositionsExpanded = ref(false)
const xrayDetailOrePositionPage = ref(1)
const xrayDetailSortOrder = ref('asc')
const xrayDetailOrePositionPageSize = ref(DEFAULT_PAGE_SIZE)
const xrayGroupSendDialogOpen = ref(false)
const xrayGroupSendLoading = ref(false)
const xrayGroupSendTtlText = ref('')
const xrayGroupSendMessage = ref('')
const xrayGroupSendError = ref('')
const selectedXrayDetailPlayer = computed(() => {
  if (!xrayDetailPlayerName.value) return null
  return (selectedXrayAnalysisResult.value?.players || [])
    .find((player) => player.playerName === xrayDetailPlayerName.value) || null
})
const xrayShareRareOreRows = computed(() => xraySharePlayer.value?.rareOreRows || [])
const xrayDetailRareOreRows = computed(() => selectedXrayDetailPlayer.value?.rareOreRows || [])
const xrayShareOrePositionTotalPages = computed(() => xrayPageCount(xrayShareRareOreRows.value.length, xrayShareOrePositionPageSize.value))
const xrayDetailOrePositionTotalPages = computed(() => xrayPageCount(xrayDetailRareOreRows.value.length, xrayDetailOrePositionPageSize.value))
const xraySharePagedRareOreRows = computed(() => pagedXrayRows(
  xrayShareRareOreRows.value,
  xrayShareOrePositionPage.value,
  xrayShareOrePositionPageSize.value
))
const xrayDetailPagedRareOreRows = computed(() => orderedPageRows(
  xrayDetailRareOreRows.value,
  xrayDetailOrePositionPage.value,
  xrayDetailOrePositionPageSize.value,
  xrayDetailSortOrder.value
))
const xrayShareOrePositionOffset = computed(() => (clampPage(xrayShareOrePositionPage.value, xrayShareOrePositionTotalPages.value) - 1) * xrayShareOrePositionPageSize.value)
const xrayDetailOrePositionOffset = computed(() => (clampPage(xrayDetailOrePositionPage.value, xrayDetailOrePositionTotalPages.value) - 1) * xrayDetailOrePositionPageSize.value)
const xrayDetailResult = computed(() => xrayAnalysisResultFor(logQueryServerId.value) || null)
const logQueryCancelRequests = ref({})
const selectedLogQueryCancelling = computed(() => selectedLogFeatureResult.value?.status === 'CANCELLING' || Boolean(logQueryCancelRequests.value[logQueryStateKey(logQueryServerId.value, logQueryMode.value)]))
const selectedLogQueryRunning = computed(() => isActiveJob(selectedLogFeatureResult.value))
const selectedLogFeatureHasResult = computed(() => {
  const status = selectedLogFeatureResult.value?.status
  return Boolean(status && String(status).toUpperCase() !== 'IDLE')
})
const selectedLogQueryLoading = computed(() => (
  logQueryMode.value === LOG_QUERY_TYPE_XRAY
    ? Boolean(xrayAnalysisLoading.value[logQueryStateKey(logQueryServerId.value, LOG_QUERY_TYPE_XRAY)])
    : Boolean(logQueryLoading.value[logQueryStateKey(logQueryServerId.value)])
))
const selectedLogQuerySortOrder = computed(() => logQuerySortOrders.value[logQueryStateKey(logQueryServerId.value)] || 'asc')
const selectedLogQueryPage = computed(() => selectedLogQueryResult.value?.page || logQueryPages.value[logQueryStateKey(logQueryServerId.value)] || 1)
const selectedLogQueryPageSize = computed(() => selectedLogQueryResult.value?.pageSize || logQueryPageSizes.value[logQueryStateKey(logQueryServerId.value)] || DEFAULT_PAGE_SIZE)
const selectedLogQueryTotalPages = computed(() => selectedLogQueryResult.value?.totalPages || 0)
const eligibleImportFiles = computed(() => importPageFiles.value.filter(canDeleteImportRecord))
const selectedResetCount = computed(() => selectedResetKeys.value.size)
const allEligibleSelected = computed(() => {
  const files = eligibleImportFiles.value
  return files.length > 0 && files.every((file) => selectedResetKeys.value.has(importFileKey(file)))
})
onMounted(() => {
  if (openSharePageFromPath()) {
    return
  }
  window.addEventListener('hashchange', handleAdminHashChange)
  bootstrapAuth()
})

onUnmounted(() => {
  stopDeleteRecordPolling()
  window.removeEventListener('hashchange', handleAdminHashChange)
  stopShareTicker()
  stopServerTimeTicker()
  stopImportJobPolling()
  stopSyncJobPolling()
  stopLogQueryPolling()
  logQueryRequests.dispose()
})

function openSharePageFromPath() {
  const xrayMatch = window.location.pathname.match(/^\/xray-share\/([^/?#]+)/)
  if (xrayMatch) {
    xrayShareToken.value = decodeURIComponent(xrayMatch[1])
    authChecked.value = true
    loadXrayShareDetails()
    return true
  }

  const logMatch = window.location.pathname.match(/^\/share\/logs\/([^/?#]+)/)
  if (logMatch) {
    logShareToken.value = decodeURIComponent(logMatch[1])
    authChecked.value = true
    loadLogShareDetails()
    return true
  }

  const rankingMatch = window.location.pathname.match(/^\/share\/ranking\/([^/?#]+)/)
  if (rankingMatch) {
    rankingShareToken.value = decodeURIComponent(rankingMatch[1])
    authChecked.value = true
    loadRankingShareDetails()
    return true
  }

  const match = window.location.pathname.match(/^\/share\/([^/?#]+)/)
  if (!match) {
    return false
  }
  shareToken.value = decodeURIComponent(match[1])
  authChecked.value = true
  loadShareDetails()
  return true
}

async function loadLogShareDetails() {
  const initialLoad = !logShareData.value
  logShareLoading.value = initialLoad
  logSharePageLoading.value = !initialLoad
  logShareError.value = ''
  try {
    if (import.meta.env.DEV && logShareToken.value === 'preview') {
      const { createLogSharePreviewData } = await import('./log-share-preview')
      const preview = createLogSharePreviewData()
      const totalRows = preview.rows.length
      const totalPages = Math.max(1, Math.ceil(totalRows / logSharePageSize.value))
      logSharePage.value = clampPage(logSharePage.value, totalPages)
      const start = (logSharePage.value - 1) * logSharePageSize.value
      logShareData.value = {
        ...preview,
        rows: preview.rows.slice(start, start + logSharePageSize.value),
        page: logSharePage.value,
        pageSize: logSharePageSize.value,
        totalPages,
        totalRows
      }
    } else {
      logShareData.value = await apiGet(`/api/share/log-query/${encodeURIComponent(logShareToken.value)}`, {
        page: logSharePage.value,
        pageSize: logSharePageSize.value
      }, { auth: false })
      logSharePage.value = Number(logShareData.value.page) || 1
      logSharePageSize.value = Number(logShareData.value.pageSize) || logSharePageSize.value
    }
    startShareTicker()
  } catch (err) {
    logShareData.value = null
    logShareError.value = err.message || '链接已过期，请重新在群里查询'
    stopShareTicker()
  } finally {
    logShareLoading.value = false
    logSharePageLoading.value = false
  }
}

async function loadXrayShareDetails() {
  xrayShareLoading.value = true
  xrayShareError.value = ''
  try {
    await import('./xray-share.css')
    const data = await apiGet(`/api/share/xray/${encodeURIComponent(xrayShareToken.value)}`, {}, { auth: false })
    xrayShareData.value = data
    xrayShareEvidenceExpanded.value = false
    xrayShareEvidenceRowExpandedKeys.value = new Set()
    xrayShareOrePositionsExpanded.value = false
    xrayShareOrePositionPage.value = 1
    startShareTicker()
  } catch (err) {
    xrayShareData.value = null
    xrayShareError.value = err.message || '链接已过期，请重新生成'
    stopShareTicker()
  } finally {
    xrayShareLoading.value = false
  }
}

async function loadRankingShareDetails() {
  rankingShareLoading.value = true
  rankingShareError.value = ''
  try {
    const data = await apiGet(`/api/share/ranking/${encodeURIComponent(rankingShareToken.value)}`, {}, { auth: false })
    rankingShareData.value = data
    selectedRankingShareServerId.value = data.servers?.[0]?.serverId || ''
    startShareTicker()
  } catch (err) {
    rankingShareData.value = null
    rankingShareError.value = err.message || '链接已过期，请重新在群里查询'
    stopShareTicker()
  } finally {
    rankingShareLoading.value = false
  }
}

async function loadShareDetails() {
  shareLoading.value = true
  shareError.value = ''
  try {
    const data = import.meta.env.DEV && shareToken.value === 'preview'
      ? (await import('./player-share-preview')).createPlayerSharePreviewData()
      : await apiGet(`/api/share/${encodeURIComponent(shareToken.value)}`, {}, { auth: false })
    shareData.value = data
    selectedShareServerId.value = data.servers?.[0]?.serverId || ''
    startShareTicker()
  } catch (err) {
    shareData.value = null
    shareError.value = err.message || '链接已过期，请重新在群里查询'
    stopShareTicker()
  } finally {
    shareLoading.value = false
  }
}

function startShareTicker() {
  stopShareTicker()
  shareNow.value = Date.now()
  shareTimer = window.setInterval(() => {
    shareNow.value = Date.now()
    if (shareData.value?.expiresAt && shareRemainingMs.value <= 0) {
      shareError.value = '链接已过期，请重新在群里查询'
      stopShareTicker()
    }
    if (xrayShareData.value?.expiresAt && xrayShareRemainingMs.value <= 0) {
      xrayShareError.value = '链接已过期，请重新生成'
      stopShareTicker()
    }
    if (rankingShareData.value?.expiresAt && rankingShareRemainingMs.value <= 0) {
      rankingShareError.value = '链接已过期，请重新在群里查询'
      stopShareTicker()
    }
    if (logShareData.value?.expiresAt && logShareRemainingMs.value <= 0) {
      logShareError.value = '链接已过期，请重新在群里查询'
      stopShareTicker()
    }
  }, 1000)
}

function stopShareTicker() {
  if (shareTimer) {
    window.clearInterval(shareTimer)
    shareTimer = null
  }
}

async function bootstrapAuth() {
  if (!getAuthToken()) {
    authChecked.value = true
    return
  }

  try {
    const admin = await apiGet('/api/auth/me')
    currentAdmin.value = admin.username
    currentAdminExpiresAt.value = admin.expiresAt
    authenticated.value = true
    await syncServerTime()
    await loadAll()
    await openInitialViewFromHash()
  } catch (err) {
    clearAuthToken()
    authenticated.value = false
  } finally {
    authChecked.value = true
  }
}

async function login() {
  loginLoading.value = true
  loginError.value = ''
  try {
    const result = await apiPostJson('/api/auth/login', loginForm.value, { auth: false })
    setAuthToken(result.token)
    currentAdmin.value = result.username
    currentAdminExpiresAt.value = result.expiresAt
    authenticated.value = true
    await syncServerTime()
    await loadAll()
    await openInitialViewFromHash()
  } catch (err) {
    loginError.value = err.message
  } finally {
    loginLoading.value = false
  }
}

async function logout() {
  if (currentView.value === 'settingsPage' && settingsPage.value && !settingsPage.value.canLeave()) return
  stopDeleteRecordPolling()
  deleteRecordJob.value = null
  resetRemotePath.value = ''
  stopImportJobPolling()
  stopSyncJobPolling()
  stopLogQueryPolling()
  logQueryRequests.dispose()
  logQueryLoading.value = {}
  stopServerTimeTicker()
  await apiPost('/api/auth/logout').catch(() => {})
  clearAuthToken()
  authenticated.value = false
  currentAdmin.value = ''
  currentAdminExpiresAt.value = null
  importMessage.value = ''
  error.value = ''
  loginForm.value.password = ''
}

async function loadAll() {
  loading.value = true
  error.value = ''
  try {
    const baseParams = {
      serverId: filters.value.serverId === 'all' ? '' : filters.value.serverId,
      from: filters.value.from,
      to: filters.value.to,
      player: filters.value.player
    }
    const [optionsData, serverData, overviewData, playerData, dailyData, importData] = await Promise.all([
      apiGet('/api/stats/server-options'),
      apiGet('/api/stats/servers', { from: filters.value.from, to: filters.value.to }),
      apiGet('/api/stats/overview', baseParams),
      apiGet('/api/stats/players', { ...baseParams, limit: 100 }),
      apiGet('/api/stats/daily', baseParams),
      apiGet('/api/stats/imports', { serverId: baseParams.serverId, limit: 20 })
    ])
    const options = asArray(optionsData)
    serverOptions.value = options.length ? options : serverOptions.value
    serverSummaries.value = asArray(serverData)
    overview.value = overviewData || emptyOverview()
    players.value = asArray(playerData)
    daily.value = asArray(dailyData)
    imports.value = asArray(importData)
  } catch (err) {
    handleApiError(err)
  } finally {
    loading.value = false
  }
}

async function openImportPage(serverId = filters.value.serverId) {
  const targetServerId = resolveTaskServer(serverId, filters.value.serverId, serverOptions.value)

  filters.value.serverId = targetServerId
  currentView.value = 'importPage'
  window.location.hash = taskLocation('import', targetServerId)
  stopImportJobPolling()
  importJob.value = null
  importing.value = false
  importMessage.value = ''
  error.value = ''
  await loadImportFiles()
  await restoreImportJob()
  await restoreDeleteRecordJob()
}

async function loadImportFiles({ resetJob = false } = {}) {
  importPageLoading.value = true
  error.value = ''
  try {
    importFiles.value = asArray(await apiGet('/api/import/files', {
      serverId: filters.value.serverId === 'all' ? '' : filters.value.serverId
    }))
    if (resetJob) {
      importJob.value = null
      importing.value = false
    }
    selectedResetKeys.value = new Set()
  } catch (err) {
    handleApiError(err)
  } finally {
    importPageLoading.value = false
  }
}

async function startImportJob() {
  if (deletingImportRecords.value) return
  if (filters.value.serverId === 'all') {
    error.value = '请先选择主服或 2服，再解析 CSV'
    return
  }

  importing.value = true
  error.value = ''
  importMessage.value = ''
  try {
    const job = await apiPost('/api/import/jobs', {
      serverId: filters.value.serverId === 'all' ? '' : filters.value.serverId,
      skipToday: importSkipToday.value
    })
    importJob.value = job
    saveJobId('import', job.jobId)
    importMessage.value = '解析任务已开始'
    pollImportJob(job.jobId)
  } catch (err) {
    handleApiError(err)
  } finally {
    if (!importJobRunning.value) {
      importing.value = false
    }
  }
}

function pollImportJob(jobId) {
  stopImportJobPolling()
  importJobTimer = window.setInterval(async () => {
    try {
      const job = await apiGet(`/api/import/jobs/${jobId}`)
      importJob.value = job
      if (!isActiveJob(job)) {
        stopImportJobPolling()
        importing.value = false
        importMessage.value = jobSummaryMessage('import', job)
        await loadAll()
        await loadImportFiles()
      }
    } catch (err) {
      stopImportJobPolling()
      importing.value = false
      handleApiError(err)
    }
  }, 1000)
}

function stopImportJobPolling() {
  if (importJobTimer) {
    window.clearInterval(importJobTimer)
    importJobTimer = null
  }
}

async function restoreImportJob() {
  const jobId = savedJobId('import')
  if (!jobId) {
    return
  }

  try {
    const job = await apiGet(`/api/import/jobs/${jobId}`)
    importJob.value = job
    importing.value = isActiveJob(job)
    importMessage.value = isActiveJob(job) ? '解析任务进行中' : jobSummaryMessage('import', job)
    if (isActiveJob(job)) {
      pollImportJob(job.jobId)
    }
  } catch (err) {
    clearSavedJobId('import')
  }
}

async function deleteImportRecord(file) {
  if (deletingImportRecords.value) return
  if (!file?.imported && !['IMPORTED', 'CHANGED', 'NEEDS_IMPORT'].includes(file?.status)) {
    return
  }
  const ok = window.confirm(`只删除数据库中的解析记录，不删除本地 CSV：\n${fileNameFromPath(file.remotePath)}\n\n删除后可以重新解析这个文件。确定继续吗？`)
  if (!ok) {
    return
  }

  resetRemotePath.value = file.remotePath
  error.value = ''
  try {
    await apiDelete('/api/import/files', {
      serverId: file.serverId,
      remotePath: file.remotePath
    })
    importMessage.value = '已删除数据库解析记录，可以重新解析'
    await loadImportFiles({ resetJob: true })
    await loadAll()
  } catch (err) {
    handleApiError(err)
  } finally {
    resetRemotePath.value = ''
  }
}

function stopDeleteRecordPolling() {
  ++deleteRecordVersion
  clearTimeout(deleteRecordTimer)
  deleteRecordTimer = null
  deleteRecordAbort?.abort()
  deleteRecordAbort = null
}

function pollDeleteRecordJob(jobId) {
  stopDeleteRecordPolling()
  const version = deleteRecordVersion
  async function tick() {
    deleteRecordAbort = new AbortController()
    try {
      const job = await apiGet(`/api/import/delete-jobs/${encodeURIComponent(jobId)}`, {}, { signal: deleteRecordAbort.signal })
      if (version !== deleteRecordVersion) return
      deleteRecordJob.value = job
      deleteRecordPollError.value = ''
      if (isActiveJob(job)) {
        resetRemotePath.value = '__batch__'
        deleteRecordTimer = setTimeout(tick, 1000)
        return
      }
      resetRemotePath.value = ''
      localStorage.removeItem(DELETE_JOB_STORAGE_KEY)
      importMessage.value = job.message || `已删除 ${job.deletedFiles} 个数据库解析记录${job.failedFiles ? `，失败 ${job.failedFiles} 个` : ''}`
      selectedResetKeys.value = new Set()
      await loadImportFiles({ resetJob: true })
      if (version !== deleteRecordVersion) return
      await loadAll()
      if (version === deleteRecordVersion && job.status === 'FAILED') error.value = importMessage.value
    } catch (err) {
      if (version !== deleteRecordVersion || err.name === 'AbortError') return
      if (err.status === 404 || err.status === 401) {
        localStorage.removeItem(DELETE_JOB_STORAGE_KEY)
        deleteRecordJob.value = null
        resetRemotePath.value = ''
        if (err.status === 404) await loadImportFiles({ resetJob: true })
        deleteRecordPollError.value = err.message
        return
      }
      deleteRecordPollError.value = '暂时无法获取删除进度，正在重试'
      deleteRecordTimer = setTimeout(tick, 3000)
    }
  }
  tick()
}

async function restoreDeleteRecordJob() {
  const version = deleteRecordVersion
  const saved = localStorage.getItem(DELETE_JOB_STORAGE_KEY)
  try {
    const job = await apiGet(`/api/import/delete-jobs/${encodeURIComponent(saved || 'latest')}`)
    if (version !== deleteRecordVersion || !authenticated.value) return
    if (isActiveJob(job) || saved) {
      deleteRecordJob.value = job
      resetRemotePath.value = '__batch__'
      localStorage.setItem(DELETE_JOB_STORAGE_KEY, job.jobId)
      pollDeleteRecordJob(job.jobId)
    }
  } catch (err) {
    if (version !== deleteRecordVersion || !authenticated.value) return
    if (err.status === 404) {
      localStorage.removeItem(DELETE_JOB_STORAGE_KEY)
      if (saved) deleteRecordPollError.value = '删除任务已结束或服务已重启，请以文件列表为准'
    } else handleApiError(err)
  }
}

async function deleteSelectedImportRecords() {
  if (deletingImportRecords.value) return
  const files = importPageFiles.value.filter(file => selectedResetKeys.value.has(importFileKey(file)))
  if (!files.length) return
  if (!window.confirm(`只删除数据库中的解析记录，不删除本地 CSV。\n本次将删除 ${files.length} 个文件的解析记录，删除后可以重新解析。确定继续吗？`)) return
  stopDeleteRecordPolling()
  const version = deleteRecordVersion
  resetRemotePath.value = '__batch__'
  deleteRecordJob.value = null
  deleteRecordPollError.value = ''
  error.value = ''
  deleteRecordAbort = new AbortController()
  try {
    const job = await apiPostJson('/api/import/delete-jobs', {
      files: files.map(file => ({ serverId: file.serverId, remotePath: file.remotePath }))
    }, { signal: deleteRecordAbort.signal })
    if (version !== deleteRecordVersion) return
    deleteRecordJob.value = job
    localStorage.setItem(DELETE_JOB_STORAGE_KEY, job.jobId)
    pollDeleteRecordJob(job.jobId)
  } catch (err) {
    if (version !== deleteRecordVersion || err.name === 'AbortError') return
    resetRemotePath.value = ''
    handleApiError(err)
    // An accepted job may outlive a lost acknowledgement. Discover it before retrying.
    await restoreDeleteRecordJob()
    if (version === deleteRecordVersion) await loadImportFiles()
  }
}

async function openSyncPage(serverId = filters.value.serverId) {
  const targetServerId = resolveTaskServer(serverId, filters.value.serverId, serverOptions.value)

  filters.value.serverId = targetServerId
  currentView.value = 'syncPage'
  window.location.hash = taskLocation('sync', targetServerId)
  stopSyncJobPolling()
  syncJob.value = null
  syncingFiles.value = false
  remoteFiles.value = []
  syncLocalFiles.value = []
  importMessage.value = ''
  error.value = ''
  await loadSyncPageFiles()
  await restoreSyncJob()
}

async function loadSyncPageFiles() {
  const serverId = filters.value.serverId
  const version = ++syncFileLoadVersion
  const isCurrent = () => version === syncFileLoadVersion && filters.value.serverId === serverId && currentView.value === 'syncPage'
  syncPageLoading.value = true
  error.value = ''
  try {
    const params = {
      serverId
    }
    const [remoteData, localData] = await Promise.all([
      apiGet('/api/import/remote-files', params),
      apiGet('/api/import/files', params)
    ])
    if (!isCurrent()) return
    remoteFiles.value = asArray(remoteData)
    syncLocalFiles.value = asArray(localData)
  } catch (err) {
    if (isCurrent()) handleApiError(err)
  } finally {
    if (isCurrent()) syncPageLoading.value = false
  }
}

async function startSyncJob() {
  if (filters.value.serverId === 'all') {
    error.value = '请先选择主服或 2服，再复制 CSV'
    return
  }

  syncingFiles.value = true
  error.value = ''
  importMessage.value = ''
  try {
    const job = await apiPost('/api/import/sync-jobs', {
      serverId: filters.value.serverId === 'all' ? '' : filters.value.serverId,
      skipToday: syncSkipToday.value
    })
    syncJob.value = job
    saveJobId('sync', job.jobId)
    importMessage.value = '复制任务已开始'
    pollSyncJob(job.jobId)
  } catch (err) {
    handleApiError(err)
  } finally {
    if (!syncJobRunning.value) {
      syncingFiles.value = false
    }
  }
}

function pollSyncJob(jobId) {
  syncJobPoller.start(jobId)
}

function stopSyncJobPolling() {
  syncJobPoller.stop()
}

async function loadSyncLocalFiles({ signal, isCurrent = () => true } = {}) {
  const serverId = filters.value.serverId
  try {
    const files = await apiGet('/api/import/files', { serverId }, { signal })
    if (isCurrent() && filters.value.serverId === serverId && currentView.value === 'syncPage') syncLocalFiles.value = asArray(files)
  } catch (err) {
    if (isCurrent() && err.name !== 'AbortError') handleApiError(err)
  }
}

async function deleteSelectedLocalCsv(files) {
  if (!files.length) {
    return
  }

  const ok = window.confirm(`只删除后端本地已复制的 CSV，不删除远程 SMB 文件。\n本次将删除 ${files.length} 个本地 CSV。确定继续吗？`)
  if (!ok) {
    return
  }

  deletingLocalCsv.value = true
  error.value = ''
  importMessage.value = ''
  try {
    const results = await apiPostJson('/api/import/local-files/delete', {
      files: files.map(file => ({ serverId: file.serverId, remotePath: file.remotePath }))
    })
    const deletedCount = results.filter((result) => result.deleted).length
    const failedCount = results.length - deletedCount
    importMessage.value = `已删除 ${deletedCount} 个本地 CSV${failedCount ? `，失败 ${failedCount} 个` : ''}`
    await loadSyncPageFiles()
  } catch (err) {
    handleApiError(err)
  } finally {
    deletingLocalCsv.value = false
  }
}

async function restoreSyncJob() {
  const serverId = filters.value.serverId
  const jobId = savedJobId('sync')
  if (!jobId) {
    return
  }

  try {
    const job = await apiGet(`/api/import/sync-jobs/${jobId}`)
    if (filters.value.serverId !== serverId || currentView.value !== 'syncPage' || syncingFiles.value || (syncJob.value?.jobId && syncJob.value.jobId !== jobId)) return
    syncJob.value = job
    syncingFiles.value = isActiveJob(job)
    importMessage.value = isActiveJob(job) ? '复制任务进行中' : jobSummaryMessage('sync', job)
    if (isActiveJob(job)) {
      pollSyncJob(job.jobId)
    }
  } catch (err) {
    if (filters.value.serverId !== serverId || currentView.value !== 'syncPage') return
    if (err.status === 404) clearSavedJobId('sync')
    else {
      handleApiError(err)
      if (err.status !== 401) {
        syncJob.value = { jobId, status: 'RUNNING', files: [] }
        syncingFiles.value = true
        pollSyncJob(jobId)
      }
    }
  }
}

async function openSettingsPage() {
  currentView.value = 'settingsPage'
  window.location.hash = '#/settings'
  error.value = ''
  importMessage.value = ''
}

function openProfilePage() {
  currentView.value = 'profilePage'
  window.location.hash = '#/profile'
  error.value = ''
  importMessage.value = ''
  resetPasswordForm()
}

async function openLogQueryPage(serverId = filters.value.serverId, queryType = lastLogQueryMode.value) {
  const mode = normalizeLogQueryMode(queryType)
  lastLogQueryMode.value = mode
  await openLogFeaturePage(serverId, mode)
}

async function openXrayAnalysisPage(serverId = filters.value.serverId) {
  await openLogFeaturePage(serverId, LOG_QUERY_TYPE_XRAY)
}

// Separate page entry points share task loading, forms and cached results.
async function openLogFeaturePage(serverId, queryType) {
  stopLogQueryPolling()
  currentView.value = queryType === LOG_QUERY_TYPE_XRAY ? 'xrayAnalysisPage' : 'logQueryPage'
  error.value = ''
  importMessage.value = ''
  logQueryServerId.value = serverId && serverId !== 'all' ? serverId : 'main'
  ensureLogFeatureForms(logQueryServerId.value)
  logQueryMode.value = queryType
  window.location.hash = logFeatureLocation(logQueryServerId.value, queryType)
  closeXrayPlayerDetail()
  await loadAllLogQueryStates(queryType)
}

async function loadAllLogQueryStates(queryType = logQueryMode.value) {
  await Promise.all(serverOptions.value.map((server) => loadLogFeatureState(server.serverId, true, queryType)))
  startLogQueryPollingIfNeeded()
}

async function loadLogFeatureState(serverId, silent = false, queryType = logQueryMode.value) {
  if (queryType === LOG_QUERY_TYPE_XRAY) {
    return loadXrayAnalysisState(serverId, silent)
  }
  return loadLogQueryState(serverId, silent, queryType)
}

async function loadLogQueryState(serverId, silent = false, queryType = logQueryMode.value, overrides = {}) {
  const key = logQueryStateKey(serverId, queryType)
  if (silent && logQueryLoading.value[key]) return
  const request = logQueryRequests.begin(key)
  if (!silent) logQueryLoading.value = { ...logQueryLoading.value, [key]: true }
  try {
    const result = await apiGet('/api/import/log-query-jobs/latest', {
      serverId, queryType,
      page: logQueryPages.value[key] || 1,
      pageSize: logQueryPageSizes.value[key] || DEFAULT_PAGE_SIZE,
      sortOrder: logQuerySortOrders.value[key] || 'asc',
      ...overrides
    }, { signal: request.signal })
    if (!logQueryRequests.isCurrent(key, request)) return
    logQueryPages.value = { ...logQueryPages.value, [key]: result.page || 1 }
    logQueryPageSizes.value = { ...logQueryPageSizes.value, [key]: result.pageSize || DEFAULT_PAGE_SIZE }
    logQuerySortOrders.value = { ...logQuerySortOrders.value, [key]: result.sortOrder || 'asc' }
    if (result.status === 'IDLE' && logQueryResults.value[key]?.status !== 'IDLE') resetLogQueryDates(serverId, queryType)
    logQueryResults.value = { ...logQueryResults.value, [key]: result }
  } catch (err) {
    if (logQueryRequests.isCurrent(key, request) && err.name !== 'AbortError') handleApiError(err)
  } finally {
    if (logQueryRequests.isCurrent(key, request)) {
      logQueryLoading.value = { ...logQueryLoading.value, [key]: false }
      logQueryRequests.finish(key, request)
    }
  }
}

async function loadXrayAnalysisState(serverId, silent = false) {
  const key = logQueryStateKey(serverId, LOG_QUERY_TYPE_XRAY)
  if (!silent) {
    xrayAnalysisLoading.value = { ...xrayAnalysisLoading.value, [key]: true }
  }
  try {
    const result = await apiGet('/api/import/xray-analysis-jobs/latest', { serverId })
    xrayAnalysisResults.value = { ...xrayAnalysisResults.value, [key]: result }
  } catch (err) {
    handleApiError(err)
  } finally {
    if (!silent) {
      xrayAnalysisLoading.value = { ...xrayAnalysisLoading.value, [key]: false }
    }
  }
}

async function startLogQuery(serverId) {
  const queryType = logQueryMode.value
  const key = logQueryStateKey(serverId, queryType)
  logQueryRequests.cancel(key)
  error.value = ''
  importMessage.value = ''
  logQueryPages.value = { ...logQueryPages.value, [key]: 1 }
  if (queryType === LOG_QUERY_TYPE_XRAY) {
    closeXrayPlayerDetail()
    xrayAnalysisLoading.value = { ...xrayAnalysisLoading.value, [key]: true }
  } else {
    logQueryLoading.value = { ...logQueryLoading.value, [key]: true }
  }
  try {
    if (queryType === LOG_QUERY_TYPE_XRAY) {
      const result = await apiPostJson('/api/import/xray-analysis-jobs', xrayAnalysisRequestBody(serverId))
      xrayAnalysisResults.value = { ...xrayAnalysisResults.value, [key]: result }
      importMessage.value = `${serverNameById(serverId)} 矿透分析已开始`
    } else {
      const body = logQueryRequestBody(serverId, queryType)
      const result = await apiPostJson('/api/import/log-query-jobs', body)
      logQueryResults.value = { ...logQueryResults.value, [key]: result }
      importMessage.value = `${serverNameById(serverId)} 日志查询已开始`
    }
    startLogQueryPollingIfNeeded()
  } catch (err) {
    handleApiError(err)
  } finally {
    if (queryType === LOG_QUERY_TYPE_XRAY) {
      xrayAnalysisLoading.value = { ...xrayAnalysisLoading.value, [key]: false }
    } else {
      logQueryLoading.value = { ...logQueryLoading.value, [key]: false }
    }
  }
}

async function clearLogQuery(serverId) {
  const queryType = logQueryMode.value
  const key = logQueryStateKey(serverId, queryType)
  const ok = window.confirm(`确定清空 ${serverNameById(serverId)} 的${logQueryModeLabel(queryType)}结果吗？`)
  if (!ok) return
  logQueryRequests.cancel(key)
  logQueryLoading.value = { ...logQueryLoading.value, [key]: true }
  error.value = ''
  importMessage.value = ''
  try {
    if (queryType === LOG_QUERY_TYPE_XRAY) {
      closeXrayPlayerDetail()
      const result = await apiDelete('/api/import/xray-analysis-jobs/latest', { serverId })
      xrayAnalysisResults.value = { ...xrayAnalysisResults.value, [key]: result }
    } else {
      closeLogQueryRowDetail()
      const result = await apiDelete('/api/import/log-query-jobs/latest', { serverId, queryType })
      logQueryPages.value = { ...logQueryPages.value, [key]: 1 }
      logQueryResults.value = { ...logQueryResults.value, [key]: result }
      resetLogQueryDates(serverId, queryType)
    }
    importMessage.value = `${serverNameById(serverId)} ${queryType === LOG_QUERY_TYPE_XRAY ? '分析' : '查询'}结果已清空`
  } catch (err) {
    handleApiError(err)
  } finally {
    logQueryLoading.value = { ...logQueryLoading.value, [key]: false }
  }
}

async function cancelLogQuery(serverId) {
  const queryType = logQueryMode.value
  const key = logQueryStateKey(serverId, queryType)
  const result = selectedLogFeatureResult.value
  if (!result?.jobId || !isActiveJob(result) || logQueryCancelRequests.value[key] || result.status === 'CANCELLING') return
  logQueryRequests.cancel(key)
  logQueryLoading.value = { ...logQueryLoading.value, [key]: true }
  logQueryCancelRequests.value = { ...logQueryCancelRequests.value, [key]: true }
  error.value = ''
  try {
    const endpoint = queryType === LOG_QUERY_TYPE_XRAY ? 'xray-analysis-jobs' : 'log-query-jobs'
    const cancelled = await apiPostJson(`/api/import/${endpoint}/cancel`, { serverId, queryType, jobId: result.jobId })
    if (queryType === LOG_QUERY_TYPE_XRAY) xrayAnalysisResults.value = { ...xrayAnalysisResults.value, [key]: cancelled }
    else {
      logQueryResults.value = { ...logQueryResults.value, [key]: cancelled }
      await loadLogQueryState(serverId, false, queryType)
    }
    importMessage.value = '已请求取消任务'
    startLogQueryPollingIfNeeded()
  } catch (err) {
    handleApiError(err)
  } finally {
    logQueryCancelRequests.value = { ...logQueryCancelRequests.value, [key]: false }
    logQueryLoading.value = { ...logQueryLoading.value, [key]: false }
  }
}

function startLogQueryPollingIfNeeded() {
  if (!Object.values(logQueryResults.value).some(isActiveJob)
      && !Object.values(xrayAnalysisResults.value).some(isActiveJob)) {
    stopLogQueryPolling()
    return
  }
  if (logQueryTimer) {
    return
  }
  logQueryTimer = window.setInterval(async () => {
    const activeKeys = Object.entries(logQueryResults.value)
      .filter(([, result]) => isActiveJob(result))
      .map(([key]) => key)
    if (!activeKeys.length) {
      const activeXrayKeys = Object.entries(xrayAnalysisResults.value)
        .filter(([, result]) => isActiveJob(result))
        .map(([key]) => key)
      if (!activeXrayKeys.length) {
        stopLogQueryPolling()
        return
      }
      await Promise.all(activeXrayKeys.map((key) => {
        const [, serverId] = key.split(':')
        return loadXrayAnalysisState(serverId, true)
      }))
      return
    }
    const activeXrayKeys = Object.entries(xrayAnalysisResults.value)
      .filter(([, result]) => isActiveJob(result))
      .map(([key]) => key)
    await Promise.all(activeKeys.map((key) => {
      const [queryType, serverId] = key.split(':')
      return loadLogQueryState(serverId, true, queryType)
    }).concat(activeXrayKeys.map((key) => {
      const [, serverId] = key.split(':')
      return loadXrayAnalysisState(serverId, true)
    })))
  }, 1800)
}

function stopLogQueryPolling() {
  if (logQueryTimer) {
    window.clearInterval(logQueryTimer)
    logQueryTimer = null
  }
}

function setLogQueryServer(serverId) {
  closeXrayPlayerDetail()
  ensureLogFeatureForms(serverId)
  logQueryServerId.value = serverId
  window.location.hash = logFeatureLocation(serverId, logQueryMode.value)
  loadLogFeatureState(serverId, true)
}

function setLogQueryMode(queryType) {
  if (currentView.value !== 'logQueryPage') return
  queryType = normalizeLogQueryMode(queryType)
  if (logQueryMode.value === queryType) return
  lastLogQueryMode.value = queryType
  logQueryMode.value = queryType
  window.location.hash = logFeatureLocation(logQueryServerId.value, queryType)
  closeXrayPlayerDetail()
  error.value = ''
  importMessage.value = ''
  loadAllLogQueryStates(queryType)
}

function openXrayPlayerDetail(player) {
  if (!player?.playerName) return
  xrayDetailPlayerName.value = player.playerName
  xrayDetailEvidenceExpanded.value = false
  xrayDetailEvidenceRowExpandedKeys.value = new Set()
  xrayDetailOrePositionsExpanded.value = false
  xrayDetailOrePositionPage.value = 1
  closeXrayGroupSendDialog()
  currentView.value = 'xrayDetailPage'
  logQueryMode.value = LOG_QUERY_TYPE_XRAY
  window.location.hash = logFeatureLocation(logQueryServerId.value, LOG_QUERY_TYPE_XRAY, player.playerName)
}

function closeXrayPlayerDetail() {
  xrayDetailPlayerName.value = ''
  xrayDetailEvidenceExpanded.value = false
  xrayDetailEvidenceRowExpandedKeys.value = new Set()
  xrayDetailOrePositionsExpanded.value = false
  xrayDetailOrePositionPage.value = 1
  closeXrayGroupSendDialog()
}

async function openXrayDetailPage(serverId, playerName) {
  logQueryServerId.value = serverId && serverId !== 'all' ? serverId : 'main'
  logQueryMode.value = LOG_QUERY_TYPE_XRAY
  xrayDetailPlayerName.value = playerName || ''
  xrayDetailEvidenceExpanded.value = false
  xrayDetailEvidenceRowExpandedKeys.value = new Set()
  xrayDetailOrePositionsExpanded.value = false
  xrayDetailOrePositionPage.value = 1
  closeXrayGroupSendDialog()
  currentView.value = 'xrayDetailPage'
  window.location.hash = logFeatureLocation(logQueryServerId.value, LOG_QUERY_TYPE_XRAY, playerName)
  error.value = ''
  importMessage.value = ''
  await loadXrayAnalysisState(logQueryServerId.value)
  startLogQueryPollingIfNeeded()
}

function openXrayGroupSendDialog() {
  xrayGroupSendTtlText.value = ''
  xrayGroupSendMessage.value = ''
  xrayGroupSendError.value = ''
  xrayGroupSendDialogOpen.value = true
}

function closeXrayGroupSendDialog() {
  xrayGroupSendDialogOpen.value = false
  xrayGroupSendLoading.value = false
  xrayGroupSendTtlText.value = ''
  xrayGroupSendMessage.value = ''
  xrayGroupSendError.value = ''
}

async function sendXrayDetailToGroup() {
  if (!selectedXrayDetailPlayer.value?.playerName) return
  xrayGroupSendLoading.value = true
  xrayGroupSendMessage.value = ''
  xrayGroupSendError.value = ''
  try {
    const ttlMinutes = parseShareDurationMinutes(xrayGroupSendTtlText.value)
    const result = await apiPostJson('/api/share/xray/send-to-group', {
      serverId: logQueryServerId.value,
      serverName: serverNameById(logQueryServerId.value),
      fromTime: xrayDetailResult.value?.fromTime || '',
      toTime: xrayDetailResult.value?.toTime || '',
      playerName: selectedXrayDetailPlayer.value.playerName,
      ttlMinutes,
      player: selectedXrayDetailPlayer.value
    })
    xrayGroupSendMessage.value = `已提交发送任务，链接 ${durationText(result.ttlMinutes || ttlMinutes)}内有效`
    importMessage.value = '已提交给 AstrBot 插件，请确认插件已启用群发送并填好群号'
  } catch (err) {
    xrayGroupSendError.value = err.message || '发送任务提交失败'
  } finally {
    xrayGroupSendLoading.value = false
  }
}

async function backToXrayAnalysis() {
  await openXrayAnalysisPage(logQueryServerId.value)
}

function logQueryStateKey(serverId, queryType = logQueryMode.value) {
  return `${queryType}:${serverId}`
}

function logQueryResultFor(serverId, queryType = logQueryMode.value) {
  return logQueryResults.value[logQueryStateKey(serverId, queryType)]
}

function xrayAnalysisResultFor(serverId) {
  return xrayAnalysisResults.value[logQueryStateKey(serverId, LOG_QUERY_TYPE_XRAY)]
}

function logQueryModeLabel(queryType = logQueryMode.value) {
  if (queryType === LOG_QUERY_TYPE_PLAYER_KEYWORD) return '综合筛选'
  if (queryType === LOG_QUERY_TYPE_XRAY) return '矿透分析'
  return '通过坐标查询'
}

function setLogQueryPage(page) {
  const totalPages = selectedLogQueryTotalPages.value || 1
  const nextPage = Math.max(1, Math.min(Number(page) || 1, totalPages))
  if (nextPage !== selectedLogQueryPage.value) loadLogQueryState(logQueryServerId.value, false, logQueryMode.value, { page: nextPage })
}
function setLogQueryPageSize(pageSize) {
  const size = PAGE_SIZE_OPTIONS.includes(Number(pageSize)) ? Number(pageSize) : DEFAULT_PAGE_SIZE
  loadLogQueryState(logQueryServerId.value, false, logQueryMode.value, { page: 1, pageSize: size })
}
function setLogQuerySortOrder(sortOrder) {
  loadLogQueryState(logQueryServerId.value, false, logQueryMode.value, { page: 1, sortOrder })
}
function setXrayDetailSortOrder(sortOrder) {
  xrayDetailSortOrder.value = sortOrder
  xrayDetailOrePositionPage.value = 1
}

function openLogQueryRowDetail(row) {
  selectedLogQueryRow.value = row
}

function closeLogQueryRowDetail() {
  selectedLogQueryRow.value = null
}

function logQueryRowFieldText(value) {
  const text = String(value ?? '').trim()
  return text || '-'
}

function logQueryDetailAuditText(row, field) {
  const display = String(row?.[`${field}Display`] ?? '').trim()
  const raw = String(row?.[field] ?? '').trim()
  if (display && raw && display !== raw) return `${display}（${raw}）`
  return display || raw || '-'
}

function logQueryRowDetailFields(row = selectedLogQueryRow.value) {
  if (!row) return []
  return [
    ['文件', row.fileName || fileNameFromPath(row.filePath)],
    ['行号', row.lineNumber],
    ['时间', [row.date, row.time].filter(Boolean).join(' ')],
    ['玩家', row.playerName],
    ['行为', logQueryActionDisplay(row.action)],
    ['玩家坐标', logQueryCoordinateText(row.x, row.y, row.z, row.dimension)],
    ['交互坐标', logQueryInteractionCoordinateText(row)],
    ['文件路径', row.filePath]
  ].map(([label, value]) => ({ label, value: logQueryRowFieldText(value) }))
}

function logQueryRowDetailText(row = selectedLogQueryRow.value) {
  if (!row) return ''
  const lines = logQueryRowDetailFields(row).map((field) => `${field.label}：${field.value}`)
  lines.push(`详细1：${logQueryDetailAuditText(row, 'detail1')}`)
  lines.push(`详细2：${logQueryDetailAuditText(row, 'detail2')}`)
  return lines.join('\n')
}

async function copyLogQueryRowDetail() {
  const text = logQueryRowDetailText()
  if (!text) return
  try {
    await navigator.clipboard.writeText(text)
    importMessage.value = '日志详情已复制'
  } catch (err) {
    error.value = '复制失败，请手动选中文本复制'
  }
}

function scrollLogQueryToTop() {
  window.scrollTo({ top: 0, behavior: 'smooth' })
}

function scrollToElementById(id) {
  document.getElementById(id)?.scrollIntoView({ behavior: 'smooth', block: 'start' })
}

function scrollToXrayShareOrePositions() {
  scrollToElementById('xray-share-rare-ore-positions')
}

function scrollToXrayDetailOrePositions() {
  scrollToElementById('xray-detail-rare-ore-positions')
}

function xrayAnalysisRequestBody(serverId) {
  const form = xrayAnalysisForms.value[serverId] || defaultXrayAnalysisForm()
  const fromDate = form.fromDate || (form.fromTime || '').slice(0, 10)
  const toDate = form.toDate || (form.toTime || '').slice(0, 10)
  saveXrayAnalysisForms()
  return {
    serverId,
    fromTime: fromDate ? `${fromDate}T00:00:00` : '',
    toTime: toDate ? `${toDate}T23:59:59` : '',
    playerName: (form.playerName || '').trim(),
    dimension: (form.dimension || '').trim()
  }
}

function logQueryRequestBody(serverId, queryType) {
  if (queryType === LOG_QUERY_TYPE_PLAYER_KEYWORD) {
    const form = logTextQueryForms.value[serverId] || defaultLogTextQueryForm()
    const playerName = (form.playerName || '').trim()
    const keyword = (form.keyword || '').trim()
    const actions = Array.isArray(form.actions) ? form.actions : []
    saveLogTextQueryForms()
    return {
      serverId,
      queryType,
      fromDate: form.fromDate,
      toDate: form.toDate,
      playerName,
      keyword,
      actions
    }
  }

  const form = logQueryForms.value[serverId] || defaultLogQueryForm()
  saveLogQueryForms()
  return {
    serverId,
    queryType,
    fromDate: form.fromDate,
    toDate: form.toDate,
    ...coordinateQueryParameters(form),
    dimension: form.dimension
  }
}

function ensureLogFeatureForms(serverId) {
  // Server options come from the API after setup; initialize lazily without
  // replacing a form the administrator has already edited in this session.
  for (const [forms, readSaved, defaults] of [
    [logQueryForms, readSavedLogQueryForms, defaultLogQueryForm],
    [logTextQueryForms, readSavedLogTextQueryForms, defaultLogTextQueryForm],
    [xrayAnalysisForms, readSavedXrayAnalysisForms, defaultXrayAnalysisForm]
  ]) {
    if (!forms.value[serverId]) forms.value[serverId] = readSaved()[serverId] || defaults()
  }
}

function readSavedLogQueryForms() {
  const defaults = Object.fromEntries(serverOptions.value.map((server) => [server.serverId, defaultLogQueryForm()]))
  try {
    const saved = JSON.parse(localStorage.getItem(LOG_QUERY_FORM_STORAGE_KEY) || '{}')
    for (const server of serverOptions.value) {
      defaults[server.serverId] = {
        ...defaults[server.serverId],
        ...(saved[server.serverId] || {}),
        position1: savedCoordinateInput(saved[server.serverId], 1),
        position2: savedCoordinateInput(saved[server.serverId], 2),
        fromDate: saved[server.serverId]?.fromDate || saved[server.serverId]?.date || defaults[server.serverId].fromDate,
        toDate: saved[server.serverId]?.toDate || saved[server.serverId]?.date || defaults[server.serverId].toDate
      }
    }
  } catch (err) {
    // 表单缓存坏了就回到默认值。
  }
  return defaults
}

function defaultLogQueryForm() {
  const today = todayDateInput()
  return {
    fromDate: today,
    toDate: today,
    rangeMode: 'box',
    position1: '',
    position2: '',
    centerPosition: '',
    radius: 5,
    dimension: ''
  }
}

function readSavedLogTextQueryForms() {
  const defaults = Object.fromEntries(serverOptions.value.map((server) => [server.serverId, defaultLogTextQueryForm()]))
  try {
    const saved = JSON.parse(localStorage.getItem(LOG_TEXT_QUERY_FORM_STORAGE_KEY) || '{}')
    for (const server of serverOptions.value) {
      defaults[server.serverId] = {
        ...defaults[server.serverId],
        ...(saved[server.serverId] || {}),
        fromDate: saved[server.serverId]?.fromDate || defaults[server.serverId].fromDate,
        toDate: saved[server.serverId]?.toDate || defaults[server.serverId].toDate,
        actions: Array.isArray(saved[server.serverId]?.actions) ? saved[server.serverId].actions : (logActionOptions.some(option => option.value && option.value === saved[server.serverId]?.action) ? [saved[server.serverId].action] : [])
      }
    }
  } catch (err) {
    // 表单缓存坏了就回到默认值。
  }
  return defaults
}

function resetLogQueryDates(serverId, queryType) {
  ensureLogFeatureForms(serverId)
  const forms = queryType === LOG_QUERY_TYPE_PLAYER_KEYWORD ? logTextQueryForms : logQueryForms
  const today = todayDateInput()
  forms.value[serverId] = { ...forms.value[serverId], fromDate: today, toDate: today }
  if (queryType === LOG_QUERY_TYPE_PLAYER_KEYWORD) saveLogTextQueryForms()
  else saveLogQueryForms()
}

function defaultLogTextQueryForm() {
  const today = todayDateInput()
  return {
    fromDate: today,
    toDate: today,
    playerName: '',
    keyword: '',
    actions: []
  }
}

function readSavedXrayAnalysisForms() {
  const defaults = Object.fromEntries(serverOptions.value.map((server) => [server.serverId, defaultXrayAnalysisForm()]))
  try {
    const saved = JSON.parse(localStorage.getItem(LOG_XRAY_ANALYSIS_FORM_STORAGE_KEY) || '{}')
    for (const server of serverOptions.value) {
      const savedForm = saved[server.serverId] || {}
      defaults[server.serverId] = {
        ...defaults[server.serverId],
        ...savedForm,
        fromDate: savedForm.fromDate || (savedForm.fromTime || '').slice(0, 10) || defaults[server.serverId].fromDate,
        toDate: savedForm.toDate || (savedForm.toTime || '').slice(0, 10) || defaults[server.serverId].toDate
      }
    }
  } catch (err) {
    // 本地缓存坏了就回到默认值。
  }
  return defaults
}

function defaultXrayAnalysisForm() {
  const today = todayDateInput()
  return {
    fromDate: today,
    toDate: today,
    playerName: '',
    dimension: ''
  }
}

function saveLogQueryForms() {
  try {
    localStorage.setItem(LOG_QUERY_FORM_STORAGE_KEY, JSON.stringify(logQueryForms.value))
  } catch (err) {
    // 本地缓存失败不影响查询。
  }
}

function saveLogTextQueryForms() {
  try {
    localStorage.setItem(LOG_TEXT_QUERY_FORM_STORAGE_KEY, JSON.stringify(logTextQueryForms.value))
  } catch (err) {
    // 本地缓存失败不影响查询。
  }
}

function saveXrayAnalysisForms() {
  try {
    localStorage.setItem(LOG_XRAY_ANALYSIS_FORM_STORAGE_KEY, JSON.stringify(xrayAnalysisForms.value))
  } catch (err) {
    // 本地缓存失败不影响查询。
  }
}

function todayDateInput() {
  const date = new Date()
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}

const logQueryRadiusError = computed(() => {
  const form = logQueryForms.value[logQueryServerId.value]
  if (form?.rangeMode !== 'radius') return ''
  const raw = String(form.radius ?? '').trim()
  if (!raw) return ''
  const radius = Number(raw)
  if (!Number.isFinite(radius)) return '请输入有效的半径'
  if (radius > 200) return '半径不能超过 200 格'
  if (radius < 0) return '半径不能小于 0 格'
  return ''
})

function validateLogCoordinateInput(event) {
  const input = event.target
  input.setCustomValidity(!input.value || parseCoordinateInput(input.value) ? '' : '请输入三个坐标，例如：34 35 523（支持空格或逗号分隔）')
}

function durationText(minutes) {
  const safeMinutes = Math.max(1, Math.round(Number(minutes) || 0))
  if (safeMinutes % 1440 === 0) return `${safeMinutes / 1440}天`
  if (safeMinutes % 60 === 0) return `${safeMinutes / 60}小时`
  if (safeMinutes > 60) {
    const hours = Math.floor(safeMinutes / 60)
    const restMinutes = safeMinutes % 60
    return `${hours}小时${restMinutes}分钟`
  }
  return `${safeMinutes}分钟`
}

function parseShareDurationMinutes(value) {
  const text = String(value || '').trim()
  if (!text) return 1440
  const match = text.match(/^(\d+(?:\.\d+)?)\s*(分钟|分|m|min|小时|时|h|天|日|d)?$/i)
  if (!match) {
    throw new Error('有效时间格式不对，可填 1天、6小时、30分钟；留空默认 1天')
  }
  const amount = Number(match[1])
  if (!Number.isFinite(amount) || amount <= 0) {
    throw new Error('有效时间必须大于 0')
  }
  const unit = (match[2] || '天').toLowerCase()
  let minutes = amount * 1440
  if (['分钟', '分', 'm', 'min'].includes(unit)) {
    minutes = amount
  } else if (['小时', '时', 'h'].includes(unit)) {
    minutes = amount * 60
  }
  return Math.max(5, Math.min(Math.round(minutes), 10080))
}

function serverNameById(serverId) {
  return serverOptions.value.find((server) => server.serverId === serverId)?.serverName || serverId
}

function resetPasswordForm() {
  passwordForm.value = {
    currentPassword: '',
    newPassword: '',
    confirmPassword: ''
  }
  showCurrentPassword.value = false
  showNewPassword.value = false
  showConfirmPassword.value = false
}

async function changePassword() {
  error.value = ''
  importMessage.value = ''

  if (passwordForm.value.newPassword !== passwordForm.value.confirmPassword) {
    error.value = '两次输入的新密码不一致'
    return
  }
  if (passwordForm.value.newPassword.length < 8) {
    error.value = '新密码至少 8 位'
    return
  }

  passwordSaving.value = true
  try {
    await apiPostJson('/api/auth/password', {
      currentPassword: passwordForm.value.currentPassword,
      newPassword: passwordForm.value.newPassword
    })
    importMessage.value = '密码已修改'
    resetPasswordForm()
  } catch (err) {
    handleApiError(err)
  } finally {
    passwordSaving.value = false
  }
}

async function goDashboard() {
  stopImportJobPolling()
  stopSyncJobPolling()
  stopLogQueryPolling()
  importing.value = false
  syncingFiles.value = false
  currentView.value = 'dashboard'
  window.location.hash = ''
  await loadAll()
}

function handleApiError(err) {
  error.value = err.message
  if (!getAuthToken()) {
    authenticated.value = false
    currentAdmin.value = ''
  }
}

function selectServer(serverId) {
  filters.value.serverId = serverId
  selectedResetKeys.value = new Set()
  importJob.value = null
  syncJob.value = null
  loadAll()
}

// The shell emits intent; page loaders retain ownership of data and task state.
async function navigateAdmin(id) {
  if (currentView.value === 'settingsPage' && id !== 'settings' && settingsPage.value && !settingsPage.value.canLeave()) return
  const serverId = resolveTaskServer(filters.value.serverId, filters.value.serverId, serverOptions.value)
  const queryServerId = ['logQueryPage', 'xrayAnalysisPage', 'xrayDetailPage'].includes(currentView.value) ? logQueryServerId.value : serverId
  stopImportJobPolling()
  stopSyncJobPolling()
  stopLogQueryPolling()
  window.scrollTo(0, 0)
  if (id === 'dashboard') await goDashboard()
  else if (id === 'logs') await openLogQueryPage(queryServerId)
  else if (id === 'xray') await openXrayAnalysisPage(queryServerId)
  else if (id === 'sync') await openSyncPage(serverId)
  else if (id === 'txtCopy') { currentView.value = 'txtCopyPage'; window.location.hash = '#/txt-copy' }
  else if (id === 'groupChat') { currentView.value = 'groupChatPage'; window.location.hash = '#/group-chat' }
  else if (id === 'import') await openImportPage(serverId)
  else if (id === 'settings') await openSettingsPage()
  else if (id === 'profile') openProfilePage()
}

async function handleAdminHashChange() {
  if (!authChecked.value || !authenticated.value) return
  const hash = window.location.hash || ''
  if (hash === '#admin-main') return
  if (currentView.value === 'settingsPage' && hash !== '#/settings' && settingsPage.value && !settingsPage.value.canLeave()) {
    window.history.replaceState(null, '', '#/settings')
    return
  }
  const route = parseLogFeatureLocation(hash)
  const taskRoute = parseTaskLocation(hash)
  if (taskRoute) {
    const serverId = resolveTaskServer(taskRoute.serverId, filters.value.serverId, serverOptions.value)
    if (currentView.value === taskRoute.view && filters.value.serverId === serverId) return
  } else if (route) {
    const serverId = route.serverId || (filters.value.serverId === 'all' ? 'main' : filters.value.serverId)
    if (currentView.value === route.view && logQueryServerId.value === serverId
        && logQueryMode.value === route.mode
        && (route.view !== 'xrayDetailPage' || xrayDetailPlayerName.value === route.playerName)) return
  } else {
    const view = { '': 'dashboard', '#': 'dashboard', '#/sync': 'syncPage', '#/txt-copy': 'txtCopyPage', '#/group-chat': 'groupChatPage', '#/import': 'importPage', '#/settings': 'settingsPage', '#/profile': 'profilePage' }[hash]
    if (!view || currentView.value === view) return
    if (view === 'dashboard') { await goDashboard(); return }
  }
  stopImportJobPolling()
  stopSyncJobPolling()
  stopLogQueryPolling()
  await openInitialViewFromHash()
}

async function openInitialViewFromHash() {
  const hash = window.location.hash || ''
  const featureRoute = parseLogFeatureLocation(hash)
  const taskRoute = parseTaskLocation(hash)
  if (featureRoute) {
    const serverId = featureRoute.serverId || filters.value.serverId
    if (featureRoute.view === 'xrayDetailPage') await openXrayDetailPage(serverId, featureRoute.playerName)
    else if (featureRoute.view === 'xrayAnalysisPage') await openXrayAnalysisPage(serverId)
    else await openLogQueryPage(serverId, featureRoute.mode)
  } else if (hash === '#/txt-copy') {
    currentView.value = 'txtCopyPage'
  } else if (hash === '#/group-chat') {
    currentView.value = 'groupChatPage'
  } else if (taskRoute) {
    if (taskRoute.view === 'syncPage') await openSyncPage(taskRoute.serverId)
    else await openImportPage(taskRoute.serverId)
  } else if (hash === '#/profile') {
    openProfilePage()
  } else if (hash === '#/settings') {
    await openSettingsPage()
  }
}

function currentServerScope() {
  return filters.value.serverId === 'all' ? 'all' : filters.value.serverId
}

function jobStorageKey(type) {
  return type === 'sync' ? SYNC_JOB_STORAGE_KEY : IMPORT_JOB_STORAGE_KEY
}

function readSavedJobs(type) {
  try {
    return JSON.parse(localStorage.getItem(jobStorageKey(type)) || '{}')
  } catch (err) {
    return {}
  }
}

function savedJobId(type) {
  return readSavedJobs(type)[currentServerScope()] || ''
}

function saveJobId(type, jobId) {
  try {
    const jobs = readSavedJobs(type)
    jobs[currentServerScope()] = jobId
    localStorage.setItem(jobStorageKey(type), JSON.stringify(jobs))
  } catch (err) {
    // 保存状态失败不影响任务本身继续运行。
  }
}

function clearSavedJobId(type) {
  try {
    const jobs = readSavedJobs(type)
    delete jobs[currentServerScope()]
    localStorage.setItem(jobStorageKey(type), JSON.stringify(jobs))
  } catch (err) {
    // 忽略本地缓存清理失败。
  }
}

function isActiveJob(job) {
  return ACTIVE_JOB_STATUSES.includes(job?.status)
}

function logQueryStatusLabel(status) {
  if (logQueryMode.value === LOG_QUERY_TYPE_XRAY) {
    const labels = { IDLE: '未分析', PENDING: '等待分析', RUNNING: '分析中', FINISHED: '分析完成', FAILED: '分析失败' }
    if (labels[status]) return labels[status]
  }
  const labels = {
    IDLE: '未查询',
    PENDING: '等待查询',
    RUNNING: '查询中',
    CANCELLING: '取消中',
    CANCELLED: '已取消',
    FINISHED: '查询完成',
    FINISHED_WITH_ERRORS: '部分文件失败',
    FAILED: '查询失败'
  }
  return labels[status] || status || '未查询'
}

function logQueryCoordinateText(x, y, z, dimension) {
  if (!x || x === '-') return '-'
  const dim = dimension && dimension !== '-' ? ` · ${dimension}` : ''
  return `${x} ${y} ${z}${dim}`
}

function formatCoordValue(value) {
  const number = Number(value)
  if (!Number.isFinite(number)) return '-'
  return Number.isInteger(number) ? String(number) : number.toLocaleString(undefined, { maximumFractionDigits: 3 })
}

function logQueryInteractionCoordinateText(row) {
  const explicit = [row?.x2, row?.y2, row?.z2]
    .some((value) => {
      const text = String(value ?? '').trim()
      return text && text !== '-'
    })
  if (!explicit && isBucketLogAction(row?.action)) {
    return logQueryCoordinateText(row?.x, row?.y, row?.z, row?.dimension)
  }
  return logQueryCoordinateText(row?.x2, row?.y2, row?.z2, row?.dimension2)
}

function evidenceRowText(row) {
  const when = [row?.date, row?.time].filter(Boolean).join(' ')
  const playerPosition = logQueryCoordinateText(row?.x, row?.y, row?.z, row?.dimension)
  const targetPosition = logQueryInteractionCoordinateText(row)
  return [
    when,
    row?.action && row.action !== '-' ? logQueryActionDisplay(row.action) : '',
    logQuerySummaryDetailText(row),
    playerPosition !== '-' ? `玩家 ${playerPosition}` : '',
    targetPosition !== '-' ? `交互 ${targetPosition}` : ''
  ].filter(Boolean).join(' · ') || '-'
}

function xrayOreCounts(player) {
  return player?.ores?.length ? player.ores : (player?.rareOres || [])
}

function xrayAnalysisOreCounts(player) {
  return player?.analysisOres?.length ? player.analysisOres : xrayOreCounts(player)
}

function xrayAnalysisValue(player, analysisKey, fallbackKey = null) {
  const value = player?.[analysisKey]
  if (value !== undefined && value !== null) return value
  return fallbackKey ? (player?.[fallbackKey] || 0) : 0
}

function xrayAnalysisTrackingEvidenceCount(player) {
  return player?.analysisTrackingEvidenceCount ?? player?.evidence?.length ?? player?.trackingEvidenceCount ?? 0
}

function xrayPageCount(total, pageSize) {
  return Math.max(1, Math.ceil((Number(total) || 0) / (Number(pageSize) || 100)))
}

function clampPage(page, totalPages) {
  return Math.max(1, Math.min(Number(page) || 1, Number(totalPages) || 1))
}

function pagedXrayRows(rows, page, pageSize) {
  const totalPages = xrayPageCount(rows.length, pageSize)
  const safePage = clampPage(page, totalPages)
  const start = (safePage - 1) * pageSize
  return rows.slice(start, start + pageSize)
}

async function setLogSharePage(page) {
  const nextPage = clampPage(page, logShareTotalPages.value)
  if (nextPage === logSharePage.value || logSharePageLoading.value) return
  logSharePage.value = nextPage
  await loadLogShareDetails()
}

async function setLogSharePageSize(pageSize) {
  if (logSharePageLoading.value) return
  const parsed = Number.parseInt(pageSize, 10)
  logSharePageSize.value = LOG_SHARE_PAGE_SIZE_OPTIONS.includes(parsed) ? parsed : 50
  logSharePage.value = 1
  await loadLogShareDetails()
}

function setXrayShareOrePositionPage(page) {
  xrayShareOrePositionPage.value = clampPage(page, xrayShareOrePositionTotalPages.value)
}

function setXrayDetailOrePositionPage(page) {
  xrayDetailOrePositionPage.value = clampPage(page, xrayDetailOrePositionTotalPages.value)
}

function setXrayShareOrePositionPageSize(pageSize) {
  const safePageSize = XRAY_ORE_POSITION_PAGE_SIZE_OPTIONS.includes(Number(pageSize)) ? Number(pageSize) : 100
  xrayShareOrePositionPageSize.value = safePageSize
  xrayShareOrePositionPage.value = 1
}

function setXrayDetailOrePositionPageSize(pageSize) {
  const safePageSize = XRAY_ORE_POSITION_PAGE_SIZE_OPTIONS.includes(Number(pageSize)) ? Number(pageSize) : DEFAULT_PAGE_SIZE
  xrayDetailOrePositionPageSize.value = safePageSize
  xrayDetailOrePositionPage.value = 1
}

function toggleXrayShareOrePositions() {
  xrayShareOrePositionsExpanded.value = !xrayShareOrePositionsExpanded.value
  xrayShareOrePositionPage.value = 1
}

function toggleXrayDetailOrePositions() {
  xrayDetailOrePositionsExpanded.value = !xrayDetailOrePositionsExpanded.value
  xrayDetailOrePositionPage.value = 1
}

function xrayEvidenceRowKey(player, evidence, index) {
  return `${player?.playerName || ''}:${index}:${evidence?.startedAt || ''}:${evidence?.endedAt || ''}:${evidence?.summary || ''}`
}

function isShareXrayEvidenceRowsExpanded(evidence, index) {
  return xrayShareEvidenceRowExpandedKeys.value.has(xrayEvidenceRowKey(xraySharePlayer.value, evidence, index))
}

function toggleShareXrayEvidenceRows(evidence, index) {
  const key = xrayEvidenceRowKey(xraySharePlayer.value, evidence, index)
  const next = new Set(xrayShareEvidenceRowExpandedKeys.value)
  if (next.has(key)) {
    next.delete(key)
  } else {
    next.add(key)
  }
  xrayShareEvidenceRowExpandedKeys.value = next
}

function isDetailXrayEvidenceRowsExpanded(evidence, index) {
  return xrayDetailEvidenceRowExpandedKeys.value.has(xrayEvidenceRowKey(selectedXrayDetailPlayer.value, evidence, index))
}

function toggleDetailXrayEvidenceRows(evidence, index) {
  const key = xrayEvidenceRowKey(selectedXrayDetailPlayer.value, evidence, index)
  const next = new Set(xrayDetailEvidenceRowExpandedKeys.value)
  if (next.has(key)) {
    next.delete(key)
  } else {
    next.add(key)
  }
  xrayDetailEvidenceRowExpandedKeys.value = next
}

function formatDateOnly(value) {
  if (!value) return '-'
  return new Intl.DateTimeFormat('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit'
  }).format(new Date(value))
}

function formatDotDate(value) {
  return formatDateOnly(value).replace(/\//g, '.')
}

function xrayAnalysisDateRange(result) {
  if (!result?.fromTime || !result?.toTime) return '-'
  return `${formatDateOnly(result.fromTime)} - ${formatDateOnly(result.toTime)}`
}

function xrayRiskClass(score) {
  const value = Number(score) || 0
  if (value >= 85) return 'extreme'
  if (value >= 70) return 'high'
  if (value >= 45) return 'medium'
  if (value >= 20) return 'low'
  return 'watch'
}

function xrayEvidenceTime(evidence) {
  if (!evidence?.startedAt || !evidence?.endedAt) return '-'
  return `${formatDateTime(evidence.startedAt)} - ${formatDateTime(evidence.endedAt)}`
}

function xrayMiningSessionTime(player) {
  if (!player?.miningSessionStart || !player?.miningSessionEnd) return '-'
  return `${formatDateTime(player.miningSessionStart)} - ${formatDateTime(player.miningSessionEnd)}`
}

function xrayMiningSessionDuration(player) {
  if (!player?.miningSessionStart || !player?.miningSessionEnd) return '-'
  const minutes = Math.max(1, Math.round((new Date(player.miningSessionEnd).getTime() - new Date(player.miningSessionStart).getTime()) / 60000))
  return durationText(minutes)
}

function xrayRareRatioText(player) {
  const ratio = Number(player?.undergroundRareOreRatio)
  if (!Number.isFinite(ratio) || ratio <= 0) return '-'
  return `${(ratio * 100).toFixed(ratio >= 0.1 ? 1 : 2)}%`
}

function xrayAnalysisRareRatioText(player) {
  const ratio = Number(player?.analysisUndergroundRareOreRatio ?? player?.undergroundRareOreRatio)
  if (!Number.isFinite(ratio) || ratio <= 0) return '-'
  return `${(ratio * 100).toFixed(ratio >= 0.1 ? 1 : 2)}%`
}

function jobSummaryMessage(type, job) {
  if (job?.status === 'FAILED' && job.message) {
    return `${type === 'sync' ? '复制' : '解析'}任务失败：${job.message}`
  }
  const actionLabel = type === 'sync' ? '复制' : '导入'
  return `扫描 ${job?.scannedFiles || 0} 个文件，${actionLabel} ${job?.importedFiles || 0} 个，跳过 ${job?.skippedFiles || 0} 个，失败 ${job?.failedFiles || 0} 个`
}

async function syncServerTime() {
  try {
    const data = await apiGet('/api/stats/server-time')
    const serverMs = new Date(data.serverTime).getTime()
    serverTimeOffset = serverMs - Date.now()
    startServerTimeTicker()
  } catch (err) {
    serverTimeOffset = 0
    startServerTimeTicker()
  }
}

function startServerTimeTicker() {
  stopServerTimeTicker()
  tickServerTime()
  serverTimeTimer = window.setInterval(tickServerTime, 1000)
}

function stopServerTimeTicker() {
  if (serverTimeTimer) {
    window.clearInterval(serverTimeTimer)
    serverTimeTimer = null
  }
}

function tickServerTime() {
  const now = new Date(Date.now() + serverTimeOffset)
  serverTimeDisplay.value = now.getFullYear()
    + '-' + String(now.getMonth() + 1).padStart(2, '0')
    + '-' + String(now.getDate()).padStart(2, '0')
    + ' ' + String(now.getHours()).padStart(2, '0')
    + ':' + String(now.getMinutes()).padStart(2, '0')
    + ':' + String(now.getSeconds()).padStart(2, '0')
}

function emptyOverview() {
  return {
    playerCount: 0,
    brokenCount: 0,
    placedCount: 0,
    totalCount: 0,
    importedFileCount: 0,
    lastImportedAt: null
  }
}

function asArray(value) {
  return Array.isArray(value) ? value : []
}

function formatNumber(value) {
  return new Intl.NumberFormat('zh-CN').format(value || 0)
}

function shareBreakPercent(server) {
  const total = Number(server?.totalCount || 0)
  const broken = Number(server?.brokenCount || 0)
  if (!total) return 0
  return Math.round(Math.min(100, Math.max(0, (broken / total) * 100)))
}

function sharePlacePercent(server) {
  return Number(server?.totalCount || 0) > 0 ? 100 - shareBreakPercent(server) : 0
}

function shareCountTotal(items) {
  return asArray(items).reduce((total, item) => total + Number(item?.count || 0), 0)
}

function cleanShareText(value, fallback = '') {
  const text = String(value || fallback).trim()
  return text.replace(/[。！？!?]+$/g, '')
}

function formatDateTime(value) {
  if (!value) return '-'
  return new Intl.DateTimeFormat('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit'
  }).format(new Date(value))
}

function formatShareMoment(value) {
  if (!value) return '-'
  return new Intl.DateTimeFormat('zh-CN', {
    month: 'numeric',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit'
  }).format(new Date(value)).replace('/', '月').replace(' ', '日 ')
}

function fileSize(value) {
  if (!value) return '0 B'
  if (value < 1024) return `${value} B`
  if (value < 1024 * 1024) return `${(value / 1024).toFixed(1)} KB`
  return `${(value / 1024 / 1024).toFixed(1)} MB`
}

function sourceStat(serverId, field) {
  return serverSummaries.value.find((item) => item.serverId === serverId)?.[field] || 0
}

function fileNameFromPath(path) {
  if (!path) return '-'
  const parts = path.split(/[\\/]/)
  return parts[parts.length - 1] || path
}

function statusLabel(status, context = 'import') {
  const labels = {
    READY: '就绪',
    PENDING: context === 'sync' ? '待复制' : '待解析',
    RUNNING: context === 'sync' ? '复制中' : '解析中',
    FINISHED: '已完成',
    FINISHED_WITH_ERRORS: '完成有失败',
    IMPORTED: '成功',
    COPIED: '已复制',
    SKIPPED: '跳过',
    SKIPPED_TODAY: '今日跳过',
    NEEDS_IMPORT: '需解析',
    CHANGED: '已变化',
    FAILED: '失败',
    REMOTE: '远程',
    INFO: '信息'
  }
  return labels[status] || status
}

function statusClass(status) {
  return {
    imported: status === 'IMPORTED' || status === 'COPIED' || status === 'FINISHED',
    running: status === 'RUNNING',
    pending: status === 'PENDING' || status === 'NEEDS_IMPORT' || status === 'CHANGED' || status === 'REMOTE',
    skipped: status === 'SKIPPED' || status === 'SKIPPED_TODAY' || status === 'INFO',
    failed: status === 'FAILED' || status === 'FINISHED_WITH_ERRORS'
  }
}

function canDeleteImportRecord(file) {
  return Boolean(file?.imported || ['IMPORTED', 'CHANGED', 'NEEDS_IMPORT'].includes(file?.status))
}

function importFileKey(file) {
  return `${file.serverId}\n${file.remotePath}`
}

function toggleImportFileSelection(file, checked) {
  const next = new Set(selectedResetKeys.value)
  const key = importFileKey(file)
  if (checked) {
    next.add(key)
  } else {
    next.delete(key)
  }
  selectedResetKeys.value = next
}

function toggleAllImportFileSelection(checked) {
  selectedResetKeys.value = checked
    ? new Set(eligibleImportFiles.value.map(importFileKey))
    : new Set()
}
</script>

<template>
  <main v-if="isSharePage" class="player-share-v2">
    <section v-if="shareLoading" class="share-v2-state share-v2-state-loading">
      <div class="share-v2-state-mark"><Blocks :size="24" /></div>
      <h1>正在打开足迹</h1>
      <p>正在读取玩家的完整活动报告...</p>
    </section>

    <section v-else-if="shareError" class="share-v2-state share-v2-state-error">
      <div class="share-v2-state-mark"><Clock :size="24" /></div>
      <h1>链接已过期</h1>
      <p>{{ shareError }}</p>
    </section>

    <template v-else-if="shareData && selectedShareServer">
      <header class="share-v2-hero">
        <div class="share-v2-hero-content">
          <p class="share-v2-player-label">玩家活动档案</p>
          <h1>{{ shareData.playerName }}</h1>
          <p class="share-v2-hero-description">生存活动</p>
          <div class="share-v2-hero-meta">
            <span><Layers :size="15" />{{ selectedShareServer.serverName }}</span>
            <span><CalendarDays :size="15" />数据截止 {{ selectedShareServer.latestLogDate || shareData.latestLogDate || '-' }}</span>
            <span><Clock :size="15" />{{ shareRemainingText }}</span>
          </div>
        </div>
        <div class="share-v2-hero-score">
          <div class="share-v2-score-head">
            <span class="share-v2-score-label">总活动</span>
            <span class="share-v2-score-server">{{ selectedShareServer.serverName }}</span>
          </div>
          <strong>{{ formatNumber(selectedShareServer.totalCount) }}</strong>
          <span class="share-v2-score-unit">次方块行为</span>
          <div
            class="share-v2-score-bar"
            :class="{ 'is-empty': !selectedShareServer.totalCount }"
            :style="{ '--break-percent': `${shareBreakPercent(selectedShareServer)}%` }"
            aria-label="破坏与放置行为占比"
          >
            <span class="is-broken"></span>
            <span class="is-placed"></span>
          </div>
          <div class="share-v2-score-split">
            <span>
              <i class="is-broken"></i>
              破坏
              <b>{{ shareBreakPercent(selectedShareServer) }}%</b>
              <small>{{ formatNumber(selectedShareServer.brokenCount) }}</small>
            </span>
            <span>
              <i class="is-placed"></i>
              放置
              <b>{{ sharePlacePercent(selectedShareServer) }}%</b>
              <small>{{ formatNumber(selectedShareServer.placedCount) }}</small>
            </span>
          </div>
        </div>
      </header>

      <section class="share-v2-server-bar" aria-label="服务器数据">
        <a
          class="share-v2-server-context"
          :href="shareMinecraftJoinUrl"
          @click="minecraftLaunchNoticeOpen = true"
        >
          <span class="share-v2-context-icon"><Database :size="18" /></span>
          <div>
            <p>MINECRAFT</p>
            <strong>{{ selectedShareServer.serverName }}</strong>
            <small>点击启动游戏</small>
          </div>
        </a>
        <nav class="share-v2-server-tabs" aria-label="服务器">
          <button
            v-for="server in shareServers"
            :key="server.serverId"
            type="button"
            :class="{ active: selectedShareServerId === server.serverId }"
            @click="selectedShareServerId = server.serverId"
          >
            <Layers :size="15" />
            <span>{{ server.serverName }}</span>
          </button>
        </nav>
      </section>

      <section class="share-v2-stat-grid" aria-label="核心统计">
        <article class="share-v2-stat-card is-blue">
          <div class="share-v2-stat-icon"><Blocks :size="20" /></div>
          <div>
            <span>总活动</span>
            <strong>{{ formatNumber(selectedShareServer.totalCount) }}</strong>
            <small>全部方块行为</small>
          </div>
        </article>
        <article class="share-v2-stat-card is-orange">
          <div class="share-v2-stat-icon"><Hammer :size="20" /></div>
          <div>
            <span>破坏数量</span>
            <strong>{{ formatNumber(selectedShareServer.brokenCount) }}</strong>
            <small>方块破坏记录</small>
          </div>
        </article>
        <article class="share-v2-stat-card is-green">
          <div class="share-v2-stat-icon"><Layers :size="20" /></div>
          <div>
            <span>放置数量</span>
            <strong>{{ formatNumber(selectedShareServer.placedCount) }}</strong>
            <small>建造与放置</small>
          </div>
        </article>
        <article class="share-v2-stat-card is-purple">
          <div class="share-v2-stat-icon"><CalendarDays :size="20" /></div>
          <div>
            <span>首次记录</span>
            <strong>{{ formatDotDate(selectedShareServer.firstSeenAt) }}</strong>
            <small>最早发现玩家足迹</small>
          </div>
        </article>
      </section>

      <section class="share-v2-layout">
        <div class="share-v2-main-column">
          <section class="share-v2-panel share-v2-milestone-panel">
            <div class="share-v2-panel-heading">
              <div>
                <p class="share-v2-eyebrow">SURVIVAL TIMELINE</p>
                <h2>生存里程碑</h2>
                <p>从第一块木头到第一次找到远古残骸，记录每一个关键节点</p>
              </div>
            </div>
            <div class="share-v2-milestone-grid">
              <article
                v-for="item in (selectedShareServer.milestones || [])"
                :key="item.type"
                class="share-v2-milestone-card"
                :class="{ 'is-missing': !item.firstSeenAt }"
              >
                <div class="share-v2-milestone-icon">
                  <CheckCircle2 v-if="item.firstSeenAt" :size="18" />
                  <CircleSlash v-else :size="18" />
                </div>
                <div class="share-v2-milestone-body">
                  <span>{{ item.label }}</span>
                  <strong>{{ item.firstSeenAt ? formatShareMoment(item.firstSeenAt) : cleanShareText(item.missingText, '还没有记录') }}</strong>
                  <small v-if="item.firstSeenAt">{{ cleanShareText(item.detail || item.foundText || '已记录', '已记录') }}</small>
                </div>
                <em>{{ item.firstSeenAt ? '已完成' : '未完成' }}</em>
              </article>
            </div>
          </section>

          <section class="share-v2-panel share-v2-resource-panel share-v2-resource-panel-wide">
            <div class="share-v2-panel-heading share-v2-resource-heading">
              <div>
                <p class="share-v2-eyebrow">资源统计</p>
                <h2>矿物收获</h2>
                <p>记录已获取的矿物与尚未发现的资源</p>
              </div>
              <div class="share-v2-heading-count">
                <strong>{{ formatNumber(shareCountTotal(selectedShareServer.ores)) }}</strong>
                <span>个</span>
              </div>
            </div>
            <div class="share-v2-resource-grid share-v2-resource-featured">
              <article
                v-for="ore in (selectedShareServer.ores || []).filter(item => item.count)"
                :key="ore.type"
                class="share-v2-resource-item"
              >
                <span class="share-v2-resource-icon" :class="{ 'is-image': shareResourceIconPath(ore.type) }">
                  <img
                    v-if="shareResourceIconPath(ore.type)"
                    :src="shareResourceIconPath(ore.type)"
                    :alt="`${ore.label}图标`"
                    @error="markShareResourceIconFailed(ore.type)"
                  />
                  <Pickaxe v-else :size="17" />
                </span>
                <div class="share-v2-resource-body">
                  <div class="share-v2-resource-name"><span>{{ ore.label }}</span></div>
                  <div class="share-v2-resource-value">
                    <strong>{{ formatNumber(ore.count) }}</strong>
                  </div>
                </div>
              </article>
            </div>
            <div
              v-if="(selectedShareServer.ores || []).some(item => !item.count)"
              class="share-v2-unrecorded"
            >
              <span class="share-v2-unrecorded-label">尚未记录：</span>
              <div class="share-v2-unrecorded-list">
                <span>{{ (selectedShareServer.ores || []).filter(item => !item.count).map(item => item.label).join('、') }}</span>
              </div>
            </div>
          </section>

          <div class="share-v2-resource-pair">
            <section class="share-v2-panel share-v2-resource-panel share-v2-resource-panel-wood">
              <div class="share-v2-panel-heading share-v2-resource-heading">
                <div>
                  <p class="share-v2-eyebrow">原木收集</p>
                  <h2>砍树统计</h2>
                  <p>不同树木原木的累计获取数量</p>
                </div>
                <div class="share-v2-heading-count">
                  <strong>{{ formatNumber(shareCountTotal(selectedShareServer.woods)) }}</strong>
                  <span>个</span>
                </div>
              </div>
              <div class="share-v2-resource-grid share-v2-resource-featured">
                <article
                  v-for="wood in (selectedShareServer.woods || []).filter(item => item.count)"
                  :key="wood.type"
                  class="share-v2-resource-item"
                >
                  <span class="share-v2-resource-icon" :class="{ 'is-image': shareResourceIconPath(wood.type) }">
                    <img
                      v-if="shareResourceIconPath(wood.type)"
                      :src="shareResourceIconPath(wood.type)"
                      :alt="`${wood.label}图标`"
                      @error="markShareResourceIconFailed(wood.type)"
                    />
                    <TreePine v-else :size="17" />
                  </span>
                  <div class="share-v2-resource-body">
                    <div class="share-v2-resource-name"><span>{{ wood.label }}</span></div>
                    <div class="share-v2-resource-value">
                      <strong>{{ formatNumber(wood.count) }}</strong>
                    </div>
                  </div>
                </article>
              </div>
              <div
                v-if="(selectedShareServer.woods || []).some(item => !item.count)"
                class="share-v2-unrecorded"
              >
                <span class="share-v2-unrecorded-label">尚未记录：</span>
                <div class="share-v2-unrecorded-list">
                  <span>{{ (selectedShareServer.woods || []).filter(item => !item.count).map(item => item.label).join('、') }}</span>
                </div>
              </div>
            </section>

            <section class="share-v2-panel share-v2-resource-panel share-v2-resource-panel-saplings">
              <div class="share-v2-panel-heading share-v2-resource-heading">
                <div>
                  <p class="share-v2-eyebrow">种植记录</p>
                  <h2>栽树统计</h2>
                  <p>树苗与菌类的累计放置数量</p>
                </div>
                <div class="share-v2-heading-count">
                  <strong>{{ formatNumber(shareCountTotal(selectedShareServer.saplings)) }}</strong>
                  <span>棵</span>
                </div>
              </div>
              <div class="share-v2-resource-grid share-v2-resource-featured">
                <article
                  v-for="sapling in (selectedShareServer.saplings || []).filter(item => item.count)"
                  :key="sapling.type"
                  class="share-v2-resource-item"
                >
                  <span class="share-v2-resource-icon" :class="{ 'is-image': shareResourceIconPath(sapling.type) }">
                    <img
                      v-if="shareResourceIconPath(sapling.type)"
                      :src="shareResourceIconPath(sapling.type)"
                      :alt="`${sapling.label}图标`"
                      @error="markShareResourceIconFailed(sapling.type)"
                    />
                    <Sprout v-else :size="17" />
                  </span>
                  <div class="share-v2-resource-body">
                    <div class="share-v2-resource-name"><span>{{ sapling.label }}</span></div>
                    <div class="share-v2-resource-value">
                      <strong>{{ formatNumber(sapling.count) }}</strong>
                    </div>
                  </div>
                </article>
              </div>
              <div
                v-if="(selectedShareServer.saplings || []).some(item => !item.count)"
                class="share-v2-unrecorded"
              >
                <span class="share-v2-unrecorded-label">尚未记录：</span>
                <div class="share-v2-unrecorded-list">
                  <span>{{ (selectedShareServer.saplings || []).filter(item => !item.count).map(item => item.label).join('、') }}</span>
                </div>
              </div>
            </section>
          </div>
        </div>

        <aside class="share-v2-aside">
          <section class="share-v2-panel share-v2-overview-panel">
            <div class="share-v2-panel-heading share-v2-compact-heading">
              <div>
                <p class="share-v2-eyebrow">SERVER SNAPSHOT</p>
                <h2>记录概览</h2>
              </div>
              <span class="share-v2-server-pill">{{ selectedShareServer.serverName }}</span>
            </div>
            <div class="share-v2-facts">
              <div><span>当前服务器</span><strong>{{ selectedShareServer.serverName }}</strong></div>
              <div><span>首次记录时间</span><strong>{{ formatDateTime(selectedShareServer.firstSeenAt) }}</strong></div>
              <div><span>数据截止日期</span><strong>{{ selectedShareServer.latestLogDate || shareData.latestLogDate || '-' }}</strong></div>
              <div><span>分享链接状态</span><strong class="is-valid">{{ shareRemainingText }}</strong></div>
            </div>
          </section>

          <section class="share-v2-panel share-v2-coverage-panel">
            <div class="share-v2-panel-heading share-v2-compact-heading">
              <div>
                <p class="share-v2-eyebrow">DATA COVERAGE</p>
                <h2>数据覆盖</h2>
              </div>
              <Database :size="18" />
            </div>
            <div class="share-v2-coverage-list">
              <div><span>生存里程碑</span><strong>{{ (selectedShareServer.milestones || []).length }} 项</strong></div>
              <div><span>矿物项目</span><strong>{{ (selectedShareServer.ores || []).length }} 项</strong></div>
              <div><span>树木项目</span><strong>{{ (selectedShareServer.woods || []).length }} 项</strong></div>
              <div><span>栽树项目</span><strong>{{ (selectedShareServer.saplings || []).length }} 项</strong></div>
            </div>
          </section>

          <section class="share-v2-panel share-v2-note-panel">
            <div class="share-v2-note-icon"><Blocks :size="18" /></div>
            <div>
              <strong>这是一份独立服务器报告</strong>
              <p>切换上方服务器后，所有里程碑、矿物、砍树和栽树数据都会同步切换</p>
            </div>
          </section>
        </aside>
      </section>

      <footer class="share-v2-footer">
        <span>{{ selectedShareServer.serverName }}·我的游戏信息</span>
      </footer>
    </template>
  </main>

  <PublicReportLayout v-else-if="isXraySharePage" class="xray-share-report" :loading="xrayShareLoading" :error="xrayShareError" :ready="Boolean(xrayShareData && xraySharePlayer)" loading-title="正在打开分析" loading-text="正在读取矿透分析详情" error-title="无法打开分析">
    <template #header>
      <div>
        <p class="log-share-eyebrow">MINING ANALYSIS</p>
        <h1>{{ xraySharePlayer.playerName }}</h1>
        <p>{{ xrayShareData.serverName }} · {{ xrayAnalysisDateRange(xrayShareData) }}</p>
      </div>
      <div class="log-share-target">
        <span><Eye :size="16" /> 风险评估</span>
        <strong>{{ xraySharePlayer.riskScore }} <small>/ 100</small></strong>
        <small>{{ xraySharePlayer.riskLevel }} · {{ xrayShareRemainingText }}</small>
      </div>
    </template>
    <template #summary>
      <article><Hammer :size="19" /><span>周期破坏方块</span><strong>{{ formatNumber(xrayAnalysisValue(xraySharePlayer, 'analysisBreaks', 'miningSessionBreaks')) }}</strong></article>
      <article><Pickaxe :size="19" /><span>周期稀有矿</span><strong>{{ formatNumber(xrayAnalysisValue(xraySharePlayer, 'analysisRareOreBreaks', 'miningSessionRareOreBreaks')) }}</strong></article>
      <article><FileText :size="19" /><span>周期追矿证据</span><strong>{{ formatNumber(xrayAnalysisTrackingEvidenceCount(xraySharePlayer)) }}</strong></article>
      <article><Clock :size="19" /><span>十分钟挖取峰值</span><strong>{{ formatNumber(xrayAnalysisValue(xraySharePlayer, 'analysisPeakRareOreWindowCount', 'peakRareOreWindowCount')) }}</strong></article>
    </template>
    <section class="log-share-content">
      <div class="log-share-heading"><div><p>ANALYSIS DETAILS</p><h2>分析统计</h2></div><span>周期总量与单次会话分开展示</span></div>
      <section class="xray-share-stat-group">
        <header class="xray-share-stat-heading"><span class="xray-share-section-icon"><CalendarDays :size="19" /></span><div class="xray-share-section-title"><h3>筛选周期</h3><small>{{ xrayAnalysisDateRange(xrayShareData) }}</small></div></header>
        <PublicMetricGrid :items="xraySharePeriodMetrics" />
      </section>
      <section class="xray-share-stat-group">
        <header class="xray-share-stat-heading"><span class="xray-share-section-icon is-purple"><Pickaxe :size="19" /></span><div class="xray-share-section-title"><h3>最可疑挖矿会话</h3><small>单次会话的时间、矿物及挖掘行为</small></div><span class="xray-share-duration"><Clock :size="14" />持续 {{ xrayMiningSessionDuration(xraySharePlayer) }}</span></header>
        <dl class="xray-share-session-facts">
          <div><dt><CalendarDays :size="15" />会话时间</dt><dd>{{ formatReportTimeRange(xraySharePlayer.miningSessionStart, xraySharePlayer.miningSessionEnd) }}</dd></div>
          <div><dt><Clock :size="15" />挖取峰值窗口</dt><dd v-if="xraySharePlayer.peakRareOreWindowStart && xraySharePlayer.peakRareOreWindowEnd">{{ formatReportTimeRange(xraySharePlayer.peakRareOreWindowStart, xraySharePlayer.peakRareOreWindowEnd) }}</dd><dd v-else>-</dd></div>
        </dl>
        <PublicMetricGrid :items="xrayShareSessionMetrics" />
      </section>
      <section class="xray-share-stat-group">
        <header class="xray-share-stat-heading"><span class="xray-share-section-icon is-orange"><Layers :size="19" /></span><div class="xray-share-section-title"><h3>周期矿物明细</h3><small>按矿物类型查看周期挖取数量</small></div></header>
        <div v-if="xrayAnalysisOreCounts(xraySharePlayer).length" class="xray-share-ore-list">
          <div v-for="ore in xrayAnalysisOreCounts(xraySharePlayer)" :key="ore.oreType"><span class="xray-share-ore-icon"><Pickaxe :size="17" /></span><span>{{ ore.displayName }}</span><strong>{{ formatNumber(ore.count) }}<small> 个</small></strong></div>
        </div>
        <p v-else class="xray-share-empty">暂无矿物记录</p>
      </section>
    </section>
    <section class="log-share-content">
      <div class="log-share-heading"><div><p>RISK SUMMARY</p><h2>风险原因</h2></div><span>{{ xraySharePlayer.reasons?.length || 0 }} 条风险线索</span></div>
      <ul v-if="xraySharePlayer.reasons?.length" class="xray-share-reasons"><li v-for="reason in xraySharePlayer.reasons" :key="reason">{{ reason }}</li></ul>
      <p v-else class="xray-share-empty">没有风险原因记录</p>
    </section>
    <section class="log-share-content">
      <div class="log-share-heading">
        <div><p>EVIDENCE</p><h2>筛选周期证据</h2></div>
        <button v-if="xraySharePlayer.evidence?.length" class="xray-share-toggle" type="button" :aria-expanded="xrayShareEvidenceExpanded" @click="xrayShareEvidenceExpanded = !xrayShareEvidenceExpanded">
          <FileText :size="16" /><span>{{ xrayShareEvidenceExpanded ? '收起证据' : `查看 ${xraySharePlayer.evidence.length} 条证据` }}</span><ChevronDown :size="15" :class="{ 'xray-share-rotated': xrayShareEvidenceExpanded }" />
        </button>
      </div>
      <div v-if="xraySharePlayer.evidence?.length && xrayShareEvidenceExpanded" class="xray-share-evidence">
        <article v-for="(evidence, evidenceIndex) in xraySharePlayer.evidence" :key="`${xraySharePlayer.playerName}:${evidence.startedAt}:${evidence.summary}`" class="xray-share-evidence-card">
          <header class="xray-share-evidence-heading">
            <span class="xray-share-evidence-number">{{ String(evidenceIndex + 1).padStart(2, '0') }}</span>
            <div><h3>{{ evidence.summary }}</h3><p>{{ xrayEvidenceTime(evidence) }}</p></div>
          </header>
          <button v-if="evidence.rows?.length" class="xray-share-toggle xray-share-toggle-subtle" type="button" :aria-expanded="isShareXrayEvidenceRowsExpanded(evidence, evidenceIndex)" @click="toggleShareXrayEvidenceRows(evidence, evidenceIndex)">
            <FileText :size="15" /><span>{{ isShareXrayEvidenceRowsExpanded(evidence, evidenceIndex) ? '收起原始记录' : `查看 ${evidence.rows.length} 条原始记录` }}</span><ChevronDown :size="14" :class="{ 'xray-share-rotated': isShareXrayEvidenceRowsExpanded(evidence, evidenceIndex) }" />
          </button>
          <ol v-if="evidence.rows?.length && isShareXrayEvidenceRowsExpanded(evidence, evidenceIndex)" class="xray-share-evidence-lines">
            <li v-for="row in evidence.rows" :key="`${row.filePath}:${row.lineNumber}`">{{ evidenceRowText(row) }}</li>
          </ol>
        </article>
      </div>
      <p v-else class="xray-share-empty">{{ xraySharePlayer.evidence?.length ? '按需展开证据，查看时间范围与对应的原始记录' : '没有追矿证据' }}</p>
    </section>
    <section id="xray-share-rare-ore-positions" class="log-share-content">
      <div class="log-share-heading">
        <div><p>ORE POSITIONS</p><h2>稀有矿位置</h2></div>
        <button v-if="xrayShareRareOreRows.length" class="xray-share-toggle" type="button" :aria-expanded="xrayShareOrePositionsExpanded" @click="toggleXrayShareOrePositions"><Crosshair :size="16" /><span>{{ xrayShareOrePositionsExpanded ? '收起位置' : `查看 ${xrayShareRareOreRows.length} 条位置` }}</span><ChevronDown :size="15" :class="{ 'xray-share-rotated': xrayShareOrePositionsExpanded }" /></button>
      </div>
      <template v-if="xrayShareOrePositionsExpanded && xrayShareRareOreRows.length">
        <div class="log-share-pagination xray-share-pagination">
          <span>第 {{ clampPage(xrayShareOrePositionPage, xrayShareOrePositionTotalPages) }} / {{ xrayShareOrePositionTotalPages }} 页，共 {{ formatNumber(xrayShareRareOreRows.length) }} 条</span>
          <div>
            <label>每页
              <select :value="xrayShareOrePositionPageSize" @change="setXrayShareOrePositionPageSize($event.target.value)">
                <option v-for="size in XRAY_ORE_POSITION_PAGE_SIZE_OPTIONS" :key="size" :value="size">{{ size }}</option>
              </select>
            </label>
            <button type="button" title="上一页" aria-label="上一页" :disabled="xrayShareOrePositionPage <= 1" @click="setXrayShareOrePositionPage(xrayShareOrePositionPage - 1)"><ChevronLeft :size="18" /></button>
            <button type="button" title="下一页" aria-label="下一页" :disabled="xrayShareOrePositionPage >= xrayShareOrePositionTotalPages" @click="setXrayShareOrePositionPage(xrayShareOrePositionPage + 1)"><ChevronRight :size="18" /></button>
          </div>
        </div>
        <div class="log-share-table-wrap">
          <table class="xray-ore-position-table">
            <thead><tr><th>顺序</th><th>时间</th><th>矿物</th><th>玩家坐标</th><th>交互坐标</th></tr></thead>
            <tbody><tr v-for="(row, index) in xraySharePagedRareOreRows" :key="`${row.filePath}:${row.lineNumber}:share-rare`">
              <td>{{ xrayShareOrePositionOffset + index + 1 }}</td><td>{{ row.date }} {{ row.time }}</td>
              <td class="message-cell">{{ logQuerySummaryDetailText(row) }}</td><td>{{ logQueryCoordinateText(row.x, row.y, row.z, row.dimension) }}</td><td>{{ logQueryInteractionCoordinateText(row) }}</td>
            </tr></tbody>
          </table>
        </div>
        <div class="log-share-cards">
          <article v-for="(row, index) in xraySharePagedRareOreRows" :key="`${row.filePath}:${row.lineNumber}:share-card`">
            <div><strong>{{ xrayShareOrePositionOffset + index + 1 }} · {{ logQuerySummaryDetailText(row) }}</strong><span>{{ row.date }} {{ row.time }}</span></div>
            <p>交互坐标：{{ logQueryInteractionCoordinateText(row) }}</p>
            <small><Crosshair :size="14" />玩家坐标：{{ logQueryCoordinateText(row.x, row.y, row.z, row.dimension) }}</small>
          </article>
        </div>
        <div class="log-share-pagination xray-share-pagination">
          <span>第 {{ clampPage(xrayShareOrePositionPage, xrayShareOrePositionTotalPages) }} / {{ xrayShareOrePositionTotalPages }} 页，共 {{ formatNumber(xrayShareRareOreRows.length) }} 条</span>
          <div>
            <label>每页
              <select :value="xrayShareOrePositionPageSize" @change="setXrayShareOrePositionPageSize($event.target.value)">
                <option v-for="size in XRAY_ORE_POSITION_PAGE_SIZE_OPTIONS" :key="size" :value="size">{{ size }}</option>
              </select>
            </label>
            <button type="button" title="上一页" aria-label="上一页" :disabled="xrayShareOrePositionPage <= 1" @click="setXrayShareOrePositionPage(xrayShareOrePositionPage - 1)"><ChevronLeft :size="18" /></button>
            <button type="button" title="下一页" aria-label="下一页" :disabled="xrayShareOrePositionPage >= xrayShareOrePositionTotalPages" @click="setXrayShareOrePositionPage(xrayShareOrePositionPage + 1)"><ChevronRight :size="18" /></button>
          </div>
        </div>
        <button class="xray-share-toggle xray-share-back-top" type="button" @click="scrollToXrayShareOrePositions"><ArrowUp :size="16" />返回稀有矿位置</button>
      </template>
      <p v-else class="xray-share-empty">{{ xrayShareRareOreRows.length ? `共 ${formatNumber(xrayShareRareOreRows.length)} 条位置记录，展开后查看` : '没有稀有矿位置记录' }}</p>
    </section>
    <template #footer>{{ xrayShareData.serverName }} · 矿透分析</template>
  </PublicReportLayout>

  <PublicReportLayout v-else-if="isLogSharePage" :loading="logShareLoading" :error="logShareError" :ready="Boolean(logShareData)">
    <template #header>
        <div>
          <p class="log-share-eyebrow">SERVER LOGS</p>
          <h1>服务器日志</h1>
          <p>{{ logShareData.serverName }} · {{ logShareData.fromDate || '-' }} 至 {{ logShareData.toDate || '-' }}</p>
        </div>
        <div class="log-share-target">
          <span><Crosshair :size="16" /> 查询中心</span>
          <strong>{{ formatCoordValue(logShareData.x) }}, {{ formatCoordValue(logShareData.y) }}, {{ formatCoordValue(logShareData.z) }}</strong>
          <small>半径 {{ formatCoordValue(logShareData.radius) }} 格 · {{ logShareRemainingText }}</small>
        </div>
    </template>
    <template #summary>
        <article><Search :size="19" /><span>匹配日志</span><strong>{{ formatNumber(logShareData.matchedRows || 0) }}</strong></article>
        <article><FileText :size="19" /><span>日志总数</span><strong>{{ formatNumber(logShareTotalRows) }}</strong></article>
        <article><Database :size="19" /><span>扫描文件</span><strong>{{ formatNumber(logShareData.scannedFiles || 0) }}</strong></article>
        <article><Clock :size="19" /><span>链接状态</span><strong>{{ logShareRemainingText }}</strong></article>
    </template>
      <section class="log-share-content">
        <div class="log-share-heading">
          <div>
            <p>LOG ENTRIES</p>
            <h2>日志明细</h2>
          </div>
          <span>共 {{ formatNumber(logShareTotalRows) }} 条日志，当前按页加载</span>
        </div>

        <div v-if="logSharePagedRows.length" class="log-share-table-wrap">
          <table>
            <thead>
              <tr><th>时间</th><th>玩家</th><th>行为</th><th>位置</th><th>详情</th></tr>
            </thead>
            <tbody>
              <tr v-for="row in logSharePagedRows" :key="`${row.fileName}:${row.lineNumber}`">
                <td>{{ row.date }} {{ row.time }}</td>
                <td><strong>{{ row.playerName || '-' }}</strong></td>
                <td><span class="log-share-action" :class="logShareActionClass(row.action)">{{ logQueryActionDisplay(row.action) }}</span></td>
                <td>{{ logQueryInteractionCoordinateText(row) }}</td>
                <td class="message-cell">{{ logQuerySummaryDetailText(row) }}</td>
              </tr>
            </tbody>
          </table>
        </div>

        <div v-if="logSharePagedRows.length" class="log-share-cards">
          <article v-for="row in logSharePagedRows" :key="`${row.fileName}:${row.lineNumber}:card`" :class="logShareActionClass(row.action)">
            <div><strong>{{ row.playerName || '-' }}</strong><span>{{ row.date }} {{ row.time }}</span></div>
            <p>{{ logQueryActionDisplay(row.action) }} · {{ logQuerySummaryDetailText(row) }}</p>
            <small><Crosshair :size="14" />{{ logQueryInteractionCoordinateText(row) }}</small>
          </article>
        </div>

        <p v-else class="log-share-empty">{{ logShareData.message || '没有匹配日志' }}</p>

        <div v-if="logShareTotalRows" class="log-share-pagination">
          <span>第 {{ clampPage(logSharePage, logShareTotalPages) }} / {{ logShareTotalPages }} 页，共 {{ formatNumber(logShareTotalRows) }} 条</span>
          <div>
            <label>每页
              <select :value="logSharePageSize" :disabled="logSharePageLoading" @change="setLogSharePageSize($event.target.value)">
                <option v-for="size in LOG_SHARE_PAGE_SIZE_OPTIONS" :key="size" :value="size">{{ size }}</option>
              </select>
            </label>
            <button type="button" title="上一页" aria-label="上一页" :disabled="logSharePageLoading || logSharePage <= 1" @click="setLogSharePage(logSharePage - 1)"><ChevronLeft :size="18" /></button>
            <button type="button" title="下一页" aria-label="下一页" :disabled="logSharePageLoading || logSharePage >= logShareTotalPages" @click="setLogSharePage(logSharePage + 1)"><ChevronRight :size="18" /></button>
          </div>
        </div>
      </section>

    <template #footer>{{ logShareData.serverName }} · 服务器日志</template>
  </PublicReportLayout>

  <main v-else-if="isRankingSharePage" class="ranking-share-v2">
    <section v-if="rankingShareLoading" class="share-v2-state">
      <div class="share-v2-state-mark"><Clock :size="24" /></div>
      <div>
        <h1>正在打开排行榜</h1>
        <p>正在读取排行数据</p>
      </div>
    </section>

    <section v-else-if="rankingShareError" class="share-v2-state share-v2-state-error">
      <div class="share-v2-state-mark"><Clock :size="24" /></div>
      <div>
        <h1>链接已过期</h1>
        <p>{{ rankingShareError }}</p>
      </div>
    </section>

    <template v-else-if="rankingShareData && rankingShareServers.length">
      <header class="ranking-share-hero">
        <div class="ranking-share-heading">
          <div>
            <span class="ranking-share-server">
              <Layers :size="14" />
              {{ selectedRankingShareServer?.serverName }}
            </span>
            <h1>{{ rankingShareTitle }}</h1>
            <p>
              {{ isActiveRankingShare
                ? `按最近 ${rankingSharePeriodDays || ''} 天的方块活动排序`
                : '按破坏与放置方块总数排序' }}
            </p>
            <span class="ranking-share-period">
              <CalendarDays :size="15" />
              {{ rankingShareData.fromDate || '-' }} 至 {{ rankingShareData.toDate || '-' }}
            </span>
          </div>
        </div>
      </header>

      <section class="share-v2-server-bar" aria-label="切换服务器">
        <a
          class="share-v2-server-context"
          :href="rankingMinecraftJoinUrl"
          @click="minecraftLaunchNoticeOpen = true"
        >
          <span class="share-v2-context-icon"><Database :size="18" /></span>
          <div>
            <p>MINECRAFT</p>
            <strong>{{ selectedRankingShareServer?.serverName || '-' }}</strong>
            <small>点击启动游戏</small>
          </div>
        </a>
        <nav class="share-v2-server-tabs" aria-label="服务器">
          <button
            v-for="server in rankingShareServers"
            :key="server.serverId"
            type="button"
            :class="{ active: selectedRankingShareServerId === server.serverId }"
            @click="selectedRankingShareServerId = server.serverId"
          >
            <Layers :size="15" />
            <span>{{ server.serverName }}</span>
          </button>
        </nav>
      </section>

      <section class="share-v2-stat-grid" aria-label="榜单统计">
        <article class="share-v2-stat-card is-blue">
          <div class="share-v2-stat-icon"><UserRound :size="20" /></div>
          <div>
            <span>展示人数</span>
            <strong>{{ rankingShareData.limit }}</strong>
            <small>展示排名前 {{ rankingShareData.limit }} 名玩家</small>
          </div>
        </article>
        <article class="share-v2-stat-card is-orange">
          <div class="share-v2-stat-icon"><Hammer :size="20" /></div>
          <div>
            <span>{{ isActiveRankingShare ? '期间破坏' : '累计破坏' }}</span>
            <strong>{{ formatNumber(rankingShareTotals.brokenCount) }}</strong>
            <small>{{ isActiveRankingShare ? '统计周期内合计' : '上榜玩家合计' }}</small>
          </div>
        </article>
        <article class="share-v2-stat-card is-green">
          <div class="share-v2-stat-icon"><Layers :size="20" /></div>
          <div>
            <span>{{ isActiveRankingShare ? '期间放置' : '累计放置' }}</span>
            <strong>{{ formatNumber(rankingShareTotals.placedCount) }}</strong>
            <small>{{ isActiveRankingShare ? '统计周期内合计' : '上榜玩家合计' }}</small>
          </div>
        </article>
        <article class="share-v2-stat-card is-purple ranking-v2-leader-stat">
          <div class="share-v2-stat-icon"><Crown :size="20" /></div>
          <div>
            <span>榜首</span>
            <strong>{{ formatNumber(rankingSharePlayers[0]?.totalCount || 0) }}</strong>
            <small>{{ isActiveRankingShare ? '活跃第一' : '当前榜首' }} {{ rankingSharePlayers[0]?.playerName || '-' }}</small>
          </div>
        </article>
      </section>

      <section class="share-v2-panel ranking-v2-panel">
        <div class="share-v2-panel-heading">
          <div>
            <p class="share-v2-eyebrow">{{ isActiveRankingShare ? '近期活跃' : '玩家排行' }}</p>
            <h2>{{ selectedRankingShareServer?.serverName || '' }}{{ rankingShareTitle }}</h2>
            <p>{{ isActiveRankingShare ? `统计最近 ${rankingSharePeriodDays || ''} 天的破坏与放置行为` : '破坏与放置方块数量越高，排名越靠前' }}</p>
          </div>
        </div>

        <div v-if="rankingSharePlayers.length" class="ranking-v2-list">
          <article
            v-for="(player, index) in rankingSharePlayers"
            :key="player.playerName"
            class="ranking-v2-row"
            :class="{
              'is-first': index === 0,
              'is-second': index === 1,
              'is-third': index === 2
            }"
          >
            <span class="ranking-v2-rank">
              <Crown v-if="index === 0" :size="17" />
              <b v-else>{{ index + 1 }}</b>
            </span>
            <div class="ranking-v2-player">
              <strong>{{ player.playerName }}</strong>
              <span>第 {{ index + 1 }} 名</span>
            </div>
            <div class="ranking-v2-metric is-broken">
              <span>破坏</span>
              <strong>{{ formatNumber(player.brokenCount) }}</strong>
              <small>{{ shareBreakPercent(player) }}%</small>
            </div>
            <div class="ranking-v2-metric is-placed">
              <span>放置</span>
              <strong>{{ formatNumber(player.placedCount) }}</strong>
              <small>{{ sharePlacePercent(player) }}%</small>
            </div>
            <div class="ranking-v2-mobile-breakdown">
              <div class="ranking-v2-mobile-values">
                <span>
                  破坏 <b>{{ formatNumber(player.brokenCount) }}</b>
                  <small>{{ shareBreakPercent(player) }}%</small>
                </span>
                <span>
                  放置 <b>{{ formatNumber(player.placedCount) }}</b>
                  <small>{{ sharePlacePercent(player) }}%</small>
                </span>
              </div>
              <div
                class="ranking-v2-mobile-bar"
                :style="{ '--break-percent': `${shareBreakPercent(player)}%` }"
                aria-label="破坏与放置占比"
              >
                <span class="is-broken"></span>
                <span class="is-placed"></span>
              </div>
            </div>
            <div class="ranking-v2-total">
              <span>{{ isActiveRankingShare ? '近期活动' : '总活动' }}</span>
              <strong>{{ formatNumber(player.totalCount) }}</strong>
            </div>
          </article>
        </div>
        <p v-else class="ranking-v2-empty">该服务器暂无排行数据</p>
      </section>

      <footer class="share-v2-footer">
        <span>{{ selectedRankingShareServer?.serverName }}·{{ rankingShareTitle }}</span>
      </footer>
    </template>
  </main>

  <AdminLogin v-else-if="!authChecked || !authenticated" :checking="!authChecked" :loading="loginLoading" :error="loginError" v-model:username="loginForm.username" v-model:password="loginForm.password" @submit="login" />

  <AdminLayout v-else :active="activeAdminPage" :account="currentAdmin" :server-time="serverTimeDisplay" @navigate="navigateAdmin" @logout="logout">
  <main class="app-shell">
    <SettingsPage v-if="currentView === 'settingsPage'" ref="settingsPage" @unauthorized="handleApiError(new Error('登录已过期，请重新登录'))" />

    <section v-else-if="currentView === 'profilePage'" class="result-page">
      <div class="topbar">
        <div class="actions">
          <span class="admin-badge">
            <UserRound :size="16" />
            {{ currentAdmin }}
          </span>
        </div>
      </div>

      <p v-if="error" class="notice error">{{ error }}</p>
      <p v-if="importMessage" class="notice success">{{ importMessage }}</p>

      <div class="settings-grid">
        <article class="panel wide">
          <div class="panel-title">
            <h2>账号信息</h2>
          </div>
          <div class="settings-form profile-form">
            <label>
              <span>账号</span>
              <input type="text" :value="currentAdmin" disabled />
            </label>
            <label>
              <span>登录有效期</span>
              <input type="text" :value="formatDateTime(currentAdminExpiresAt)" disabled />
            </label>
          </div>
        </article>

        <article class="panel wide">
          <div class="panel-title">
            <h2>修改密码</h2>
          </div>
          <form class="settings-form profile-form" @submit.prevent="changePassword">
            <label>
              <span>当前密码</span>
              <div class="password-field">
                <input :type="showCurrentPassword ? 'text' : 'password'" v-model="passwordForm.currentPassword" autocomplete="current-password" />
                <button type="button" class="icon-button ghost password-toggle" @click="showCurrentPassword = !showCurrentPassword" tabindex="-1">
                  <EyeOff v-if="showCurrentPassword" :size="16" />
                  <Eye v-else :size="16" />
                </button>
              </div>
            </label>
            <label>
              <span>新密码</span>
              <div class="password-field">
                <input :type="showNewPassword ? 'text' : 'password'" v-model="passwordForm.newPassword" autocomplete="new-password" />
                <button type="button" class="icon-button ghost password-toggle" @click="showNewPassword = !showNewPassword" tabindex="-1">
                  <EyeOff v-if="showNewPassword" :size="16" />
                  <Eye v-else :size="16" />
                </button>
              </div>
            </label>
            <label>
              <span>确认新密码</span>
              <div class="password-field">
                <input :type="showConfirmPassword ? 'text' : 'password'" v-model="passwordForm.confirmPassword" autocomplete="new-password" />
                <button type="button" class="icon-button ghost password-toggle" @click="showConfirmPassword = !showConfirmPassword" tabindex="-1">
                  <EyeOff v-if="showConfirmPassword" :size="16" />
                  <Eye v-else :size="16" />
                </button>
              </div>
            </label>
            <div class="settings-actions">
              <button class="primary-button" type="submit" :disabled="passwordSaving">
                <Save :size="18" />
                <span>{{ passwordSaving ? '保存中' : '保存密码' }}</span>
              </button>
            </div>
          </form>
        </article>
      </div>
    </section>

    <GroupChatCopyPage v-else-if="currentView === 'txtCopyPage'" @settings="openSettingsPage" />
    <GroupChatPage v-else-if="currentView === 'groupChatPage'" />

    <FileSyncPage v-else-if="currentView === 'syncPage'" :key="filters.serverId" extension="CSV" :scope-name="selectedServerName" :remote-files="remoteFiles" :local-files="syncLocalFiles" :job="syncJob" :loading="syncPageLoading" :starting="syncingFiles" :deleting="deletingLocalCsv" v-model:skip-today="syncSkipToday" :error="error" :message="importMessage" @refresh="loadSyncPageFiles" @start="startSyncJob" @delete="deleteSelectedLocalCsv">
      <template #scope><div class="admin-server-picker"><AdminSelect :model-value="filters.serverId" :options="adminServerOptions" label="任务服务器" caption="服务器" @update:model-value="openSyncPage" /></div></template>
    </FileSyncPage>

    <section v-else-if="currentView === 'importPage'" class="result-page">
      <div class="topbar">
        <div class="actions">
          <div class="admin-server-picker">
            <AdminSelect :model-value="filters.serverId" :options="adminServerOptions" label="任务服务器" caption="服务器" @update:model-value="openImportPage" />
          </div>
          <button class="icon-button ghost" type="button" title="刷新文件" :disabled="importPageLoading || importJobRunning" @click="loadImportFiles">
            <RefreshCw :size="18" :class="{ spin: importPageLoading }" />
          </button>
          <label class="toggle-label">
            <input type="checkbox" v-model="importSkipToday" :disabled="importJobRunning" />
            <span>跳过当天文件</span>
          </label>
          <button class="primary-button" type="button" :disabled="importing || importJobRunning || deletingImportRecords || !importPageFiles.length" @click="startImportJob">
            <Play :size="18" />
            <span>{{ importJobRunning ? '解析中' : `开始解析 ${selectedServerName}` }}</span>
          </button>
        </div>
      </div>

      <p v-if="error" class="notice error">{{ error }}</p>
      <p v-if="importMessage" class="notice success">{{ importMessage }}</p>

      <section class="result-summary-grid">
        <article class="metric-card compact">
          <div class="metric-icon blue"><FileText :size="20" /></div>
          <p>本地 CSV</p>
          <strong>{{ formatNumber(importPageFiles.length) }}</strong>
        </article>
        <article class="metric-card compact">
          <div class="metric-icon green"><CheckCircle2 :size="20" /></div>
          <p>已导入</p>
          <strong>{{ formatNumber(importJob?.importedFiles || importFiles.filter((file) => file.status === 'IMPORTED').length) }}</strong>
        </article>
        <article class="metric-card compact">
          <div class="metric-icon amber"><CircleSlash :size="20" /></div>
          <p>跳过</p>
          <strong>{{ formatNumber(importJob?.skippedFiles || importFiles.filter((file) => file.status === 'SKIPPED_TODAY').length) }}</strong>
        </article>
        <article class="metric-card compact">
          <div class="metric-icon red"><XCircle :size="20" /></div>
          <p>失败</p>
          <strong>{{ formatNumber(importJob?.failedFiles || 0) }}</strong>
        </article>
      </section>

      <article class="panel wide">
        <div class="panel-title">
          <h2>文件解析情况</h2>
          <span>{{ importJob?.status || 'READY' }}</span>
        </div>
        <p v-if="deleteRecordPollError" class="notice error">{{ deleteRecordPollError }}</p>
        <div class="bulk-actions">
          <label class="bulk-check">
            <input
              type="checkbox"
              :checked="allEligibleSelected"
              :disabled="importJobRunning || deletingImportRecords || !eligibleImportFiles.length"
              @change="toggleAllImportFileSelection($event.target.checked)"
            />
            <span>选择可删除记录</span>
          </label>
          <button
            class="table-action danger"
            type="button"
            :disabled="importJobRunning || deletingImportRecords || !selectedResetCount"
            @click="deleteSelectedImportRecords"
          >
            <Trash2 :size="15" />
            <span>{{ resetRemotePath === '__batch__' ? `批量删除中 ${deleteRecordJob?.processedFiles || 0} / ${deleteRecordJob?.totalFiles || selectedResetCount}` : `批量删记录 ${selectedResetCount}` }}</span>
          </button>
        </div>
        <div class="table-wrap">
          <table>
            <thead>
              <tr>
                <th>选择</th>
                <th>文件</th>
                <th>服务器</th>
                <th>状态</th>
                <th>大小</th>
                <th>读取行</th>
                <th>忽略行</th>
                <th>说明</th>
                <th>操作</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="file in importPageFiles" :key="`${file.serverId}:${file.remotePath}`">
                <td>
                  <input
                    type="checkbox"
                    :checked="selectedResetKeys.has(importFileKey(file))"
                    :disabled="importJobRunning || deletingImportRecords || !canDeleteImportRecord(file)"
                    @change="toggleImportFileSelection(file, $event.target.checked)"
                  />
                </td>
                <td class="path-cell" :title="file.remotePath">{{ file.fileName || fileNameFromPath(file.remotePath) }}</td>
                <td>{{ file.serverName }}</td>
                <td>
                  <span class="status-pill" :class="statusClass(file.status)">
                    {{ statusLabel(file.status) }}
                  </span>
                </td>
                <td>{{ fileSize(file.fileSize) }}</td>
                <td>{{ formatNumber(file.rowCount) }}</td>
                <td>{{ formatNumber(file.ignoredCount) }}</td>
                <td class="message-cell">{{ file.message || '-' }}</td>
                <td>
                  <button
                    class="table-action danger"
                    type="button"
                    :disabled="importJobRunning || deletingImportRecords || !canDeleteImportRecord(file)"
                    @click="deleteImportRecord(file)"
                  >
                    <Trash2 :size="15" />
                    <span>{{ resetRemotePath === file.remotePath ? '删除中' : '删记录' }}</span>
                  </button>
                </td>
              </tr>
              <tr v-if="!importPageFiles.length">
                <td colspan="9" class="empty">{{ importPageLoading ? '正在扫描本地 CSV' : '暂无本地 CSV 文件' }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </article>
    </section>

    <section v-else-if="currentView === 'logQueryPage' || currentView === 'xrayAnalysisPage'" class="result-page log-query-page" :class="{ 'xray-analysis-page': currentView === 'xrayAnalysisPage' }">
      <div class="log-query-toolbar">
        <div class="log-query-toolbar-actions">
          <div class="admin-server-picker">
            <AdminSelect :model-value="logQueryServerId" :options="adminServerOptions" label="服务器" caption="服务器" @update:model-value="setLogQueryServer" />
          </div>
          <button class="icon-button ghost" type="button" :title="currentView === 'xrayAnalysisPage' ? '刷新分析状态' : '刷新查询状态'" :disabled="selectedLogQueryLoading" @click="loadLogFeatureState(logQueryServerId)">
            <RefreshCw :size="18" :class="{ spin: selectedLogQueryLoading || selectedLogQueryRunning }" />
          </button>
        </div>
      </div>

      <LogQueryTabs v-if="currentView === 'logQueryPage'" :mode="logQueryMode" @change="setLogQueryMode" />

      <p v-if="error" class="notice error">{{ error }}</p>
      <p v-if="importMessage" class="notice success">{{ importMessage }}</p>

      <article class="panel wide log-filter-panel">
        <div class="panel-title">
          <h2>{{ serverNameById(logQueryServerId) }} {{ logQueryModeLabel(logQueryMode) }}</h2>
          <div class="log-query-task-status">
            <span>{{ logQueryStatusLabel(selectedLogFeatureResult?.status) }}</span>
            <button v-if="selectedLogQueryRunning" class="secondary-button" type="button" :disabled="selectedLogQueryCancelling" @click="cancelLogQuery(logQueryServerId)">
              <CircleSlash :size="16" /><span>{{ selectedLogQueryCancelling ? '取消中…' : '取消任务' }}</span>
            </button>
          </div>
        </div>

        <form v-if="logQueryMode === LOG_QUERY_TYPE_COORDINATE" :key="logQueryServerId" class="log-query-form log-coordinate-form" @submit.prevent="startLogQuery(logQueryServerId)">
          <div class="log-coordinate-mode" role="group" aria-label="坐标查询范围方式">
            <button v-for="option in coordinateRangeOptions" :key="option.value" type="button" :aria-pressed="logQueryForms[logQueryServerId].rangeMode === option.value" :class="{ active: logQueryForms[logQueryServerId].rangeMode === option.value }" @click="logQueryForms[logQueryServerId].rangeMode = option.value; saveLogQueryForms()">{{ option.label }}</button>
          </div>
          <label>
            <span>开始日期</span>
            <input v-model="logQueryForms[logQueryServerId].fromDate" type="date" required @change="saveLogQueryForms" />
          </label>
          <label>
            <span>结束日期</span>
            <input v-model="logQueryForms[logQueryServerId].toDate" type="date" required @change="saveLogQueryForms" />
          </label>
          <label class="log-coordinate-dimension">
            <span>维度</span>
            <input v-model.trim="logQueryForms[logQueryServerId].dimension" type="text" placeholder="主世界/下界/末地，留空不限" @change="saveLogQueryForms" />
          </label>
          <template v-if="logQueryForms[logQueryServerId].rangeMode === 'radius'">
            <label>
              <span>中心交互坐标</span>
              <input key="radius-center" v-model="logQueryForms[logQueryServerId].centerPosition" type="text" placeholder="X Y Z，例如：34 35 523" required @input="validateLogCoordinateInput" @change="saveLogQueryForms" />
            </label>
            <label>
              <span>半径（格）</span>
              <input v-model="logQueryForms[logQueryServerId].radius" type="number" min="0" max="200" step="any" placeholder="例如：5" required :aria-invalid="Boolean(logQueryRadiusError)" :aria-describedby="logQueryRadiusError ? 'log-query-radius-error' : undefined" @change="saveLogQueryForms" />
              <small v-if="logQueryRadiusError" id="log-query-radius-error" class="log-query-field-error" role="status">{{ logQueryRadiusError }}</small>
            </label>
          </template>
          <template v-else>
            <label>
              <span>交互坐标范围起点</span>
              <input v-model="logQueryForms[logQueryServerId].position1" type="text" placeholder="X Y Z，例如：34 35 523" required @input="validateLogCoordinateInput" @change="saveLogQueryForms" />
            </label>
            <label>
              <span>交互坐标范围终点</span>
              <input v-model="logQueryForms[logQueryServerId].position2" type="text" placeholder="X Y Z，例如：40 40 530" required @input="validateLogCoordinateInput" @change="saveLogQueryForms" />
            </label>
          </template>
          <div class="log-query-actions">
            <button class="primary-button" type="submit" :disabled="selectedLogQueryRunning || selectedLogQueryLoading">
              <Search :size="18" />
              <span>{{ selectedLogQueryRunning ? '查询中' : `开始查询 ${serverNameById(logQueryServerId)}` }}</span>
            </button>
            <button class="secondary-button" type="button" :disabled="selectedLogQueryRunning" @click="clearLogQuery(logQueryServerId)">
              <Trash2 :size="18" />
              <span>清空结果</span>
            </button>
          </div>
        </form>

        <form v-else-if="logQueryMode === LOG_QUERY_TYPE_PLAYER_KEYWORD" class="log-query-form log-combined-form" @submit.prevent="startLogQuery(logQueryServerId)">
          <label>
            <span>开始日期（留空不限）</span>
            <input v-model="logTextQueryForms[logQueryServerId].fromDate" type="date" @change="saveLogTextQueryForms" />
          </label>
          <label>
            <span>结束日期（留空不限）</span>
            <input v-model="logTextQueryForms[logQueryServerId].toDate" type="date" @change="saveLogTextQueryForms" />
          </label>
          <PlayerNameAutocomplete v-model="logTextQueryForms[logQueryServerId].playerName" :server-id="logQueryServerId" @change="saveLogTextQueryForms" />
          <label>
            <span>关键字</span>
            <input v-model.trim="logTextQueryForms[logQueryServerId].keyword" type="text" placeholder="详细信息关键字" @change="saveLogTextQueryForms" />
          </label>
          <div class="log-combined-footer">
            <LogActionFilter v-model="logTextQueryForms[logQueryServerId].actions" @update:model-value="saveLogTextQueryForms" />
            <div class="log-query-actions">
              <button class="primary-button" type="submit" :disabled="selectedLogQueryRunning || selectedLogQueryLoading">
                <Search :size="18" />
                <span>{{ selectedLogQueryRunning ? '查询中' : `开始查询 ${serverNameById(logQueryServerId)}` }}</span>
              </button>
              <button class="secondary-button" type="button" :disabled="selectedLogQueryRunning" @click="clearLogQuery(logQueryServerId)">
                <Trash2 :size="18" />
                <span>清空结果</span>
              </button>
            </div>
          </div>
        </form>

        <form v-else class="log-query-form" @submit.prevent="startLogQuery(logQueryServerId)">
          <label>
            <span>开始日期</span>
            <input v-model="xrayAnalysisForms[logQueryServerId].fromDate" type="date" required @change="saveXrayAnalysisForms" />
          </label>
          <label>
            <span>结束日期</span>
            <input v-model="xrayAnalysisForms[logQueryServerId].toDate" type="date" required @change="saveXrayAnalysisForms" />
          </label>
          <PlayerNameAutocomplete v-model="xrayAnalysisForms[logQueryServerId].playerName" :server-id="logQueryServerId" placeholder="留空分析全部玩家" @change="saveXrayAnalysisForms" />
          <label>
            <span>维度</span>
            <input v-model.trim="xrayAnalysisForms[logQueryServerId].dimension" type="text" placeholder="主世界/下界，留空不限" @change="saveXrayAnalysisForms" />
          </label>
          <div class="log-query-actions">
            <button class="primary-button" type="submit" :disabled="selectedLogQueryRunning || selectedLogQueryLoading">
              <Eye :size="18" />
              <span>{{ selectedLogQueryRunning ? '分析中' : `开始分析 ${serverNameById(logQueryServerId)}` }}</span>
            </button>
            <button class="secondary-button" type="button" :disabled="selectedLogQueryRunning" @click="clearLogQuery(logQueryServerId)">
              <Trash2 :size="18" />
              <span>清空结果</span>
            </button>
          </div>
        </form>
      </article>

      <section v-if="selectedLogFeatureHasResult" class="result-summary-grid">
        <article class="metric-card compact">
          <div class="metric-icon blue"><FileText :size="20" /></div>
          <p>扫描文件</p>
          <strong>{{ formatNumber(logQueryMode === LOG_QUERY_TYPE_XRAY ? selectedXrayAnalysisResult?.scannedFiles || 0 : selectedLogQueryResult?.scannedFiles || 0) }}</strong>
        </article>
        <article class="metric-card compact">
          <div class="metric-icon green"><Search :size="20" /></div>
          <p>{{ logQueryMode === LOG_QUERY_TYPE_XRAY ? '分析玩家' : '匹配事件' }}</p>
          <strong>{{ formatNumber(logQueryMode === LOG_QUERY_TYPE_XRAY ? selectedXrayAnalysisResult?.playerCount || 0 : selectedLogQueryResult?.matchedRows || 0) }}</strong>
        </article>
        <article class="metric-card compact">
          <div class="metric-icon amber"><Database :size="20" /></div>
          <p>{{ logQueryMode === LOG_QUERY_TYPE_XRAY ? '最高风险' : '扫描行数' }}</p>
          <strong>{{ formatNumber(logQueryMode === LOG_QUERY_TYPE_XRAY ? selectedXrayAnalysisResult?.maxRiskScore || 0 : selectedLogQueryResult?.scannedRows || 0) }}</strong>
        </article>
        <article class="metric-card compact">
          <div class="metric-icon red"><XCircle :size="20" /></div>
          <p>解析失败</p>
          <strong>{{ formatNumber(logQueryMode === LOG_QUERY_TYPE_XRAY ? selectedXrayAnalysisResult?.failedFiles || 0 : selectedLogQueryResult?.failedFiles || 0) }}</strong>
        </article>
      </section>

      <article v-if="selectedLogFeatureHasResult" class="panel wide admin-log-results">
        <div class="panel-title">
          <h2>{{ logQueryMode === LOG_QUERY_TYPE_XRAY ? '分析结果' : '查询结果' }}</h2>
          <span>{{ selectedLogFeatureResult?.message || (logQueryMode === LOG_QUERY_TYPE_XRAY ? '还没有分析记录' : '还没有查询记录') }}</span>
        </div>
        <p v-if="selectedLogQueryRunning" class="settings-hint">
          {{ selectedLogQueryCancelling ? '正在停止任务并释放资源…' : `正在扫描 ${selectedLogFeatureResult?.currentFile || 'CSV 文件'}，关掉页面也可以稍后回来查看结果。` }}
        </p>
        <template v-if="logQueryMode === LOG_QUERY_TYPE_XRAY">
          <div class="table-toolbar">
            <span>
              疑似玩家 {{ formatNumber(selectedXrayAnalysisResult?.playerCount || 0) }} 名，风险项 {{ formatNumber(selectedXrayAnalysisResult?.findingCount || 0) }} 个，最高 {{ formatNumber(selectedXrayAnalysisResult?.maxRiskScore || 0) }} 分
            </span>
          </div>
          <div class="table-wrap">
            <table class="xray-table">
              <thead>
                <tr>
                  <th>玩家</th>
                  <th>疑似风险</th>
                  <th>周期稀有矿</th>
                  <th>周期十分钟峰值</th>
                  <th>追矿证据</th>
                  <th>操作</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="player in (selectedXrayAnalysisResult?.players || [])" :key="player.playerName">
                  <td>{{ player.playerName }}</td>
                  <td>
                    <span class="risk-pill" :class="xrayRiskClass(player.riskScore)">
                      {{ player.riskScore }} / 100
                    </span>
                    <small class="risk-level">{{ player.riskLevel }}</small>
                  </td>
                  <td>
                    <div class="ore-stack compact">
                      <strong>{{ formatNumber(xrayAnalysisValue(player, 'analysisRareOreBreaks', 'miningSessionRareOreBreaks')) }}</strong>
                    </div>
                  </td>
                  <td>
                    <div class="ore-stack compact">
                      <strong>{{ formatNumber(xrayAnalysisValue(player, 'analysisPeakRareOreWindowCount', 'peakRareOreWindowCount')) }}</strong>
                    </div>
                  </td>
                  <td>{{ formatNumber(xrayAnalysisTrackingEvidenceCount(player)) }}</td>
                  <td>
                    <button class="secondary-button compact-button" type="button" @click="openXrayPlayerDetail(player)">
                      <Eye :size="16" />
                      <span>查看详细</span>
                    </button>
                  </td>
                </tr>
                <tr v-if="!(selectedXrayAnalysisResult?.players || []).length">
                  <td colspan="6" class="empty">
                    {{ selectedLogQueryRunning ? '正在分析，请稍等' : (selectedXrayAnalysisResult?.message || '还没有分析记录') }}
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </template>
        <template v-else>
        <div class="table-toolbar">
          <span>
            第 {{ selectedLogQueryPage }} / {{ selectedLogQueryTotalPages || 1 }} 页，本页 {{ formatNumber(selectedLogQueryRows.length) }} 条，共 {{ formatNumber(selectedLogQueryResult?.matchedRows || 0) }} 条
          </span>
          <div class="pagination-controls">
            <AdminPagination :page="selectedLogQueryPage" :total-pages="selectedLogQueryTotalPages || 1" :page-size="selectedLogQueryPageSize" :sort-order="selectedLogQuerySortOrder" @order="setLogQuerySortOrder" :loading="selectedLogQueryLoading" @page="setLogQueryPage" @page-size="setLogQueryPageSize" />
          </div>
        </div>
        <div class="table-wrap log-query-table-wrap">
          <table class="log-query-table">
            <colgroup>
              <col style="width: 14%" /><col style="width: 5%" /><col style="width: 14%" /><col style="width: 11%" />
              <col style="width: 12%" /><col style="width: 14%" /><col style="width: 14%" /><col style="width: 16%" />
            </colgroup>
            <thead>
              <tr>
                <th>文件</th>
                <th>行号</th>
                <th>时间</th>
                <th>玩家</th>
                <th>行为</th>
                <th>玩家坐标</th>
                <th>交互坐标</th>
                <th>详情</th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="row in selectedLogQueryRows"
                :key="`${row.filePath}:${row.lineNumber}`"
                class="clickable-row"
                tabindex="0"
                @click="openLogQueryRowDetail(row)"
                @keydown.enter.prevent="openLogQueryRowDetail(row)"
                @keydown.space.prevent="openLogQueryRowDetail(row)"
              >
                <td class="path-cell" :title="row.filePath">{{ row.fileName }}</td>
                <td>{{ row.lineNumber }}</td>
                <td>{{ row.date }} {{ row.time }}</td>
                <td>{{ row.playerName }}</td>
                <td>{{ logQueryActionDisplay(row.action) }}</td>
                <td>{{ logQueryCoordinateText(row.x, row.y, row.z, row.dimension) }}</td>
                <td>{{ logQueryInteractionCoordinateText(row) }}</td>
                <td class="message-cell">{{ logQuerySummaryDetailText(row) }}</td>
              </tr>
              <tr v-if="!selectedLogQueryRows.length">
                <td colspan="8" class="empty">
                  {{ selectedLogQueryRunning ? '正在查询，请稍等' : (selectedLogQueryResult?.message || '还没有查询记录') }}
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <div class="log-query-card-list">
          <button
            v-for="row in selectedLogQueryRows"
            :key="`${row.filePath}:${row.lineNumber}:card`"
            class="log-query-card"
            type="button"
            @click="openLogQueryRowDetail(row)"
          >
            <span class="log-query-card-head">
              <strong>{{ row.date }} {{ row.time }}</strong>
              <span>{{ logQueryActionDisplay(row.action) }}</span>
            </span>
            <span class="log-query-card-player">{{ row.playerName || '-' }}</span>
            <span class="log-query-card-coord">交互：{{ logQueryInteractionCoordinateText(row) }}</span>
            <span class="log-query-card-detail">{{ logQuerySummaryDetailText(row) }}</span>
          </button>
          <p v-if="!selectedLogQueryRows.length" class="empty log-query-card-empty">
            {{ selectedLogQueryRunning ? '正在查询，请稍等' : (selectedLogQueryResult?.message || '还没有查询记录') }}
          </p>
        </div>
        <div class="table-toolbar bottom-toolbar">
          <span>
            第 {{ selectedLogQueryPage }} / {{ selectedLogQueryTotalPages || 1 }} 页，本页 {{ formatNumber(selectedLogQueryRows.length) }} 条，共 {{ formatNumber(selectedLogQueryResult?.matchedRows || 0) }} 条
          </span>
          <div class="pagination-controls">
            <AdminPagination :page="selectedLogQueryPage" :total-pages="selectedLogQueryTotalPages || 1" :page-size="selectedLogQueryPageSize" :sort-order="selectedLogQuerySortOrder" @order="setLogQuerySortOrder" :loading="selectedLogQueryLoading" @page="setLogQueryPage" @page-size="setLogQueryPageSize" />
            <button class="secondary-button" type="button" @click="scrollLogQueryToTop">
              <ArrowUp :size="18" />
              <span>返回顶部</span>
            </button>
          </div>
        </div>
        </template>
      </article>

      <div v-if="selectedLogQueryRow" class="modal-backdrop log-detail-backdrop" @click.self="closeLogQueryRowDetail">
        <article class="modal-panel log-detail-panel">
          <div class="modal-title">
            <div>
              <p class="eyebrow">日志详情</p>
              <h2>{{ selectedLogQueryRow.action || '当前行' }}</h2>
            </div>
            <button class="icon-button ghost" type="button" title="关闭" @click="closeLogQueryRowDetail">
              <XCircle :size="18" />
            </button>
          </div>

          <div class="log-detail-grid">
            <div v-for="field in logQueryRowDetailFields()" :key="field.label" class="log-detail-field">
              <span>{{ field.label }}</span>
              <strong>{{ field.value }}</strong>
            </div>
          </div>

          <section class="log-detail-block">
            <h3>详细1</h3>
            <pre>{{ logQueryDetailAuditText(selectedLogQueryRow, 'detail1') }}</pre>
          </section>
          <section class="log-detail-block">
            <h3>详细2</h3>
            <pre>{{ logQueryDetailAuditText(selectedLogQueryRow, 'detail2') }}</pre>
          </section>

          <div class="modal-actions">
            <button class="secondary-button" type="button" @click="copyLogQueryRowDetail">
              <Copy :size="18" />
              <span>复制详情</span>
            </button>
            <button class="primary-button" type="button" @click="closeLogQueryRowDetail">关闭</button>
          </div>
        </article>
      </div>
    </section>

    <section v-else-if="currentView === 'xrayDetailPage'" class="result-page xray-detail-page">
      <div class="topbar">
        <div>
          <p class="eyebrow">矿透分析详情</p>
          <h1>{{ selectedXrayDetailPlayer?.playerName || xrayDetailPlayerName || '玩家详情' }}</h1>
          <p class="muted-line">{{ serverNameById(logQueryServerId) }}</p>
          <p class="muted-line">统计日期：{{ xrayAnalysisDateRange(xrayDetailResult) }}</p>
        </div>
        <div class="actions">
          <button class="primary-button" type="button" :disabled="!selectedXrayDetailPlayer" @click="openXrayGroupSendDialog">
            <Send :size="18" />
            <span>发送到Q群</span>
          </button>
          <button class="secondary-button" type="button" @click="backToXrayAnalysis">
            <ArrowLeft :size="18" />
            <span>返回矿透分析</span>
          </button>
        </div>
      </div>

      <article v-if="selectedLogQueryLoading" class="panel wide">
        <p class="settings-hint">正在读取矿透分析详情...</p>
      </article>

      <article v-else-if="!selectedXrayDetailPlayer" class="panel wide">
        <div class="panel-title">
          <h2>没有找到这个玩家</h2>
          <span>{{ xrayDetailPlayerName || '-' }}</span>
        </div>
        <p class="settings-hint">可能还没有分析结果，或者最新结果里没有这个玩家。返回矿透分析重新运行一次即可。</p>
      </article>

      <template v-else>
        <article class="panel wide">
          <div class="xray-detail-header">
            <div>
              <h3>{{ selectedXrayDetailPlayer.playerName }}</h3>
              <span>{{ selectedXrayDetailPlayer.riskScore }} / 100 · {{ selectedXrayDetailPlayer.riskLevel }}</span>
            </div>
            <span class="risk-pill" :class="xrayRiskClass(selectedXrayDetailPlayer.riskScore)">
              {{ selectedXrayDetailPlayer.riskScore }} / 100
            </span>
          </div>

          <div class="xray-detail-body">
            <section class="xray-detail-section xray-toggle-section">
              <h4>筛选周期总量</h4>
              <div class="xray-detail-grid">
                <div>
                  <span>破坏方块</span>
                  <strong>{{ formatNumber(xrayAnalysisValue(selectedXrayDetailPlayer, 'analysisBreaks', 'miningSessionBreaks')) }}</strong>
                </div>
                <div>
                  <span>地下破坏</span>
                  <strong>{{ formatNumber(xrayAnalysisValue(selectedXrayDetailPlayer, 'analysisUndergroundBreaks', 'miningSessionUndergroundBreaks')) }}</strong>
                </div>
                <div>
                  <span>稀有矿</span>
                  <strong>{{ formatNumber(xrayAnalysisValue(selectedXrayDetailPlayer, 'analysisRareOreBreaks', 'miningSessionRareOreBreaks')) }}</strong>
                </div>
                <div>
                  <span>钻石矿</span>
                  <strong>{{ formatNumber(xrayAnalysisValue(selectedXrayDetailPlayer, 'analysisDiamondOreBreaks', 'miningSessionDiamondOreBreaks')) }}</strong>
                </div>
                <div>
                  <span>远古残骸</span>
                  <strong>{{ formatNumber(xrayAnalysisValue(selectedXrayDetailPlayer, 'analysisAncientDebrisBreaks', 'miningSessionAncientDebrisBreaks')) }}</strong>
                </div>
                <div>
                  <span>地下稀有矿占比</span>
                  <strong>{{ xrayAnalysisRareRatioText(selectedXrayDetailPlayer) }}</strong>
                </div>
                <div>
                  <span>十分钟挖取峰值</span>
                  <strong>{{ formatNumber(xrayAnalysisValue(selectedXrayDetailPlayer, 'analysisPeakRareOreWindowCount', 'peakRareOreWindowCount')) }}</strong>
                </div>
                <div>
                  <span>周期追矿证据</span>
                  <strong>{{ formatNumber(xrayAnalysisTrackingEvidenceCount(selectedXrayDetailPlayer)) }}</strong>
                </div>
              </div>
            </section>

            <section class="xray-detail-section xray-toggle-section">
              <h4>最可疑挖矿会话</h4>
              <div class="xray-detail-grid">
                <div>
                  <span>会话时间</span>
                  <strong>{{ xrayMiningSessionTime(selectedXrayDetailPlayer) }}</strong>
                </div>
                <div>
                  <span>会话时长</span>
                  <strong>{{ xrayMiningSessionDuration(selectedXrayDetailPlayer) }}</strong>
                </div>
                <div>
                  <span>会话地下破坏</span>
                  <strong>{{ formatNumber(selectedXrayDetailPlayer.miningSessionUndergroundBreaks || selectedXrayDetailPlayer.miningSessionBreaks || 0) }}</strong>
                </div>
                <div>
                  <span>地下稀有矿占比</span>
                  <strong>{{ xrayRareRatioText(selectedXrayDetailPlayer) }}</strong>
                </div>
              </div>
            </section>

            <section class="xray-detail-section xray-toggle-section">
              <h4>最可疑挖矿会话概况</h4>
              <div class="xray-detail-grid">
                <div>
                  <span>稀有矿</span>
                  <strong>{{ formatNumber(selectedXrayDetailPlayer.miningSessionRareOreBreaks) }}</strong>
                </div>
                <div>
                  <span>钻石矿</span>
                  <strong>{{ formatNumber(selectedXrayDetailPlayer.miningSessionDiamondOreBreaks) }}</strong>
                </div>
                <div>
                  <span>远古残骸</span>
                  <strong>{{ formatNumber(selectedXrayDetailPlayer.miningSessionAncientDebrisBreaks) }}</strong>
                </div>
                <div>
                  <span>会话追矿证据</span>
                  <strong>{{ formatNumber(selectedXrayDetailPlayer.trackingEvidenceCount) }}</strong>
                </div>
                <div>
                  <span>十分钟挖取峰值</span>
                  <strong>{{ formatNumber(selectedXrayDetailPlayer.peakRareOreWindowCount || 0) }}</strong>
                </div>
                <div>
                  <span>短时窗口</span>
                  <strong v-if="selectedXrayDetailPlayer.peakRareOreWindowStart && selectedXrayDetailPlayer.peakRareOreWindowEnd">
                    {{ formatDateTime(selectedXrayDetailPlayer.peakRareOreWindowStart) }} - {{ formatDateTime(selectedXrayDetailPlayer.peakRareOreWindowEnd) }}
                  </strong>
                  <strong v-else>-</strong>
                </div>
              </div>
            </section>

            <section class="xray-detail-section xray-toggle-section">
              <h4>原因</h4>
              <div class="xray-chip-list">
                <span v-for="reason in selectedXrayDetailPlayer.reasons" :key="reason">{{ reason }}</span>
                <span v-if="!selectedXrayDetailPlayer.reasons?.length">-</span>
              </div>
            </section>

            <section class="xray-detail-section">
              <h4>筛选周期矿物明细</h4>
              <div class="xray-chip-list">
                <span v-for="ore in xrayAnalysisOreCounts(selectedXrayDetailPlayer)" :key="ore.oreType">
                  {{ ore.displayName }} x{{ formatNumber(ore.count) }}
                </span>
                <span v-if="!xrayAnalysisOreCounts(selectedXrayDetailPlayer).length">-</span>
              </div>
            </section>

            <section class="xray-detail-section">
              <h4>筛选周期证据</h4>
              <div class="xray-expand-row">
                <button
                  v-if="selectedXrayDetailPlayer.evidence?.length"
                  class="secondary-button mini-button"
                  type="button"
                  @click="xrayDetailEvidenceExpanded = !xrayDetailEvidenceExpanded"
                >
                  <span>{{ xrayDetailEvidenceExpanded ? '收起' : `展开 ${selectedXrayDetailPlayer.evidence.length} 条` }}</span>
                </button>
              </div>
              <div v-if="selectedXrayDetailPlayer.evidence?.length && xrayDetailEvidenceExpanded" class="xray-evidence-list">
                <div v-for="(evidence, evidenceIndex) in selectedXrayDetailPlayer.evidence" :key="`${selectedXrayDetailPlayer.playerName}:${evidence.startedAt}:${evidence.summary}`" class="evidence-block">
                  <strong>{{ evidence.summary }}</strong>
                  <small>{{ xrayEvidenceTime(evidence) }} · +{{ evidence.score }}</small>
                  <button
                    v-if="evidence.rows?.length"
                    class="secondary-button mini-button"
                    type="button"
                    @click="toggleDetailXrayEvidenceRows(evidence, evidenceIndex)"
                  >
                    <span>{{ isDetailXrayEvidenceRowsExpanded(evidence, evidenceIndex) ? '收起明细' : `展开明细 ${evidence.rows.length} 条` }}</span>
                  </button>
                  <div v-if="evidence.rows?.length && isDetailXrayEvidenceRowsExpanded(evidence, evidenceIndex)" class="evidence-lines">
                    <span v-for="row in evidence.rows" :key="`${row.filePath}:${row.lineNumber}`">
                      {{ evidenceRowText(row) }}
                    </span>
                  </div>
                </div>
              </div>
              <p v-else-if="!selectedXrayDetailPlayer.evidence?.length" class="settings-hint">没有追矿证据。</p>
            </section>

            <section id="xray-detail-rare-ore-positions" class="xray-detail-section">
              <h4>稀有矿位置</h4>
              <div class="xray-expand-row">
                <button
                  v-if="selectedXrayDetailPlayer.rareOreRows?.length"
                  class="secondary-button mini-button"
                  type="button"
                  @click="toggleXrayDetailOrePositions"
                >
                  <span>{{ xrayDetailOrePositionsExpanded ? '收起' : `展开 ${xrayDetailRareOreRows.length} 条` }}</span>
                </button>
              </div>
              <div v-if="selectedXrayDetailPlayer.rareOreRows?.length && xrayDetailOrePositionsExpanded" class="table-wrap">
                <div class="table-toolbar">
                  <span>
                    第 {{ clampPage(xrayDetailOrePositionPage, xrayDetailOrePositionTotalPages) }} / {{ xrayDetailOrePositionTotalPages }} 页，本页 {{ formatNumber(xrayDetailPagedRareOreRows.length) }} 条，共 {{ formatNumber(xrayDetailRareOreRows.length) }} 条
                  </span>
                  <div class="pagination-controls">
                    <AdminPagination :page="clampPage(xrayDetailOrePositionPage, xrayDetailOrePositionTotalPages)" :total-pages="xrayDetailOrePositionTotalPages" :page-size="xrayDetailOrePositionPageSize" :sort-order="xrayDetailSortOrder" @order="setXrayDetailSortOrder" @page="setXrayDetailOrePositionPage" @page-size="setXrayDetailOrePositionPageSize" />
                  </div>
                </div>
                <table class="xray-ore-position-table">
                  <thead>
                    <tr>
                      <th>顺序</th>
                      <th>时间</th>
                      <th>矿物</th>
                      <th>玩家坐标</th>
                      <th>交互坐标</th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr v-for="(row, index) in xrayDetailPagedRareOreRows" :key="`${row.filePath}:${row.lineNumber}:rare`">
                      <td>{{ xrayDetailOrePositionOffset + index + 1 }}</td>
                      <td>{{ row.date }} {{ row.time }}</td>
                      <td class="message-cell">{{ logQuerySummaryDetailText(row) }}</td>
                      <td>{{ logQueryCoordinateText(row.x, row.y, row.z, row.dimension) }}</td>
                      <td>{{ logQueryInteractionCoordinateText(row) }}</td>
                    </tr>
                  </tbody>
                </table>
                <div class="table-toolbar bottom-toolbar">
                  <span>
                    第 {{ clampPage(xrayDetailOrePositionPage, xrayDetailOrePositionTotalPages) }} / {{ xrayDetailOrePositionTotalPages }} 页，本页 {{ formatNumber(xrayDetailPagedRareOreRows.length) }} 条，共 {{ formatNumber(xrayDetailRareOreRows.length) }} 条
                  </span>
                  <div class="pagination-controls">
                    <AdminPagination :page="clampPage(xrayDetailOrePositionPage, xrayDetailOrePositionTotalPages)" :total-pages="xrayDetailOrePositionTotalPages" :page-size="xrayDetailOrePositionPageSize" :sort-order="xrayDetailSortOrder" @order="setXrayDetailSortOrder" @page="setXrayDetailOrePositionPage" @page-size="setXrayDetailOrePositionPageSize" />
                    <button class="secondary-button" type="button" @click="scrollToXrayDetailOrePositions">
                      <ArrowUp :size="18" />
                      <span>返回稀有矿位置</span>
                    </button>
                  </div>
                </div>
              </div>
              <p v-else-if="!selectedXrayDetailPlayer.rareOreRows?.length" class="settings-hint">没有稀有矿位置记录。</p>
            </section>
          </div>
        </article>
      </template>

      <div v-if="xrayGroupSendDialogOpen" class="modal-backdrop" @click.self="closeXrayGroupSendDialog">
        <form class="modal-panel" @submit.prevent="sendXrayDetailToGroup">
          <div class="modal-title">
            <div>
              <p class="eyebrow">AstrBot</p>
              <h2>发送到Q群</h2>
            </div>
            <button class="icon-button ghost" type="button" title="关闭" @click="closeXrayGroupSendDialog">
              <XCircle :size="18" />
            </button>
          </div>

          <label>
            <span>链接有效时间</span>
            <input v-model.trim="xrayGroupSendTtlText" type="text" placeholder="留空默认 1天，可填 6小时/30分钟/2天" />
          </label>
          <p class="settings-hint">发送内容使用 AstrBot 插件的 share_base_url 生成详情链接；只填数字时按天计算。</p>
          <p v-if="xrayGroupSendError" class="notice error">{{ xrayGroupSendError }}</p>
          <p v-if="xrayGroupSendMessage" class="notice success">{{ xrayGroupSendMessage }}</p>

          <div class="modal-actions">
            <button class="secondary-button" type="button" @click="closeXrayGroupSendDialog">取消</button>
            <button class="primary-button" type="submit" :disabled="xrayGroupSendLoading">
              <Send :size="18" />
              <span>{{ xrayGroupSendLoading ? '提交中...' : '发送到Q群' }}</span>
            </button>
          </div>
        </form>
      </div>
    </section>

    <template v-else>
    <div class="admin-dashboard-controls">
    <section class="topbar">
      <div class="actions">
        <button class="icon-button ghost" type="button" title="刷新" :disabled="loading" @click="loadAll">
          <RefreshCw :size="18" :class="{ spin: loading }" />
        </button>
      </div>
    </section>

    <section class="server-tabs" aria-label="服务器">
      <button
        v-for="server in serverTabs"
        :key="server.serverId"
        type="button"
        :class="{ active: filters.serverId === server.serverId }"
        @click="selectServer(server.serverId)"
      >
        <Layers :size="16" />
        <span>{{ server.serverName }}</span>
        <small v-if="server.serverId !== 'all'">{{ formatNumber(sourceStat(server.serverId, 'totalCount')) }}</small>
      </button>
    </section>
    </div>

    <p v-if="error" class="notice error">{{ error }}</p>
    <p v-if="importMessage" class="notice success">{{ importMessage }}</p>

    <section class="metric-grid">
      <article class="metric-card">
        <div class="metric-icon red"><Hammer :size="20" /></div>
        <p>破坏方块</p>
        <strong>{{ formatNumber(overview.brokenCount) }}</strong>
      </article>
      <article class="metric-card">
        <div class="metric-icon green"><Blocks :size="20" /></div>
        <p>放置方块</p>
        <strong>{{ formatNumber(overview.placedCount) }}</strong>
      </article>
      <article class="metric-card">
        <div class="metric-icon blue"><UsersRound :size="20" /></div>
        <p>玩家数量</p>
        <strong>{{ formatNumber(overview.playerCount) }}</strong>
      </article>
      <article class="metric-card">
        <div class="metric-icon amber"><Database :size="20" /></div>
        <p>导入文件</p>
        <strong>{{ formatNumber(overview.importedFileCount) }}</strong>
        <small v-if="overview.lastImportedAt">{{ formatDateTime(overview.lastImportedAt) }}</small>
      </article>
    </section>

    <section class="content-grid">
      <article class="panel wide">
        <div class="panel-title">
          <h2>玩家排行</h2>
          <span>{{ selectedServerName }} · {{ formatNumber(overview.totalCount) }} 次方块行为</span>
        </div>
        <div class="table-wrap">
          <table>
            <thead>
              <tr>
                <th>玩家</th>
                <th>破坏</th>
                <th>放置</th>
                <th>合计</th>
                <th>首次记录</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="player in players" :key="player.playerName">
                <td>{{ player.playerName }}</td>
                <td>{{ formatNumber(player.brokenCount) }}</td>
                <td>{{ formatNumber(player.placedCount) }}</td>
                <td>{{ formatNumber(player.totalCount) }}</td>
                <td>{{ formatDateTime(player.firstSeenAt) }}</td>
              </tr>
              <tr v-if="!players.length">
                <td colspan="5" class="empty">暂无数据</td>
              </tr>
            </tbody>
          </table>
        </div>
      </article>

      <article class="panel">
        <div class="panel-title">
          <h2>活跃玩家</h2>
          <span>Top {{ leadingPlayers.length }}</span>
        </div>
        <div class="rank-list">
          <div v-for="player in leadingPlayers" :key="player.playerName" class="rank-row">
            <span>{{ player.playerName }}</span>
            <div class="rank-bar">
              <i :style="{ width: `${Math.max((player.totalCount / Math.max(overview.totalCount, 1)) * 100, 3)}%` }"></i>
            </div>
            <strong>{{ formatNumber(player.totalCount) }}</strong>
          </div>
          <p v-if="!leadingPlayers.length" class="empty">暂无数据</p>
        </div>
      </article>

      <article class="panel wide">
        <div class="panel-title">
          <h2>每日走势</h2>
          <span>{{ daily.length }} 天</span>
        </div>
        <div class="trend">
          <div v-for="day in daily" :key="day.statDate" class="trend-row">
            <time>{{ day.statDate.slice(5) }}</time>
            <div class="trend-track">
              <i class="broken" :style="{ width: `${(day.brokenCount / maxDailyTotal) * 100}%` }"></i>
              <i class="placed" :style="{ width: `${(day.placedCount / maxDailyTotal) * 100}%` }"></i>
            </div>
            <strong>{{ formatNumber(day.totalCount) }}</strong>
          </div>
          <p v-if="!daily.length" class="empty">暂无数据</p>
        </div>
      </article>

      <article class="panel">
        <div class="panel-title">
          <h2>最近导入</h2>
          <span>{{ imports.length }} 个文件</span>
        </div>
        <div class="import-list">
          <div v-for="file in imports" :key="`${file.serverId}:${file.remotePath}`" class="import-row">
            <div>
              <strong>{{ file.fileName }}</strong>
              <small>{{ file.serverName }} · {{ file.logDate || '-' }} · {{ fileSize(file.fileSize) }}</small>
            </div>
            <span>{{ formatNumber(file.rowCount - file.ignoredCount) }}</span>
          </div>
          <p v-if="!imports.length" class="empty">暂无数据</p>
        </div>
      </article>
    </section>
    </template>
  </main>
  </AdminLayout>

  <div
    v-if="minecraftLaunchNoticeOpen"
    class="minecraft-launch-backdrop"
    @click.self="minecraftLaunchNoticeOpen = false"
  >
    <section class="minecraft-launch-dialog" role="dialog" aria-modal="true" aria-labelledby="minecraft-launch-title">
      <div>
        <h2 id="minecraft-launch-title">启动游戏中...</h2>
        <p>将为您自动添加服务器，请稍候...</p>
      </div>
      <div class="minecraft-launch-help">
        <strong>若游戏未弹出，请检查：</strong>
        <ul>
          <li><b>a.</b><span>浏览器是否阻止了本次启动</span></li>
          <li><b>b.</b><span>设备是否已安装 Minecraft</span></li>
        </ul>
      </div>
      <button type="button" @click="minecraftLaunchNoticeOpen = false">知道了</button>
    </section>
  </div>
</template>
