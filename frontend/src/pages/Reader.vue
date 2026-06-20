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
        <button class="primary" @click="window.location.reload()">Retry</button>
      </div>
    </article>

    <div v-else class="layout" :class="{ collapsed: chat.chatCollapsed.value }">
      <article class="panel stage">
        <div class="stage-head">
          <h2>Document Stage</h2>
          <div class="pager">
            <button class="secondary" :disabled="!reader.canGoPrev.value" @click="goPrev">
              Prev
            </button>
            <span>Page {{ reader.currentPage.value }} / {{ reader.pageCount.value }}</span>
            <button class="secondary" :disabled="!reader.canGoNext.value" @click="goNext">
              Next
            </button>
            <button
              v-if="isTaskFlow"
              class="primary"
              :disabled="
                !activeTaskID || reader.loadingBundle.value || completingSession
              "
              @click="completeSession"
            >
              {{ completingSession ? 'Completing Session...' : 'Complete Session' }}
            </button>
          </div>
        </div>
        <p v-if="isTaskFlow && reader.hasNavigationBounds.value" class="lock-meta">
          Reading Window: Pages {{ reader.navigationMinPage.value }}-{{
            reader.navigationMaxPage.value
          }}
        </p>

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
          @keydown="handleViewportKeydown"
        >
          <div v-if="pdfLoadError" class="empty error">{{ pdfLoadError }}</div>
          <div
            v-else
            ref="pdfScalerRef"
            class="pdf-scaler"
            :style="{ width: `${containerWidth}px`, transform: `scale(${zoomScale})`, transformOrigin: 'top left', margin: '0 auto' }"
          >
            <vue-pdf-embed
              ref="pdfRef"
              :source="reader.notebookUrl.value"
              :page="renderedPages"
              @rendered="handlePDFRendered"
              @loading-failed="handlePDFLoadFailed"
            />
          </div>
        </div>

        <!-- Right-edge PDF Controls -->
        <div v-if="reader.pdfVisible.value && !reader.loadingBundle.value && !pdfLoadError" class="pdf-edge-controls">
          <button class="edge-btn zoom-btn" :disabled="zoomScale <= 0.5" title="Zoom out" @click="zoomOut">−</button>
          <span class="edge-zoom-val">{{ Math.round(zoomScale * 100) }}%</span>
          <button class="edge-btn zoom-btn" :disabled="zoomScale >= 2.5" title="Zoom in" @click="zoomIn">+</button>
          <div class="edge-sep"></div>
          <div class="theme-trigger-wrap">
            <button class="edge-btn dots-btn" title="Change theme" :aria-expanded="themeMenuOpen" @click="themeMenuOpen = !themeMenuOpen">···</button>
            <div v-if="themeMenuOpen" class="theme-flyout" role="menu">
              <button
                v-for="mode in ['raw','light','dark','sync']"
                :key="mode"
                class="flyout-item"
                role="menuitem"
                :class="{ active: viewMode === mode }"
                @click="viewMode = mode; themeMenuOpen = false"
              >{{ mode.charAt(0).toUpperCase() + mode.slice(1) }}</button>
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
      <div v-else-if="ragEnabled && !ragQueueStudy" class="chat-disabled">Chat is currently disabled in queue study mode.</div>
    </div>
  </section>
</template>

<script setup>
import { computed, onMounted, onUnmounted, provide, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { completeReading, getUserSettings } from '../services/appApi'
import { useReaderBase } from '../composables/useReaderBase'
import { useChat } from '../composables/useChat'
import ReaderChat from '../components/ReaderChat.vue'
import VuePdfEmbed from 'vue-pdf-embed'
import 'vue-pdf-embed/dist/styles/annotationLayer.css'
import 'vue-pdf-embed/dist/styles/textLayer.css'

const route = useRoute()
const router = useRouter()

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
const activeTaskID = ref('')
const ragEnabled = ref(false)
const ragQueueStudy = ref(true)
const ragSettingsLoaded = ref(false)
const ragSettingsError = ref(null)

// Custom PDF Viewer Refs
const pdfViewportRef = ref(null)
const pdfScalerRef = ref(null)
const pdfRef = ref(null)
const isProgrammaticScrolling = ref(false)
const pdfLoadError = ref('')
let pdfObserver = null
let pdfVisibilityObserver = null
const pageElements = new Map()

// Custom PDF Viewer Zoom & View Modes
const zoomScale = ref(1.0)
let scrollTimeout = null

const viewMode = ref('raw')
const themeMenuOpen = ref(false)
const containerWidth = ref(800)
const iframeKey = ref(0) // Safe fallback for legacy code

const renderedPages = computed(() => {
  return undefined
})

const isTaskFlow = computed(() => !!routeTaskID.value)

// Trust-based completion: user decides when reading is complete.
// Page navigation is for UI only and does not gate completion.

// Initialize on mount
onMounted(async () => {
  ragSettingsLoaded.value = false
  ragSettingsError.value = null
  try {
    const settings = await getUserSettings()
    if (settings && settings.rag_enabled !== undefined) {
      ragEnabled.value = settings.rag_enabled
    }
    if (settings && settings.rag_queue_study !== undefined) {
      ragQueueStudy.value = settings.rag_queue_study
    }
    ragSettingsLoaded.value = true
  } catch (err) {
    console.error('Failed to load settings in Reader:', err)
    ragSettingsError.value = err?.message || 'Failed to load settings'
    ragSettingsLoaded.value = true
  }

  console.log('[Reader] Mounted. route.query:', JSON.stringify(route.query))
  console.log('[Reader] routeTaskID:', routeTaskID.value)

  if (!routeTaskID.value) {
    // Browse mode: load notebook tree for manual selection
    console.log('[Reader] Browse mode - loading notebook tree')
    await reader.loadNotebookTree()
    return
  }

  // Task flow: extract query params properly
  const query = {
    notebookId: route.query.notebookId || route.query.notebook_id,
    topicId: route.query.topicId || route.query.topic_id,
    startPage: parseInt(route.query.startPage || route.query.start_page) || 0,
    endPage: parseInt(route.query.endPage || route.query.end_page) || 0,
  }
  console.log('[Reader] Task flow - extracted query:', JSON.stringify(query))
  console.warn('[Reader] Task flow - pre-init state', {
    routeTaskID: routeTaskID.value,
    fullPath: route.fullPath,
    query,
    isTaskFlow: isTaskFlow.value,
  })

  const init = await reader.initializeSession(query)
  console.log('[Reader] initializeSession result:', init)
  console.warn('[READER_INIT_CLIENT] initializeSession payload ids', {
    routeQueryTaskId: route.query.taskId,
    routeQueryTask_id: route.query.task_id,
    canonicalTaskIDFromInit: init?.task?.task_id || init?.task?.id || null,
  })
  activeTaskID.value = init?.task?.task_id || init?.task?.id || routeTaskID.value
  console.warn('[READER_INIT_CLIENT] activeTaskID assigned', {
    assignedActiveTaskID: activeTaskID.value,
    routeTaskID: routeTaskID.value,
  })
  console.log('[Reader] After init - reader state:', {
    navigationMinPage: reader.navigationMinPage.value,
    navigationMaxPage: reader.navigationMaxPage.value,
    currentPage: reader.currentPage.value,
    hasNavigationBounds: reader.hasNavigationBounds.value,
    navigationState: reader.navigationState.value,
  })
  if (init) {
    iframeKey.value++
  }
})

watch(activeTaskID, (next, prev) => {
  console.warn('[READER_STATE] activeTaskID changed', { previous: prev, next })
})

watch(() => reader.currentPage.value, (newPage) => {
  scrollToPage(newPage)
})

watch(() => reader.notebookUrl.value, () => {
  pdfLoadError.value = ''
})

watch(routeTaskID, (newId) => {
  if (newId) activeTaskID.value = newId
})

// ResizeObserver and Gesture Controller Setup
let resizeObserver = null

// Watch viewport ref to set up ResizeObserver and Event Listeners dynamically
watch(pdfViewportRef, (el, oldEl, onCleanup) => {
  if (resizeObserver) {
    resizeObserver.disconnect()
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

    resizeObserver = new ResizeObserver((entries) => {
      for (let entry of entries) {
        containerWidth.value = entry.contentRect.width
      }
    })
    resizeObserver.observe(el)

    el.addEventListener('touchstart', handleTouchStart, { passive: true })
    el.addEventListener('touchmove', handleTouchMove, { passive: false })
    el.addEventListener('touchend', handleTouchEnd, { passive: true })
    el.addEventListener('wheel', handleWheel, { passive: false })

    onCleanup(() => {
      if (resizeObserver) {
        resizeObserver.disconnect()
      }
      el.removeEventListener('touchstart', handleTouchStart)
      el.removeEventListener('touchmove', handleTouchMove)
      el.removeEventListener('touchend', handleTouchEnd)
      el.removeEventListener('wheel', handleWheel)
    })
  }
})

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
  if (pdfObserver) {
    pdfObserver.disconnect()
  }
  if (pdfVisibilityObserver) {
    pdfVisibilityObserver.disconnect()
  }
  if (resizeObserver) {
    resizeObserver.disconnect()
  }
  if (scrollTimeout) {
    clearTimeout(scrollTimeout)
  }
  pageElements.clear()
})

// Navigation methods
function goPrev() {
  reader.goPrev()
}

function goNext() {
  reader.goNext()
}

function scrollToPage(pageNum) {
  if (!pdfViewportRef.value) return
  
  const pageEl = pageElements.get(pageNum)
  if (!pageEl) return

  // Check if it's already in the center area
  const rect = pageEl.getBoundingClientRect()
  const parentRect = pdfViewportRef.value.getBoundingClientRect()
  
  // Calculate relative top and bottom positions within the parent container
  const relativeTop = rect.top - parentRect.top
  const relativeBottom = rect.bottom - parentRect.top
  const containerHeight = parentRect.height
  
  // We consider it visible if the page element is covering the center horizontal line of viewport
  const isAlreadyCenter = (relativeTop <= containerHeight * 0.5 && relativeBottom >= containerHeight * 0.5)

  if (!isAlreadyCenter) {
    isProgrammaticScrolling.value = true
    pageEl.scrollIntoView({ behavior: 'smooth', block: 'start' })
    
    if (scrollTimeout) {
      clearTimeout(scrollTimeout)
    }
    // Reset programmatic flag after smooth scroll is expected to complete
    scrollTimeout = setTimeout(() => {
      isProgrammaticScrolling.value = false
      scrollTimeout = null
    }, 850)
  }
}

function handlePDFRendered() {
  console.log('[Reader] PDF rendered. Setting up observer and initial scroll.')
  if (pdfObserver) {
    pdfObserver.disconnect()
  }
  if (pdfVisibilityObserver) {
    pdfVisibilityObserver.disconnect()
  }
  pageElements.clear()
  const pages = pdfViewportRef.value?.querySelectorAll('.vue-pdf-embed__page, .vue-pdf-embed > div')
  if (!pages || pages.length === 0) return

  // Observer for active page identification (tight 20% viewport center window)
  pdfObserver = new IntersectionObserver(
    (entries) => {
      if (isProgrammaticScrolling.value) return
      entries.forEach((entry) => {
        if (entry.isIntersecting) {
          const pageNum = parseInt(entry.target.dataset.page)
          if (pageNum && pageNum !== reader.currentPage.value) {
            reader.currentPage.value = pageNum
          }
        }
      })
    },
    {
      root: pdfViewportRef.value,
      rootMargin: '-40% 0px -40% 0px',
      threshold: 0,
    }
  )

  // Observer for rendering/filter virtualization (generous viewport area + 50% vertical margins)
  pdfVisibilityObserver = new IntersectionObserver(
    (entries) => {
      entries.forEach((entry) => {
        if (entry.isIntersecting) {
          entry.target.classList.add('is-visible')
        } else {
          entry.target.classList.remove('is-visible')
        }
      })
    },
    {
      root: pdfViewportRef.value,
      rootMargin: '50% 0px 50% 0px',
      threshold: 0,
    }
  )

  pages.forEach((pageEl, index) => {
    const pageNum = index + 1
    pageEl.dataset.page = pageNum
    pageElements.set(pageNum, pageEl)
    
    // Initially mark page element as not visible until intersection check runs
    pageEl.classList.remove('is-visible')
    
    pdfObserver.observe(pageEl)
    pdfVisibilityObserver.observe(pageEl)
  })

  // Scroll to current page on load
  scrollToPage(reader.currentPage.value)
}

function handlePDFLoadFailed(err) {
  console.error('[Reader] PDF loading failed:', err)
  pdfLoadError.value = err?.message || 'Failed to load PDF document.'
}

async function retryGetUserSettings() {
  ragSettingsLoaded.value = false
  ragSettingsError.value = null
  try {
    const settings = await getUserSettings()
    if (settings && settings.rag_enabled !== undefined) {
      ragEnabled.value = settings.rag_enabled
    }
    if (settings && settings.rag_queue_study !== undefined) {
      ragQueueStudy.value = settings.rag_queue_study
    }
    ragSettingsLoaded.value = true
  } catch (err) {
    ragSettingsError.value = err?.message || 'Failed to load settings'
    ragSettingsLoaded.value = true
  }
}

function onNotebookChange() {
  // Clear topic selection when notebook changes to prevent stale topic IDs
  reader.selectedTopicID.value = ''
  // Don't call loadBundle() here - let user select a topic first
}

async function completeSession() {
  if (completingSession.value || reader.loadingBundle.value || !activeTaskID.value) return

  completionError.value = ''
  completionMessage.value = ''
  completingSession.value = true

  try {
    const taskIDForCompletion = activeTaskID.value || routeTaskID.value || route.query.taskId || route.query.task_id
    console.warn('[COMPLETE_SESSION] pre-completeReading ids', {
      routeQueryTaskId: route.query.taskId,
      routeQueryTask_id: route.query.task_id,
      routeTaskIDComputed: routeTaskID.value,
      activeTaskID: activeTaskID.value,
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
}

.lock-meta {
  margin: 0;
  font-size: 13px;
  color: var(--muted-text);
  display: flex;
  gap: 12px;
  align-items: center;
  flex-wrap: wrap;
}

.stage-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 10px;
}

.pager {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
  color: var(--muted-text);
}

.pdf-viewport {
  width: 100%;
  height: calc(100vh - 160px);
  overflow-y: auto;
  overflow-x: hidden;
  background: var(--background);
  border: none !important;
  margin: 0 !important;
  padding: 0 !important;
  scroll-behavior: smooth;
  border-radius: 10px;
}

.pdf-viewport :deep(.vue-pdf-embed) {
  display: block;
  margin: 0 auto !important;
  padding: 0 !important;
  border: none !important;
  width: 100% !important;
}

.pdf-viewport :deep(.vue-pdf-embed__page) {
  display: block;
  margin: 0 auto !important;
  padding: 0 !important;
  border: none !important;
  margin-bottom: 0px !important;
  box-shadow: none !important;
  content-visibility: auto;
  contain-intrinsic-size: 800px 1100px;
  width: 100% !important;
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
[data-theme="light-warm"] .pdf-viewport :deep(.vue-pdf-embed__page.is-visible canvas) {
  filter: sepia(0.4) contrast(1.05) brightness(0.95);
}

[data-theme="dark-indigo"] .pdf-viewport :deep(.vue-pdf-embed__page.is-visible canvas) {
  filter: invert(0.9) hue-rotate(190deg) brightness(0.9) contrast(1.1);
}

[data-theme="dark-nord"] .pdf-viewport :deep(.vue-pdf-embed__page.is-visible canvas) {
  filter: invert(0.9) hue-rotate(160deg) saturate(0.8) brightness(0.9) contrast(1.1);
}

[data-theme="dark-emerald"] .pdf-viewport :deep(.vue-pdf-embed__page.is-visible canvas) {
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
.pdf-viewport[data-view-mode="raw"] :deep(.vue-pdf-embed__page canvas) {
  filter: none !important;
}

.pdf-viewport[data-view-mode="raw"] {
  background: #ffffff !important;
}

.pdf-viewport[data-view-mode="light"] :deep(.vue-pdf-embed__page.is-visible canvas) {
  filter: sepia(0.5) contrast(1.1) brightness(0.95) !important;
}

.pdf-viewport[data-view-mode="light"] {
  background: #f8f1e3 !important;
}

.pdf-viewport[data-view-mode="dark"] :deep(.vue-pdf-embed__page.is-visible canvas) {
  filter: invert(1) hue-rotate(180deg) !important;
}

.pdf-viewport[data-view-mode="dark"] {
  background: #121214 !important;
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
  box-shadow: 0 4px 18px rgba(0,0,0,0.10);
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
  transition: background 0.18s ease, transform 0.12s ease;
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
  box-shadow: 0 8px 24px rgba(0,0,0,0.13);
  z-index: 20;
  min-width: 80px;
  animation: flyout-in 0.18s ease;
}

@keyframes flyout-in {
  from { opacity: 0; transform: translateY(-50%) translateX(6px); }
  to   { opacity: 1; transform: translateY(-50%) translateX(0); }
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
</style>
