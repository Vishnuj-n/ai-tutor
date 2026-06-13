import { ref, computed } from 'vue'
import {
  getNotebookTopicTree,
  getReaderTopicBundle,
  initializeReadingSession,
} from '../services/appApi'

/**
 * useReaderBase - Extracted base reader logic for task-flow-only reading sessions.
 * Handles: notebook/topic loading, page navigation, session initialization.
 * Does NOT handle: AI chat (see useChat.js), manual reading flow (deprecated).
 */
export function useReaderBase(taskID) {
  // State
  const notebookTree = ref([])
  const selectedNotebookID = ref('')
  const selectedTopicID = ref('')
  const loadingTree = ref(false)
  const loadingBundle = ref(false)
  const globalError = ref('')

  // Document state
  const topicTitle = ref('Reader')
  const notebookUrl = ref('')
  const fileType = ref('')
  const pageCount = ref(1)
  const currentPage = ref(1)
  const topicStartPage = ref(0)
  const topicEndPage = ref(0)
  const sections = ref([])
  const activeSection = ref(null)

  // Navigation bounds for task-flow sessions.
  const navigationMinPage = ref(0)
  const navigationMaxPage = ref(0)
  const navigationState = ref(null)

  // Computed
  const selectedNotebook = computed(
    () => notebookTree.value.find((n) => n.notebook_id === selectedNotebookID.value) || null
  )

  const selectedNotebookTitle = computed(() => selectedNotebook.value?.title || '')

  const availableTopics = computed(() => {
    const topics = selectedNotebook.value?.topics || []
    return [...topics].sort((a, b) => {
      const aNum = extractChapterNumber(a.title)
      const bNum = extractChapterNumber(b.title)
      if (aNum !== null || bNum !== null) {
        if (aNum !== null && bNum !== null) {
          if (aNum !== bNum) return aNum - bNum
        } else if (aNum !== null) return -1
        else if (bNum !== null) return 1
      }
      return a.title.localeCompare(b.title, undefined, { numeric: true, sensitivity: 'base' })
    })
  })

  const selectedTopicTitle = computed(() => {
    const match = availableTopics.value.find((t) => t.topic_id === selectedTopicID.value)
    return match?.title || ''
  })

  const pdfVisible = computed(() => fileType.value === 'pdf' && notebookUrl.value !== '')

  const hasNavigationBounds = computed(
    () => navigationMinPage.value > 0 && navigationMaxPage.value >= navigationMinPage.value
  )

  const canGoPrev = computed(() => {
    if (!pdfVisible.value) return false
    const effectiveMin = hasNavigationBounds.value ? navigationMinPage.value : 1
    return currentPage.value > effectiveMin
  })

  const canGoNext = computed(() => {
    if (!pdfVisible.value) return false
    const effectiveMax = hasNavigationBounds.value ? navigationMaxPage.value : pageCount.value
    return currentPage.value < effectiveMax
  })

  const pdfSource = computed(() => {
    if (!notebookUrl.value) return ''
    return `${notebookUrl.value}#page=${currentPage.value}&zoom=page-fit`
  })

  // Methods
  function extractChapterNumber(title) {
    const matches = /^chapter\s*(\d+)\b/i.exec(String(title).trim())
    if (!matches) return null
    const num = Number(matches[1])
    return Number.isFinite(num) ? num : null
  }

  async function loadNotebookTree() {
    loadingTree.value = true
    globalError.value = ''
    try {
      const data = await getNotebookTopicTree()
      notebookTree.value = Array.isArray(data) ? data : []
    } catch (err) {
      console.error('[useReaderBase] loadNotebookTree error:', err)
      globalError.value = err?.message || 'Failed to load notebook/topic options'
    } finally {
      loadingTree.value = false
    }
  }

  async function initializeSession(query = {}) {
    if (!taskID.value) {
      globalError.value = 'Task ID required for reading session'
      return null
    }

    loadingBundle.value = true
    globalError.value = ''

    try {
      // Extract parameters with explicit null/undefined checks (0 is a valid page number)
      const notebookId = query.notebookId ?? query.notebook_id ?? ''
      const topicId = query.topicId ?? query.topic_id ?? ''
      const startPage = query.startPage ?? query.start_page ?? 0
      const endPage = query.endPage ?? query.end_page ?? 0

      if (!notebookId || !topicId) {
        globalError.value = 'Missing notebookId or topicId for reading session'
        return null
      }
      // In backend-authoritative session model, 0 means "unspecified / hydrate from DB session state"
      // Reject only: negative values, or invalid ordering when both are specified
      if (startPage < 0 || endPage < 0) {
        console.error('[useReaderBase] Invalid page bounds: negative values not allowed', {
          startPage,
          endPage,
        })
        globalError.value = `Invalid page bounds: negative values not allowed (startPage=${startPage}, endPage=${endPage})`
        return null
      }
      if (startPage > 0 && endPage > 0 && endPage < startPage) {
        console.error(
          '[useReaderBase] Invalid page bounds: endPage must be >= startPage when both specified',
          { startPage, endPage }
        )
        globalError.value = `Invalid page bounds: endPage=${endPage} must be >= startPage=${startPage} when both specified`
        return null
      }

      const result = await initializeReadingSession(
        taskID.value,
        notebookId,
        topicId,
        startPage,
        endPage
      )

      // Defensive: check ok flag first (backend contract)
      if (!result?.ok) {
        globalError.value =
          result?.error || 'Failed to initialize reading session: backend returned not ok'
        return null
      }

      // STRICT VALIDATION: Fail-loud if backend contract is violated on critical fields
      if (!result.task) {
        globalError.value = 'Contract violation: missing task data'
        return null
      }
      if (!result.page_bounds || typeof result.page_bounds !== 'object') {
        globalError.value = 'Contract violation: missing navigation bounds'
        return null
      }
      if (!result.navigation || typeof result.navigation !== 'object') {
        globalError.value = 'Contract violation: missing navigation state'
        return null
      }

      // Treat missing/empty bundle as recoverable
      let bundle = null
         if (result.bundle && typeof result.bundle === 'object') {        bundle = {
          ...result.bundle,
          sections: Array.isArray(result.bundle.sections) ? result.bundle.sections : [],
        }
      }
      globalError.value = '' // clear globalError

      // Apply initialized state
      const task = result.task
      const navigationBounds = result.page_bounds
      const nav = result.navigation

      // Validate task has required fields
      if (!task.notebook_id || !task.topic_id) {
        globalError.value = 'Invalid task data: missing notebook_id or topic_id'
        return null
      }

      selectedNotebookID.value = task.notebook_id
      selectedTopicID.value = task.topic_id

      // Validate bounds have valid values
      const validStart = Number(navigationBounds.start_page) || 1
      const validEnd = Number(navigationBounds.end_page) || validStart
      const validCurrent = Number(navigationBounds.current_page) || validStart

      navigationMinPage.value = validStart
      navigationMaxPage.value = validEnd
      currentPage.value = Math.min(Math.max(validCurrent, validStart), validEnd)
      navigationState.value = nav

      // Apply bundle data safely (bundle might be null/empty)
      topicTitle.value = bundle?.topic_title || task.topic_title || 'Reader'
      notebookUrl.value = bundle?.notebook_url || ''
      fileType.value = (bundle?.file_type || '').toLowerCase()
      pageCount.value = Math.max(1, Number(bundle?.page_count) || 1)
      topicStartPage.value = Number(bundle?.topic_start_page ?? 0)
      topicEndPage.value = Number(bundle?.topic_end_page ?? 0)
      sections.value = bundle?.sections || []
      activeSection.value = sections.value[0] || null

      return {
        task,
        navigationBounds,
        navigation: nav,
        bundle,
      }
    } catch (err) {
      console.error('[useReaderBase] initializeSession error:', err)
      globalError.value = err?.message || 'Failed to initialize reading session'
      return null
    } finally {
      loadingBundle.value = false
    }
  }

  async function loadBundle() {
    if (!selectedTopicID.value) {
      globalError.value = 'Select topic to open Reader.'
      return false
    }

    loadingBundle.value = true
    globalError.value = ''

    try {
      const result = await getReaderTopicBundle(selectedTopicID.value, selectedNotebookID.value)

      if (result?.error) {
        globalError.value = result.error
        return false
      }

      // In task flow, do not clobber state initialized by the task-session navigation bounds.
      if (hasNavigationBounds.value) {
        return true
      }

      topicTitle.value = result?.topic_title || selectedTopicTitle.value || 'Reader'
      notebookUrl.value = result?.notebook_url || ''
      fileType.value = (result?.file_type || '').toLowerCase()
      pageCount.value = Math.max(1, Number(result?.page_count) || 1)
      topicStartPage.value = Number(result?.topic_start_page) || 0
      topicEndPage.value = Number(result?.topic_end_page) || 0
      sections.value = Array.isArray(result?.sections) ? result.sections : []
      activeSection.value = sections.value[0] || null

      // Set page to topic start page (browse mode - no task navigation bounds)
      const topicStart = Number(result?.topic_start_page) || 1
      currentPage.value = topicStart
      return true
    } catch (err) {
      globalError.value = err?.message || 'Failed to load reader data'
      return false
    } finally {
      loadingBundle.value = false
    }
  }

  function goPrev() {
    if (canGoPrev.value) {
      currentPage.value -= 1
      return true
    }
    return false
  }

  function goNext() {
    if (canGoNext.value) {
      currentPage.value += 1
      return true
    }
    return false
  }

  function selectSection(section) {
    activeSection.value = section
    const page = Number(section?.page_num)
    if (Number.isFinite(page) && page > 0) {
      const effectiveMin = hasNavigationBounds.value ? navigationMinPage.value : 1
      const effectiveMax = hasNavigationBounds.value ? navigationMaxPage.value : pageCount.value
      currentPage.value = Math.min(Math.max(effectiveMin, page), effectiveMax)
    }
  }

  function clampPage(page, maxPageCount) {
    const max = Math.max(1, Number(maxPageCount) || 1)
    const normalized = Number(page)
    if (!Number.isFinite(normalized) || normalized <= 0) return 1
    if (normalized > max) return max
    return Math.floor(normalized)
  }

  return {
    // State
    notebookTree,
    selectedNotebookID,
    selectedTopicID,
    loadingTree,
    loadingBundle,
    globalError,
    topicTitle,
    notebookUrl,
    fileType,
    pageCount,
    currentPage,
    topicStartPage,
    topicEndPage,
    sections,
    activeSection,
    navigationMinPage,
    navigationMaxPage,
    navigationState,

    // Computed
    selectedNotebook,
    selectedNotebookTitle,
    availableTopics,
    selectedTopicTitle,
    pdfVisible,
    hasNavigationBounds,
    canGoPrev,
    canGoNext,
    pdfSource,

    // Methods
    loadNotebookTree,
    initializeSession,
    loadBundle,
    goPrev,
    goNext,
    selectSection,
    clampPage,
  }
}
