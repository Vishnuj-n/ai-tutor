<template>
  <section class="page">
    <header class="head">
      <p class="eyebrow">Reader</p>
      <h1>{{ reader.topicTitle.value }}</h1>
      <p class="meta">
        <span>{{ reader.sections.value.length }} sections</span>
        <span v-if="reader.selectedNotebookTitle.value"
          >Notebook: {{ reader.selectedNotebookTitle.value }}</span
        >
        <span v-if="isTaskFlow" class="task-badge">Task Mode</span>
        <span v-else class="browse-badge">Browse Mode</span>
      </p>
    </header>

    <!-- Browse Mode: Notebook/Topic Selection -->
    <article v-if="!isTaskFlow" class="panel controls">
      <label class="field">
        <span>Notebook</span>
        <select
          v-model="reader.selectedNotebookID.value"
          :disabled="
            reader.loadingTree.value ||
            reader.notebookTree.value.length === 0 ||
            reader.loadingBundle.value
          "
          @change="onNotebookChange()"
        >
          <option disabled value="">Select notebook</option>
          <option
            v-for="notebook in reader.notebookTree.value"
            :key="notebook.notebook_id"
            :value="notebook.notebook_id"
          >
            {{ notebook.title }}
          </option>
        </select>
      </label>

      <label class="field">
        <span>Topic</span>
        <select
          v-model="reader.selectedTopicID.value"
          :disabled="
            reader.loadingTree.value ||
            reader.availableTopics.value.length === 0 ||
            reader.loadingBundle.value
          "
          @change="reader.loadBundle()"
        >
          <option disabled value="">
            {{ reader.availableTopics.value.length === 0 ? 'No topics available' : 'Select topic' }}
          </option>
          <option
            v-for="topic in reader.availableTopics.value"
            :key="topic.topic_id"
            :value="topic.topic_id"
          >
            {{ topic.title }}
          </option>
        </select>
      </label>
    </article>

    <article v-if="reader.globalError.value" class="panel error fatal-error">
      <h3>Reader Initialization Error</h3>
      <p>{{ reader.globalError.value }}</p>
      <div class="error-actions">
        <button class="secondary" @click="router.push('/dashboard')">Back to Dashboard</button>
        <button class="primary" @click="reloadPage()">Retry</button>
      </div>
    </article>

    <div v-else class="layout" :class="{ collapsed: chat.chatCollapsed.value }">
      <article class="panel stage">
        <div class="stage-head">
          <div class="stage-head-left">
            <span class="page-indicator"
              >Page {{ reader.currentPage.value }} / {{ reader.pageCount.value }}</span
            >
            <button
              v-if="isTaskFlow"
              class="primary"
              :disabled="!resolvedTaskID || reader.loadingBundle.value || completingSession"
              @click="completeSession"
            >
              {{ completingSession ? 'Completing Session...' : 'Complete Session' }}
            </button>
          </div>
          <div v-if="isTaskFlow && reader.hasNavigationBounds.value" class="stage-head-right">
            <span class="reading-window-info">
              Reading Window: Pages {{ reader.navigationMinPage.value }}-{{
                reader.navigationMaxPage.value
              }}
            </span>
          </div>
        </div>

        <div v-if="reader.loadingBundle.value" class="empty">Loading document...</div>
        <div v-else-if="!reader.pdfVisible.value" class="empty">
          PDF not available for selected notebook/topic.
        </div>
        <div
          v-else
          ref="pdfViewportRef"
          class="pdf-viewport"
          tabindex="0"
          :data-view-mode="viewMode"
          :style="{
            opacity:
              (scrollState.status !== 'initializing' && scrollState.status !== 'loading') ||
              pdfLoadError
                ? 1
                : 0,
            transition: 'opacity 0.2s ease',
          }"
          @keydown="handleViewportKeydown"
        >
          <div v-if="pdfLoadError" class="empty error">{{ pdfLoadError }}</div>
          <div
            v-else
            ref="pdfScalerRef"
            class="pdf-scaler"
            :style="{ width: `${Math.round(BASE_PAGE_WIDTH * zoomScale)}px`, margin: '0 auto' }"
          >
            <div
              v-for="pageNum in reader.pageCount.value"
              :key="pageNum"
              :data-page="pageNum"
              class="pdf-page-wrapper"
            >
              <vue-pdf-embed
                v-if="renderedPages[pageNum]"
                :source="reader.notebookUrl.value"
                :page="pageNum"
                :text-layer="false"
                :annotation-layer="false"
                @rendered="() => onPageRendered(pageNum)"
                @loading-failed="handlePDFLoadFailed"
                @rendering-failed="handlePDFLoadFailed"
              />
            </div>
          </div>
        </div>

        <!-- Dev-only Scroll Debug Overlay -->
        <pre v-if="isDev" class="debug-overlay">
Scroll Status:  {{ scrollState.status }}
Target Page:    {{ scrollState.targetPage }}
Visible Page:   {{ currentVisiblePage }}
Programmatic:   {{ isProgrammaticScroll }}
        </pre>

        <!-- Right-edge PDF Controls -->
        <div
          v-if="reader.pdfVisible.value && !reader.loadingBundle.value && !pdfLoadError"
          class="pdf-edge-controls"
        >
          <button
            class="edge-btn zoom-btn"
            :disabled="zoomScale <= 0.5"
            title="Zoom out"
            @click="zoomOut"
          >
            −
          </button>
          <span class="edge-zoom-val">{{ Math.round(zoomScale * 100) }}%</span>
          <button
            class="edge-btn zoom-btn"
            :disabled="zoomScale >= 2.5"
            title="Zoom in"
            @click="zoomIn"
          >
            +
          </button>
          <div class="edge-sep"></div>
          <div class="theme-trigger-wrap">
            <button
              class="edge-btn dots-btn"
              title="Change theme"
              :aria-expanded="themeMenuOpen"
              @click="themeMenuOpen = !themeMenuOpen"
            >
              ···
            </button>
            <div v-if="themeMenuOpen" class="theme-flyout" role="menu">
              <button
                v-for="mode in ['raw', 'light', 'dark', 'sync']"
                :key="mode"
                class="flyout-item"
                role="menuitem"
                :class="{ active: viewMode === mode }"
                @click="setViewMode(mode)"
              >
                {{ mode.charAt(0).toUpperCase() + mode.slice(1) }}
              </button>
            </div>
          </div>
        </div>

        <p v-if="isTaskFlow && completionMessage" class="completion-message">
          {{ completionMessage }}
        </p>
        <p v-if="isTaskFlow && completionError" class="error">{{ completionError }}</p>
      </article>

      <ReaderChat
        v-if="ragEnabled && ragQueueStudy"
        :selected-topic-i-d="reader.selectedTopicID.value"
        :selected-topic-title="reader.selectedTopicTitle.value"
        :selected-notebook-i-d="reader.selectedNotebookID.value"
        :selected-notebook-title="reader.selectedNotebookTitle.value"
        :current-page="reader.currentPage.value"
        :topic-start-page="reader.topicStartPage.value"
        :topic-end-page="reader.topicEndPage.value"
        :rag-enabled="ragEnabled"
        :rag-settings-loaded="ragSettingsLoaded"
        :rag-settings-error="ragSettingsError"
        @retry-settings="retryGetUserSettings"
      />
      <div v-else-if="ragEnabled && !ragQueueStudy" class="chat-disabled">
        Chat is currently disabled in queue study mode.
      </div>
    </div>
  </section>
</template>

<script setup>
import { computed, nextTick, onMounted, onUnmounted, provide, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { completeReading, getUserSettings, logFrontendEvent } from '../services/appApi'
import { useReaderBase } from '../composables/useReaderBase'
import { useChat } from '../composables/useChat'
import ReaderChat from '../components/ReaderChat.vue'
import VuePdfEmbed from 'vue-pdf-embed'
import 'vue-pdf-embed/dist/styles/annotationLayer.css'
import 'vue-pdf-embed/dist/styles/textLayer.css'

const route = useRoute()
const router = useRouter()
const isDev = import.meta.env.DEV

// Get task ID from route (task flow only - manual flow deprecated)
const routeTaskID = computed(() => {
  const id = route.query.taskId || route.query.task_id
  return typeof id === 'string' ? id.trim() : ''
})

// Initialize composables
const reader = useReaderBase(routeTaskID)
const chat = useChat()
provide('chat', chat)

// Local state for completion
const completingSession = ref(false)
const completionMessage = ref('')
const completionError = ref('')
const sessionTask = ref(null)
const ragEnabled = ref(false)
const ragQueueStudy = ref(true)
const ragSettingsLoaded = ref(false)
const ragSettingsError = ref(null)

const resolvedTaskID = computed(() => {
  return (
    sessionTask.value?.task_id ||
    sessionTask.value?.id ||
    routeTaskID.value ||
    route.query.taskId ||
    route.query.task_id ||
    ''
  )
})

watch(resolvedTaskID, (next, prev) => {
  console.warn('[READER_STATE] resolvedTaskID changed', { previous: prev, next })
})

// Custom PDF Viewer Refs
const pdfViewportRef = ref(null)
const pdfScalerRef = ref(null)
const pdfLoadError = ref('')

// Custom PDF Viewer Zoom & View Modes
const BASE_PAGE_WIDTH = 800
const zoomScale = ref(1.0)

// scrollState status transitions are managed by setScrollStatus to include safety timeout fallbacks.
const scrollState = ref({
  status: 'initializing', // 'initializing' | 'loading' | 'scrolling' | 'ready'
  targetPage: null,
})
let scrollTimeoutId = null

// Track currently centered page in the viewport
const currentVisiblePage = ref(1)

// Append-only page rendering visibility flags
const renderedPages = ref({})
let intersectionObserver = null

// isProgrammaticScroll: true while a programmatic scrollIntoView is in flight.
// The scroll handler ignores events while this is set to prevent cascade.
let isProgrammaticScroll = false
let scrollDebounceId = null
let programmaticScrollTimeoutId = null

function logScroll(event, data = {}) {
  const payload = {
    status: scrollState.value.status,
    target: scrollState.value.targetPage,
    visible: currentVisiblePage.value,
    isProgrammatic: isProgrammaticScroll,
    ...data,
  }
  console.log(`[SCROLL:${event}]`, payload)
  logFrontendEvent('info', 'ReaderScroll', event, payload)
}

function scrollToPage(page) {
  const wrapper = pdfViewportRef.value?.querySelector(`[data-page="${page}"]`)
  if (wrapper) {
    logScroll('scrollToPage_start', { page })
    isProgrammaticScroll = true
    wrapper.scrollIntoView({ behavior: 'auto', block: 'start' })
    if (programmaticScrollTimeoutId) {
      clearTimeout(programmaticScrollTimeoutId)
    }
    programmaticScrollTimeoutId = setTimeout(() => {
      isProgrammaticScroll = false
      programmaticScrollTimeoutId = null
      logScroll('scrollToPage_completed', { page })
    }, 300)
    return true
  }
  return false
}

function setScrollStatus(status, targetPage = null) {
  logScroll('setScrollStatus', { transitioningTo: status, transitionTarget: targetPage })
  scrollState.value.status = status
  scrollState.value.targetPage = targetPage

  if (targetPage !== null) {
    currentVisiblePage.value = targetPage
  }

  if (scrollTimeoutId) {
    clearTimeout(scrollTimeoutId)
    scrollTimeoutId = null
  }

  // Safety fallback to prevent getting stuck in 'loading' or 'scrolling'.
  // Extended to 10s to handle slow PDF renders. Attempts a scroll before clearing.
  if (status === 'loading' || status === 'scrolling') {
    scrollTimeoutId = setTimeout(() => {
      const stuckTarget = scrollState.value.targetPage
      logScroll('safetyTimeoutFired', { stuckTarget, stuckStatus: scrollState.value.status })
      if (stuckTarget) {
        scrollToPage(stuckTarget)
      }
      scrollState.value.status = 'ready'
      scrollState.value.targetPage = null
    }, 10000)
  }
}

const viewMode = ref('raw')
const themeMenuOpen = ref(false)
const containerWidth = ref(800)

// Virtualization constants removed for native-scroll aspect-ratio pattern

// Synchronize programmatic changes of reader.currentPage back to our refs
watch(
  () => reader.currentPage.value,
  (newVal) => {
    logScroll('watchCurrentPage_triggered', { newVal })
    if (scrollState.value.status !== 'ready') {
      return
    }
    if (newVal !== currentVisiblePage.value) {
      logScroll('watchCurrentPage_programmatic_change', {
        from: currentVisiblePage.value,
        to: newVal,
      })
      setScrollStatus('scrolling', newVal)

      // Attempt scroll immediately in case the page wrapper is already rendered
      nextTick(() => {
        const scrolled = scrollToPage(newVal)
        if (scrolled) {
          logScroll('watchCurrentPage_synchronous_scroll', { page: newVal })
          setTimeout(() => {
            setScrollStatus('ready')
            logScroll('watchCurrentPage_scroll_done')
          }, 150)
        } else {
          logScroll('watchCurrentPage_wait_render', { page: newVal })
        }
      })
    }
  }
)

function setViewMode(mode) {
  viewMode.value = mode
  themeMenuOpen.value = false
}

const isTaskFlow = computed(() => {
  // Once context is settled, read mode from the context object.
  // Fall back to route query during the initialization window (context not yet set).
  const settled = reader.readerContext.value
  if (settled) return settled.mode === 'task'
  return !!routeTaskID.value
})

// Trust-based completion: user decides when reading is complete.
// Page navigation is for UI only and does not gate completion.

function onPageRendered(pageNum) {
  logScroll('onPageRendered', { pageNum })
  if (scrollState.value.status === 'loading' || scrollState.value.status === 'scrolling') {
    const targetPage = scrollState.value.targetPage || reader.currentPage.value
    if (pageNum === targetPage) {
      setTimeout(() => {
        setScrollStatus('ready')
        logScroll('onPageRendered_scroll_complete', { targetPage })
      }, 150)
    }
  }
}

// ─── RAG settings ────────────────────────────────────────────────────────────

async function loadRagSettings() {
  ragSettingsLoaded.value = false
  ragSettingsError.value = null
  try {
    const settings = await getUserSettings()
    ragEnabled.value = settings?.rag_enabled ?? false
    ragQueueStudy.value = settings?.rag_queue_study ?? true
  } catch (err) {
    console.error('Failed to load settings in Reader:', err)
    ragSettingsError.value = err?.message || 'Failed to load settings'
  } finally {
    ragSettingsLoaded.value = true
  }
}

// ─── Entry-path resolvers ─────────────────────────────────────────────────────

async function resolveTaskContext(taskQuery) {
  logScroll('resolveTaskContext_start', { taskQuery })
  setScrollStatus('loading')
  const init = await reader.initializeSession(taskQuery)
  logScroll('resolveTaskContext_initialized', { success: !!init, page: reader.currentPage.value })
  if (init) {
    sessionTask.value = init.task
    const targetPage = reader.currentPage.value
    setScrollStatus('loading', targetPage)
    logScroll('resolveTaskContext_start_scroll', { targetPage })
    await nextTick()
    const scrolled = scrollToPage(targetPage)
    if (scrolled) {
      logScroll('resolveTaskContext_immediate_scroll_success', { targetPage })
      setTimeout(() => setScrollStatus('ready'), 150)
    } else {
      setScrollStatus('ready')
    }
  } else {
    setScrollStatus('ready')
  }
}

async function resolveBrowseContext() {
  console.log('[Reader] Browse mode — resolveBrowseContext')
  await reader.loadNotebookTree()
  setScrollStatus('ready')
}

// ─── Mounted ──────────────────────────────────────────────────────────────────

// Initialize on mount
onMounted(async () => {
  await loadRagSettings()

  logFrontendEvent('info', 'ReaderInit', 'mounted', {
    query: route.query,
    routeTaskID: routeTaskID.value,
  })

  if (routeTaskID.value) {
    const taskQuery = {
      notebookId: route.query.notebookId || route.query.notebook_id,
      topicId: route.query.topicId || route.query.topic_id,
      startPage: parseInt(route.query.startPage || route.query.start_page) || 0,
      endPage: parseInt(route.query.endPage || route.query.end_page) || 0,
    }
    await resolveTaskContext(taskQuery)
  } else {
    await resolveBrowseContext()
  }
})

watch(
  () => reader.notebookUrl.value,
  async (newUrl) => {
    logScroll('watchNotebookUrl_triggered', { newUrl, page: reader.currentPage.value })
    pdfLoadError.value = ''
    if (scrollState.value.status !== 'initializing') {
      const targetPage = reader.currentPage.value
      setScrollStatus('loading', targetPage)
      // Immediately attempt scroll for cached PDFs where @rendered never re-fires
      await nextTick()
      const scrolled = scrollToPage(targetPage)
      if (scrolled) {
        logScroll('watchNotebookUrl_immediate_scroll_success', { targetPage })
        setTimeout(() => {
          setScrollStatus('ready')
        }, 150)
      } else {
        logScroll('watchNotebookUrl_wait_render', { targetPage })
      }
    }
  }
)

function reloadPage() {
  window.location.reload()
}

// ResizeObserver and Gesture Controller Setup
let resizeObserver = null

// Watch viewport ref to set up ResizeObserver and Event Listeners dynamically
watch(pdfViewportRef, (el, oldEl, onCleanup) => {
  if (resizeObserver) {
    resizeObserver.disconnect()
    resizeObserver = null
  }

  let startTouchDist = 0
  let startZoomScale = 1.0

  function handleTouchStart(e) {
    if (e.touches.length === 2) {
      const t1 = e.touches[0]
      const t2 = e.touches[1]
      startTouchDist = Math.sqrt(
        Math.pow(t2.clientX - t1.clientX, 2) + Math.pow(t2.clientY - t1.clientY, 2)
      )
      startZoomScale = zoomScale.value
    }
  }

  function handleTouchMove(e) {
    if (e.touches.length === 2 && startTouchDist > 0) {
      e.preventDefault()

      const t1 = e.touches[0]
      const t2 = e.touches[1]
      const currentTouchDist = Math.sqrt(
        Math.pow(t2.clientX - t1.clientX, 2) + Math.pow(t2.clientY - t1.clientY, 2)
      )
      const delta = currentTouchDist / startTouchDist

      if (delta > 1.05 || delta < 0.95) {
        let newScale = startZoomScale * delta
        newScale = Math.round(newScale * 100) / 100
        zoomScale.value = Math.max(0.5, Math.min(2.5, newScale))
      }
    }
  }

  function handleTouchEnd(e) {
    if (e.touches.length < 2) {
      startTouchDist = 0
    }
  }

  function handleWheel(e) {
    if (e.ctrlKey) {
      e.preventDefault()

      const factor = 1 - e.deltaY * 0.005
      let newScale = zoomScale.value * factor
      newScale = Math.round(newScale * 100) / 100
      zoomScale.value = Math.max(0.5, Math.min(2.5, newScale))
    }
  }

  if (el) {
    containerWidth.value = el.clientWidth || 800

    let initialFitDone = false
    resizeObserver = new ResizeObserver((entries) => {
      for (let entry of entries) {
        containerWidth.value = entry.contentRect.width
        if (!initialFitDone && containerWidth.value > 0) {
          if (containerWidth.value < 800) {
            zoomScale.value = Math.max(0.5, Math.round((containerWidth.value / 800) * 100) / 100)
          }
          initialFitDone = true
        }
      }
    })
    resizeObserver.observe(el)

    el.addEventListener('touchstart', handleTouchStart, { passive: true })
    el.addEventListener('touchmove', handleTouchMove, { passive: false })
    el.addEventListener('touchend', handleTouchEnd, { passive: true })
    el.addEventListener('wheel', handleWheel, { passive: false })
    el.addEventListener('scroll', handleViewportScroll, { passive: true })

    onCleanup(() => {
      if (resizeObserver) {
        resizeObserver.disconnect()
        resizeObserver = null
      }
      el.removeEventListener('touchstart', handleTouchStart)
      el.removeEventListener('touchmove', handleTouchMove)
      el.removeEventListener('touchend', handleTouchEnd)
      el.removeEventListener('wheel', handleWheel)
      el.removeEventListener('scroll', handleViewportScroll)
    })
  }
})

function setupIntersectionObserver(viewportEl) {
  if (intersectionObserver) {
    intersectionObserver.disconnect()
  }

  intersectionObserver = new IntersectionObserver(
    (entries) => {
      entries.forEach((entry) => {
        const page = parseInt(entry.target.dataset.page)
        if (Number.isNaN(page)) return
        if (entry.isIntersecting) {
          renderedPages.value[page] = true
        } else {
          renderedPages.value[page] = false
        }
      })
    },
    {
      root: viewportEl,
      rootMargin: '1000px 0px 1000px 0px', // preload pages 1000px before/after they enter viewport
      threshold: 0.01,
    }
  )

  const wrappers = viewportEl.querySelectorAll('.pdf-page-wrapper')
  wrappers.forEach((w) => intersectionObserver.observe(w))
}

// Watch pageCount, notebookUrl, and the viewport ref to dynamically update the intersection observer target elements
watch(
  [() => reader.pageCount.value, () => reader.notebookUrl.value, pdfViewportRef],
  () => {
    nextTick(() => {
      const el = pdfViewportRef.value
      if (el) {
        setupIntersectionObserver(el)
      }
    })
  },
  { immediate: true }
)

function zoomIn() {
  zoomScale.value = Math.min(2.5, Math.round((zoomScale.value + 0.1) * 100) / 100)
}

function zoomOut() {
  zoomScale.value = Math.max(0.5, Math.round((zoomScale.value - 0.1) * 100) / 100)
}

function handleViewportKeydown(e) {
  if (e.ctrlKey && (e.key === '=' || e.key === '+')) {
    e.preventDefault()
    zoomIn()
  } else if (e.ctrlKey && e.key === '-') {
    e.preventDefault()
    zoomOut()
  }
}

onUnmounted(() => {
  if (resizeObserver) {
    resizeObserver.disconnect()
    resizeObserver = null
  }
  if (intersectionObserver) {
    intersectionObserver.disconnect()
    intersectionObserver = null
  }
  if (scrollDebounceId) clearTimeout(scrollDebounceId)
  if (programmaticScrollTimeoutId) clearTimeout(programmaticScrollTimeoutId)
})

// ─── Scroll-based page tracking ───────────────────────────────────────────────
// Replaces IntersectionObserver. Reads geometry directly on scroll — no
// observer rebuild cycles, no feedback loops, no cascade.

function getVisiblePageFromScroll() {
  const viewport = pdfViewportRef.value
  if (!viewport) return null
  const viewTop = viewport.scrollTop
  const viewBottom = viewTop + viewport.clientHeight
  const wrappers = viewport.querySelectorAll('.pdf-page-wrapper')
  let bestPage = null
  let bestOverlap = 0
  for (const el of wrappers) {
    const elTop = el.offsetTop
    const elBottom = elTop + el.offsetHeight
    const overlap = Math.min(viewBottom, elBottom) - Math.max(viewTop, elTop)
    if (overlap > bestOverlap) {
      bestOverlap = overlap
      bestPage = parseInt(el.dataset.page)
    }
  }
  return bestPage
}

function handleViewportScroll() {
  if (isProgrammaticScroll) return
  if (scrollState.value.status !== 'ready') return
  if (scrollDebounceId) clearTimeout(scrollDebounceId)
  scrollDebounceId = setTimeout(() => {
    const page = getVisiblePageFromScroll()
    if (!page) return
    if (page !== currentVisiblePage.value) {
      currentVisiblePage.value = page
      reader.updateCurrentPage(page)
    }
  }, 80)
}

function handlePDFLoadFailed(err) {
  console.error('[Reader] PDF loading failed:', err)
  const errMsg =
    typeof err === 'string'
      ? err
      : err?.message || (err && JSON.stringify(err)) || 'Failed to load PDF document.'
  pdfLoadError.value = errMsg
  logFrontendEvent('error', 'ReaderPDF', 'pdf_load_failed', { error: errMsg })
  setScrollStatus('ready')
}

async function retryGetUserSettings() {
  await loadRagSettings()
}

function onNotebookChange() {
  // Clear topic selection when notebook changes to prevent stale topic IDs
  reader.selectedTopicID.value = ''
  // Don't call loadBundle() here - let user select a topic first
}

async function completeSession() {
  if (completingSession.value || reader.loadingBundle.value || !resolvedTaskID.value) return

  completionError.value = ''
  completionMessage.value = ''
  completingSession.value = true

  try {
    const taskIDForCompletion = resolvedTaskID.value
    console.warn('[COMPLETE_SESSION] pre-completeReading ids', {
      routeQueryTaskId: route.query.taskId,
      routeQueryTask_id: route.query.task_id,
      routeTaskIDComputed: routeTaskID.value,
      resolvedTaskID: resolvedTaskID.value,
      actualArg: taskIDForCompletion,
    })
    const done = await completeReading(taskIDForCompletion)
    console.warn('[COMPLETE_SESSION] completeSession() completeReading response', done)
    if (done?.error) {
      completionError.value = done.error
      return
    }
    const nextRoute = done?.quiz_task_id ? `/quiz?taskId=${done.quiz_task_id}` : '/dashboard'
    // Completion writes the follow-up quiz into the queue; navigation follows the existing route behavior.
    console.warn('[COMPLETE_SESSION] completeSession() before router.push', {
      nextRoute,
      quizTaskID: done?.quiz_task_id || null,
    })
    await router.push(nextRoute)
    console.warn('[COMPLETE_SESSION] completeSession() router.push resolved', { nextRoute })
  } catch (err) {
    console.error('[COMPLETE_SESSION] completeSession() catch', err)
    completionError.value = err?.message || 'Failed to complete session'
  } finally {
    completingSession.value = false
  }
}
</script>

<style scoped>
.page {
  display: grid;
  gap: 14px;
}

.head {
  display: grid;
  gap: 6px;
}

.eyebrow {
  margin: 0;
  font-size: 11px;
  letter-spacing: 0.14em;
  text-transform: uppercase;
  color: var(--muted-text);
  font-weight: 700;
}

h1 {
  margin: 0;
  font-size: 42px;
  font-family: 'Manrope', sans-serif;
  letter-spacing: -0.02em;
}

h2 {
  margin: 0;
  font-size: 28px;
  font-family: 'Manrope', sans-serif;
}

h3 {
  margin: 0;
  font-size: 18px;
  font-family: 'Manrope', sans-serif;
}

.meta {
  margin: 0;
  display: flex;
  gap: 12px;
  flex-wrap: wrap;
  color: var(--muted-text);
}

.task-badge {
  font-size: 10px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.06em;
  color: var(--primary);
  background: color-mix(in srgb, var(--primary) 12%, var(--surface-container-low));
  padding: 2px 8px;
  border-radius: 4px;
}

.browse-badge {
  font-size: 10px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.06em;
  color: var(--muted-text);
  background: var(--surface-container-low);
  padding: 2px 8px;
  border-radius: 4px;
}

.panel {
  background: var(--surface-container-lowest);
  border: 1px solid var(--surface-container-low);
  border-radius: 14px;
  padding: 12px;
}

.controls {
  display: grid;
  grid-template-columns: repeat(2, minmax(220px, 360px));
  gap: 10px;
}

.layout {
  display: grid;
  grid-template-columns: 1.8fr 1fr;
  gap: 12px;
}

.layout.collapsed {
  grid-template-columns: 1fr 78px;
}

.stage {
  display: grid;
  gap: 10px;
  position: relative;
  min-width: 0;
}

.stage-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 10px;
}

.stage-head-left {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
  color: var(--muted-text);
}

.stage-head-right {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
  color: var(--muted-text);
}

.reading-window-info {
  white-space: nowrap;
}

.page-indicator {
  font-weight: 600;
  white-space: nowrap;
}

.pdf-page-wrapper {
  display: block;
  margin: 0 auto;
  margin-bottom: 20px;
  width: 100%;
  aspect-ratio: 8.5 / 11;
  background: var(--surface-container-lowest, #ffffff);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.08);
  border: 1px solid rgba(0, 0, 0, 0.08);
  border-radius: 4px;
  overflow: hidden;
}

.pdf-viewport {
  width: 100%;
  height: calc(100vh - 160px);
  overflow-y: auto;
  /* ponytail: native browser layout width scaling & overflow-x centers small pages and enables horizontal scroll when zoomed */
  overflow-x: auto;
  background: var(--background);
  border: none !important;
  margin: 0 !important;
  padding: 0 !important;
  border-radius: 10px;
}

.pdf-viewport :deep(.vue-pdf-embed) {
  display: block;
  margin: 0 auto !important;
  padding: 0 !important;
  border: none !important;
  width: 100% !important;
  height: 100% !important;
}

.pdf-viewport :deep(.vue-pdf-embed__page) {
  display: block;
  margin: 0 auto !important;
  padding: 0 !important;
  width: 100% !important;
  height: 100% !important;
  border: none !important;
  box-shadow: none !important;
}

.pdf-viewport :deep(.vue-pdf-embed__page canvas) {
  width: 100% !important;
  height: auto !important;
  display: block !important;
  margin: 0 auto !important;
  padding: 0 !important;
  box-shadow: none !important;
  border: none !important;
  max-width: none !important;
  will-change: filter;
}

/* Theme filters */
[data-theme='light-warm'] .pdf-viewport :deep(.vue-pdf-embed__page canvas) {
  filter: sepia(0.4) contrast(1.05) brightness(0.95);
}

[data-theme='dark-indigo'] .pdf-viewport :deep(.vue-pdf-embed__page canvas) {
  filter: invert(0.9) hue-rotate(190deg) brightness(0.9) contrast(1.1);
}

[data-theme='dark-nord'] .pdf-viewport :deep(.vue-pdf-embed__page canvas) {
  filter: invert(0.9) hue-rotate(160deg) saturate(0.8) brightness(0.9) contrast(1.1);
}

[data-theme='dark-emerald'] .pdf-viewport :deep(.vue-pdf-embed__page canvas) {
  filter: invert(0.9) hue-rotate(90deg) saturate(0.7) brightness(0.9) contrast(1.1);
}

.completion-message {
  margin: 0;
  font-size: 13px;
  color: var(--muted-text);
}

.sections {
  display: grid;
  gap: 8px;
}

.section-list {
  display: grid;
  gap: 6px;
  max-height: 220px;
  overflow: auto;
}

.section-item {
  border: 1px solid var(--surface-container-low);
  background: var(--surface-container-lowest);
  border-radius: 10px;
  padding: 9px;
  text-align: left;
  display: grid;
  gap: 2px;
}

.section-item.active {
  border-color: var(--primary);
  background: color-mix(in srgb, var(--primary) 10%, var(--surface-container-lowest));
}

.section-title {
  font-size: 13px;
  font-weight: 700;
  color: var(--on-surface);
}

.section-page {
  font-size: 12px;
  color: var(--muted-text);
}

.field {
  display: grid;
  gap: 5px;
}

.field span {
  font-size: 12px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.08em;
  color: var(--muted-text);
}

select,
textarea {
  width: 100%;
  border: 1px solid var(--surface-container-low);
  background: var(--surface-container-lowest);
  color: var(--on-surface);
  border-radius: 10px;
  font: inherit;
  padding: 10px;
  outline: 0;
}

textarea {
  min-height: 110px;
  resize: vertical;
}

button {
  border: 0;
  border-radius: 10px;
  padding: 9px 12px;
  font-weight: 700;
  cursor: pointer;
}

button:disabled {
  opacity: 0.55;
  cursor: not-allowed;
}

.primary {
  color: var(--on-primary);
  background: linear-gradient(160deg, var(--primary), var(--primary-dim));
}

.secondary,
.ghost {
  color: var(--on-surface);
  background: var(--surface-container-low);
}

.error {
  color: #b42318;
  background: color-mix(in srgb, #b42318 12%, var(--surface-container-lowest));
  border: 1px solid color-mix(in srgb, #b42318 30%, var(--surface-container-low));
  border-radius: 10px;
  padding: 10px;
  font-size: 13px;
}

.fatal-error {
  display: grid;
  gap: 12px;
  padding: 24px;
  text-align: center;
  border-style: solid;
  border-width: 2px;
  margin: 20px 0;
}

.fatal-error h3 {
  color: #b42318;
  margin: 0;
}

.fatal-error p {
  margin: 0;
  font-size: 15px;
  color: var(--on-surface);
}

.error-actions {
  display: flex;
  justify-content: center;
  gap: 12px;
  margin-top: 8px;
}

.empty {
  color: var(--muted-text);
  background: var(--surface-container-low);
  border-radius: 10px;
  padding: 12px;
  font-size: 14px;
}

.fatal-error {
  display: grid;
  gap: 12px;
  padding: 16px;
}

.fatal-error h3 {
  margin: 0;
  font-size: 16px;
  color: #b42318;
}

.fatal-error p {
  margin: 0;
  font-size: 13px;
  line-height: 1.5;
}

.error-actions {
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
  margin-top: 4px;
}

@media (max-width: 1180px) {
  .layout,
  .layout.collapsed {
    grid-template-columns: 1fr;
  }
}

/* Custom View Mode Overrides */
.pdf-viewport[data-view-mode='raw'] :deep(.vue-pdf-embed__page canvas) {
  filter: none !important;
}

.pdf-viewport[data-view-mode='raw'] {
  background: #ffffff !important;
}

.pdf-viewport[data-view-mode='light'] :deep(.vue-pdf-embed__page canvas) {
  filter: sepia(0.5) contrast(1.1) brightness(0.95) !important;
}

.pdf-viewport[data-view-mode='light'] {
  background: #f8f1e3 !important;
}

.pdf-viewport[data-view-mode='dark'] :deep(.vue-pdf-embed__page canvas) {
  filter: invert(1) hue-rotate(180deg) !important;
}

.pdf-viewport[data-view-mode='dark'] {
  background: #121214 !important;
}

.pdf-viewport[data-view-mode='sync'] {
  background: var(--background) !important;
}

/* Right-edge PDF Controls */
.pdf-edge-controls {
  position: absolute;
  top: 50%;
  right: 10px;
  transform: translateY(-50%);
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 6px;
  background: color-mix(in srgb, var(--surface-bright) 72%, transparent);
  backdrop-filter: blur(18px);
  -webkit-backdrop-filter: blur(18px);
  padding: 10px 8px;
  border-radius: 20px;
  box-shadow: 0 4px 18px rgba(0, 0, 0, 0.1);
  border: 1px solid color-mix(in srgb, var(--outline-variant) 22%, transparent);
  z-index: 10;
  transition: opacity 0.25s ease;
}

.edge-btn {
  background: transparent;
  color: var(--on-surface);
  border: none;
  width: 30px;
  height: 30px;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  transition:
    background 0.18s ease,
    transform 0.12s ease;
  padding: 0;
  font-size: 15px;
  font-weight: 700;
  line-height: 1;
}

.edge-btn:hover:not(:disabled) {
  background: color-mix(in srgb, var(--surface-container-low) 70%, transparent);
  transform: scale(1.08);
}

.edge-btn:active:not(:disabled) {
  transform: scale(0.94);
}

.edge-btn:disabled {
  opacity: 0.38;
  cursor: not-allowed;
}

.dots-btn {
  font-size: 18px;
  letter-spacing: 1px;
  color: var(--muted-text);
}

.dots-btn:hover {
  color: var(--on-surface);
}

.edge-zoom-val {
  font-size: 11px;
  font-weight: 700;
  color: var(--muted-text);
  min-width: 30px;
  text-align: center;
  user-select: none;
}

.edge-sep {
  width: 18px;
  height: 1px;
  background: color-mix(in srgb, var(--outline-variant) 35%, transparent);
  margin: 2px 0;
}

/* Theme flyout */
.theme-trigger-wrap {
  position: relative;
}

.theme-flyout {
  position: absolute;
  right: calc(100% + 10px);
  top: 50%;
  transform: translateY(-50%);
  display: flex;
  flex-direction: column;
  gap: 4px;
  background: color-mix(in srgb, var(--surface-bright) 90%, transparent);
  backdrop-filter: blur(20px);
  -webkit-backdrop-filter: blur(20px);
  border: 1px solid color-mix(in srgb, var(--outline-variant) 25%, transparent);
  border-radius: 14px;
  padding: 8px 6px;
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.13);
  z-index: 20;
  min-width: 80px;
  animation: flyout-in 0.18s ease;
}

@keyframes flyout-in {
  from {
    opacity: 0;
    transform: translateY(-50%) translateX(6px);
  }
  to {
    opacity: 1;
    transform: translateY(-50%) translateX(0);
  }
}

.flyout-item {
  background: transparent;
  color: var(--muted-text);
  border: none;
  padding: 6px 12px;
  font-size: 12px;
  font-weight: 600;
  border-radius: 10px;
  cursor: pointer;
  text-align: left;
  transition: all 0.15s ease;
  width: 100%;
}

.flyout-item:hover {
  background: color-mix(in srgb, var(--surface-container-low) 60%, transparent);
  color: var(--on-surface);
}

.flyout-item.active {
  background: var(--primary);
  color: var(--on-primary);
}

.chat-disabled {
  color: var(--muted-text);
  background: var(--surface-container-low);
  border-radius: 10px;
  padding: 12px;
  font-size: 14px;
  text-align: center;
  display: flex;
  align-items: center;
  justify-content: center;
}

.debug-overlay {
  position: absolute;
  top: 40px;
  left: 10px;
  background: rgba(0, 0, 0, 0.85);
  color: #00ff00;
  padding: 10px;
  border-radius: 8px;
  font-family: monospace;
  font-size: 11px;
  line-height: 1.4;
  z-index: 100;
  pointer-events: none;
  margin: 0;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.25);
  border: 1px solid rgba(0, 255, 0, 0.3);
}
</style>
