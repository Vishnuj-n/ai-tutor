import { ref, computed } from 'vue'
import { getNotebookTopicTree, getReaderTopicBundle, initializeReadingSession } from '../services/appApi'

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
  const sections = ref([])
  const activeSection = ref(null)

  // Locked window state (task flow only)
  const lockedStartPage = ref(0)
  const lockedTargetPage = ref(0)

  // Computed
  const selectedNotebook = computed(() =>
    notebookTree.value.find((n) => n.notebook_id === selectedNotebookID.value) || null
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

  const hasLockedWindow = computed(() =>
    lockedStartPage.value > 0 && lockedTargetPage.value >= lockedStartPage.value
  )

  const canGoPrev = computed(() => {
    if (!pdfVisible.value) return false
    if (hasLockedWindow.value) {
      return currentPage.value > Math.max(1, lockedStartPage.value)
    }
    return currentPage.value > 1
  })

  const canGoNext = computed(() => {
    if (!pdfVisible.value) return false
    if (hasLockedWindow.value) {
      return currentPage.value < Math.min(pageCount.value, lockedTargetPage.value)
    }
    return currentPage.value < Math.max(1, pageCount.value)
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

  async function initializeSession() {
    if (!taskID.value) {
      globalError.value = 'Task ID required for reading session'
      return false
    }

    loadingBundle.value = true
    globalError.value = ''

    try {
      const result = await initializeReadingSession(taskID.value)

      if (result?.error) {
        globalError.value = result.error
        return false
      }

      // Apply initialized state
      const task = result.task
      const bounds = result.page_bounds
      const nav = result.navigation
      const bundle = result.bundle

      selectedNotebookID.value = task.notebook_id || ''
      selectedTopicID.value = task.topic_id || ''

      lockedStartPage.value = bounds?.start_page || 1
      lockedTargetPage.value = bounds?.end_page || 1
      currentPage.value = bounds?.current_page || lockedStartPage.value

      // Load bundle data if available
      if (bundle) {
        topicTitle.value = bundle.topic_title || task.topic_title || 'Reader'
        notebookUrl.value = bundle.notebook_url || ''
        fileType.value = (bundle.file_type || '').toLowerCase()
        pageCount.value = Math.max(1, Number(bundle.page_count) || 1)
        sections.value = Array.isArray(bundle.sections) ? bundle.sections : []
        activeSection.value = sections.value[0] || null
      } else {
        // Fallback: load bundle separately
        await loadBundle()
      }

      return {
        task,
        bounds,
        navigation: nav,
        bundle
      }
    } catch (err) {
      console.error('[useReaderBase] initializeSession error:', err)
      globalError.value = err?.message || 'Failed to initialize reading session'
      return false
    } finally {
      loadingBundle.value = false
    }
  }

  async function loadBundle() {
    if (!selectedTopicID.value) {
      globalError.value = 'Select topic to open Reader.'
      return
    }

    loadingBundle.value = true
    globalError.value = ''

    try {
      const result = await getReaderTopicBundle(selectedTopicID.value, selectedNotebookID.value)

      if (result?.error) {
        globalError.value = result.error
        return
      }

      topicTitle.value = result?.topic_title || selectedTopicTitle.value || 'Reader'
      notebookUrl.value = result?.notebook_url || ''
      fileType.value = (result?.file_type || '').toLowerCase()
      pageCount.value = Math.max(1, Number(result?.page_count) || 1)
      sections.value = Array.isArray(result?.sections) ? result.sections : []
      activeSection.value = sections.value[0] || null

      // Set page bounds from topic if not locked
      if (!hasLockedWindow.value) {
        const topicStart = Number(result?.topic_start_page) || 1
        const topicEnd = Number(result?.topic_end_page) || pageCount.value
        lockedStartPage.value = topicStart
        lockedTargetPage.value = Math.max(topicEnd, topicStart)
        currentPage.value = topicStart
      }
    } catch (err) {
      globalError.value = err?.message || 'Failed to load reader data'
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
      currentPage.value = Math.min(Math.max(1, page), pageCount.value)
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
    sections,
    activeSection,
    lockedStartPage,
    lockedTargetPage,

    // Computed
    selectedNotebook,
    selectedNotebookTitle,
    availableTopics,
    selectedTopicTitle,
    pdfVisible,
    hasLockedWindow,
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
    clampPage
  }
}
