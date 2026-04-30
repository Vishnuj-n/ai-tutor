<template>
  <section class="page">
    <header class="head">
      <p class="eyebrow">Reader</p>
      <h1>{{ topicTitle }}</h1>
      <p class="meta">
        <span>{{ sections.length }} sections</span>
        <span v-if="selectedNotebookTitle">Notebook: {{ selectedNotebookTitle }}</span>
      </p>
    </header>

    <article class="panel controls">
      <label class="field">
        <span>Notebook</span>
        <select v-model="selectedNotebookID" :disabled="loadingTree || notebookTree.length === 0 || loadingBundle" @change="onNotebookChange">
          <option disabled value="">Select notebook</option>
          <option v-for="notebook in notebookTree" :key="notebook.notebook_id" :value="notebook.notebook_id">
            {{ notebook.title }}
          </option>
        </select>
      </label>

      <label class="field">
        <span>Topic</span>
        <select v-model="selectedTopicID" :disabled="loadingTree || availableTopics.length === 0 || loadingBundle" @change="onTopicChange">
          <option disabled value="">
            {{ availableTopics.length === 0 ? 'No topics available' : 'Select topic' }}
          </option>
          <option v-for="topic in availableTopics" :key="topic.topic_id" :value="topic.topic_id">
            {{ topic.title }}
          </option>
        </select>
      </label>
    </article>

    <article v-if="globalError" class="panel error">{{ globalError }}</article>

    <div class="layout" :class="{ collapsed: chatCollapsed }">
      <article class="panel stage">
        <div class="stage-head">
          <h2>Document Stage</h2>
          <div class="pager">
            <button class="secondary" :disabled="!canGoPrev" @click="goPrev">Prev</button>
            <span>Page {{ currentPage }} / {{ pageCount }}</span>
            <button class="secondary" :disabled="!canGoNext" @click="goNext">Next</button>
          </div>
        </div>
        <p v-if="hasLockedWindow" class="lock-meta">Locked Session: Pages {{ lockedStartPage }}-{{ lockedTargetPage }}</p>

        <div v-if="loadingBundle" class="empty">Loading document...</div>
        <div v-else-if="!pdfVisible" class="empty">PDF not available for selected notebook/topic.</div>
        <div v-else class="pdf-wrap">
          <iframe class="pdf-frame" :src="pdfSource" title="Notebook PDF"></iframe>
        </div>

        <article class="complete-session">
          <button class="primary" :disabled="!canCompleteSession" @click="completeSession">
            {{ completingSession ? 'Completing Session...' : 'Complete Session' }}
          </button>
          <p v-if="completionMessage" class="completion-message">{{ completionMessage }}</p>
          <p v-if="completionError" class="error">{{ completionError }}</p>
        </article>
      </article>

      <!-- Boundary Breach Modal -->
      <div v-if="showBoundaryModal" class="modal-overlay" @click.self="closeBoundaryModal">
        <div class="modal-content">
          <h2>Mission Boundary Reached</h2>
          <p class="modal-subtitle">You've reached page {{ lockedTargetPage }} of your assigned reading session.</p>
          
          <div class="modal-options">
            <button class="modal-btn system-btn" @click="handleSystemDefined">
              <span class="btn-title">System Defined</span>
              <span class="btn-desc">Complete session and return to dashboard</span>
            </button>
            
            <button class="modal-btn current-btn" @click="handleCurrent">
              <span class="btn-title">Current</span>
              <span class="btn-desc">Extend to current page ({{ currentPage }})</span>
            </button>
            
            <button class="modal-btn custom-btn" @click="showCustomInput = true">
              <span class="btn-title">Custom</span>
              <span class="btn-desc">Set a custom end page</span>
            </button>
          </div>

          <div v-if="showCustomInput" class="custom-input-section">
            <label for="custom-end-page">Custom End Page:</label>
            <input
              id="custom-end-page"
              v-model.number="customEndPage"
              type="number"
              :min="currentPage"
              :max="pageCount"
              class="custom-input"
            />
            <button class="modal-btn apply-btn" @click="handleCustom">
              Apply Custom Limit
            </button>
          </div>
        </div>
      </div>

      <aside class="panel chat" :class="{ closed: chatCollapsed }">
        <div class="chat-head">
          <h2>AI Chat</h2>
          <button class="ghost" @click="toggleChat">{{ chatCollapsed ? 'Expand' : 'Collapse' }}</button>
        </div>

        <template v-if="!chatCollapsed">
          <p class="chat-context">
            Using topic <strong>{{ selectedTopicTitle || 'None' }}</strong>
            <span v-if="selectedNotebookTitle">from {{ selectedNotebookTitle }}</span>
          </p>

          <div class="messages" ref="messagesPane">
            <article v-for="(msg, idx) in chatMessages" :key="idx" class="msg" :class="msg.role">
              <p class="role">{{ msg.role === 'user' ? 'You' : 'Tutor' }}</p>
              <p v-if="msg.role === 'user'">{{ msg.text }}</p>
              <div v-else class="markdown-body" v-html="renderMarkdown(msg.text)"></div>
            </article>
          </div>

          <article v-if="chatError" class="error">{{ chatError }}</article>

          <label class="field">
            <span>Ask AI</span>
            <textarea
              v-model="chatInput"
              :disabled="chatLoading || !selectedTopicID"
              placeholder="Ask based on selected notebook/topic..."
            ></textarea>
          </label>

          <button class="primary" :disabled="chatLoading || !chatInput.trim() || !selectedTopicID" @click="sendChat">
            {{ chatLoading ? 'Thinking...' : 'Send' }}
          </button>
        </template>
      </aside>
    </div>
  </section>
</template>

<script setup>
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { askAI, completeReadingSession, explainReaderSection, getNotebookTopicTree, getReaderTopicBundle, updateTaskBoundary } from '../services/appApi'
import { renderMarkdown } from '../services/markdown'

const route = useRoute()
const router = useRouter()

const notebookTree = ref([])
const selectedNotebookID = ref('')
const selectedTopicID = ref(typeof route.query.topic === 'string' ? route.query.topic : '')
let routeStartPage = parsePageQueryValue(route.query.start)
let routeEndPage = parsePageQueryValue(route.query.end)

const topicTitle = ref('Reader')
const notebookUrl = ref('')
const fileType = ref('')
const pageCount = ref(1)
const currentPage = ref(1)
const sections = ref([])
const activeSection = ref(null)
const lockedStartPage = ref(0)
const lockedTargetPage = ref(0)
const completingSession = ref(false)
const completionMessage = ref('')
const completionError = ref('')

// Boundary breach modal state
const showBoundaryModal = ref(false)
const boundaryOverrideFlag = ref(false)
const customEndPage = ref(0)
const boundaryTaskID = ref('')
const showCustomInput = ref(false)

const chatCollapsed = ref(false)
const chatMessages = ref([])
const chatInput = ref('')
const chatLoading = ref(false)
const chatError = ref('')
const messagesPane = ref(null)

const loadingTree = ref(true)
const loadingBundle = ref(false)
const globalError = ref('')

const selectedNotebook = computed(() => notebookTree.value.find((n) => n.notebook_id === selectedNotebookID.value) || null)
const selectedNotebookTitle = computed(() => selectedNotebook.value?.title || '')
const selectedTopicTitle = computed(() => {
  const match = availableTopics.value.find((t) => t.topic_id === selectedTopicID.value)
  return match?.title || ''
})

const availableTopics = computed(() => {
  const topics = selectedNotebook.value?.topics || []
  return [...topics].sort((a, b) => {
    const aNum = extractChapterNumber(a.title)
    const bNum = extractChapterNumber(b.title)
    if (aNum !== null || bNum !== null) {
      if (aNum !== null && bNum !== null) {
        if (aNum !== bNum) {
          return aNum - bNum
        }
      } else if (aNum !== null) {
        return -1
      } else if (bNum !== null) {
        return 1
      }
    }
    return a.title.localeCompare(b.title, undefined, { numeric: true, sensitivity: 'base' })
  })
})

const pdfVisible = computed(() => fileType.value === 'pdf' && notebookUrl.value !== '')
const pdfSource = computed(() => `${notebookUrl.value}#page=${currentPage.value}&zoom=page-fit`)
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
const hasLockedWindow = computed(() => lockedStartPage.value > 0 && lockedTargetPage.value >= lockedStartPage.value)
const canCompleteSession = computed(() =>
  !loadingBundle.value &&
  !completingSession.value &&
  Boolean(selectedTopicID.value) &&
  hasLockedWindow.value
)

onMounted(async () => {
  await loadNotebookTree()
  await loadBundle()
})

async function loadNotebookTree() {
  loadingTree.value = true
  globalError.value = ''
  try {
    const data = await getNotebookTopicTree()
    notebookTree.value = Array.isArray(data) ? data : []
    applyInitialSelection()
  } catch (err) {
    globalError.value = err?.message || 'Failed to load notebook/topic options'
  } finally {
    loadingTree.value = false
  }
}

function applyInitialSelection() {
  if (notebookTree.value.length === 0) {
    selectedNotebookID.value = ''
    selectedTopicID.value = ''
    return
  }

  const preferred = selectedTopicID.value
  if (preferred) {
    for (const notebook of notebookTree.value) {
      const hit = Array.isArray(notebook.topics)
        ? notebook.topics.find((topic) => topic.topic_id === preferred)
        : null
      if (hit) {
        selectedNotebookID.value = notebook.notebook_id
        selectedTopicID.value = hit.topic_id
        return
      }
    }
  }

  const firstWithTopics = notebookTree.value.find((n) => Array.isArray(n.topics) && n.topics.length > 0)
  const fallback = firstWithTopics || notebookTree.value[0]
  selectedNotebookID.value = fallback?.notebook_id || ''
  selectedTopicID.value = fallback?.topics?.[0]?.topic_id || ''
}

function extractChapterNumber(title) {
  const matches = /^chapter\s*(\d+)\b/i.exec(String(title).trim())
  if (!matches) {
    return null
  }
  const num = Number(matches[1])
  return Number.isFinite(num) ? num : null
}

function onNotebookChange() {
  if (!availableTopics.value.some((topic) => topic.topic_id === selectedTopicID.value)) {
    selectedTopicID.value = availableTopics.value[0]?.topic_id || ''
  }
  chatMessages.value = []
  completionMessage.value = ''
  completionError.value = ''
  void loadBundle()
}

function onTopicChange() {
  chatMessages.value = []
  completionMessage.value = ''
  completionError.value = ''
  void loadBundle()
}

async function loadBundle() {
  if (!selectedTopicID.value) {
    topicTitle.value = 'Reader'
    notebookUrl.value = ''
    fileType.value = ''
    pageCount.value = 1
    currentPage.value = 1
    sections.value = []
    activeSection.value = null
    globalError.value = 'Select topic to open Reader.'
    return
  }

  loadingBundle.value = true
  globalError.value = ''

  try {
    const result = await getReaderTopicBundle(selectedTopicID.value, selectedNotebookID.value)
    if (result?.error) {
      topicTitle.value = 'Reader'
      notebookUrl.value = ''
      fileType.value = ''
      pageCount.value = 1
      currentPage.value = 1
      sections.value = []
      activeSection.value = null
      // Reset lock state on bundle error
      lockedStartPage.value = 0
      lockedTargetPage.value = 0
      globalError.value = result.error
      return
    }

    topicTitle.value = result?.topic_title || selectedTopicTitle.value || 'Reader'
    notebookUrl.value = result?.notebook_url || ''
    fileType.value = (result?.file_type || '').toLowerCase()
    pageCount.value = Math.max(1, Number(result?.page_count) || 1)
    sections.value = Array.isArray(result?.sections) ? result.sections : []
    activeSection.value = sections.value[0] || null
    const topicStart = Number(result?.topic_start_page) || 1
    const topicEnd = Number(result?.topic_end_page) || pageCount.value
    const normalizedTopicStart = clampPage(topicStart, pageCount.value)
    const normalizedTopicEnd = clampPage(Math.max(topicEnd, normalizedTopicStart), pageCount.value)

    lockedStartPage.value = routeStartPage > 0 ? clampPage(routeStartPage, pageCount.value) : normalizedTopicStart
    lockedTargetPage.value = routeEndPage > 0 ? clampPage(routeEndPage, pageCount.value) : normalizedTopicEnd
    if (lockedTargetPage.value < lockedStartPage.value) {
      lockedTargetPage.value = lockedStartPage.value
    }
    currentPage.value = hasLockedWindow.value ? lockedStartPage.value : normalizedTopicStart
  } catch (err) {
    topicTitle.value = 'Reader'
    notebookUrl.value = ''
    fileType.value = ''
    pageCount.value = 1
    currentPage.value = 1
    sections.value = []
    activeSection.value = null
    lockedStartPage.value = 0
    lockedTargetPage.value = 0
    globalError.value = err?.message || 'Failed to load reader data'
  } finally {
    loadingBundle.value = false
  }
}

function selectSection(section) {
  activeSection.value = section
  const page = Number(section?.page_num)
  if (Number.isFinite(page) && page > 0) {
    currentPage.value = Math.min(Math.max(1, page), pageCount.value)
  }
}

function goPrev() {
  if (canGoPrev.value) {
    currentPage.value -= 1
  }
}

function goNext() {
  if (canGoNext.value) {
    currentPage.value += 1
  }
}

function toggleChat() {
  chatCollapsed.value = !chatCollapsed.value
}

async function completeSession() {
  if (!canCompleteSession.value) {
    return
  }

  completionError.value = ''
  completionMessage.value = ''
  completingSession.value = true
  try {
    const result = await completeReadingSession(
      selectedTopicID.value,
      lockedStartPage.value,
      lockedTargetPage.value
    )
    if (result?.error) {
      completionError.value = result.error
      return
    }
    const generated = Number(result?.questions_generated) || 0
    const nextCursor = Number(result?.current_page_cursor) || lockedTargetPage.value + 1
    completionMessage.value = `Saved ${generated} questions. Cursor advanced to page ${nextCursor}.`
  } catch (err) {
    completionError.value = err?.message || 'Failed to complete session'
  } finally {
    completingSession.value = false
  }
}

async function sendChat() {
  if (!chatInput.value.trim() || !selectedTopicID.value) {
    return
  }

  const question = chatInput.value.trim()
  chatInput.value = ''
  chatError.value = ''
  chatMessages.value.push({ role: 'user', text: question })
  chatLoading.value = true

  try {
    const sectionId = activeSection.value?.id || ''
    const result = sectionId
      ? await explainReaderSection(sectionId, question)
      : await askAI(selectedTopicID.value, question)
    
    if (result?.error) {
      chatError.value = result.error
      return
    }

    chatMessages.value.push({ role: 'assistant', text: result?.answer || 'No answer returned.' })
    await nextTick()
    if (messagesPane.value) {
      messagesPane.value.scrollTop = messagesPane.value.scrollHeight
    }
  } catch (err) {
    chatError.value = err?.message || 'Failed to send message'
  } finally {
    chatLoading.value = false
  }
}

function parsePageQueryValue(value) {
  if (typeof value !== 'string') {
    return 0
  }
  const parsed = Number(value)
  if (!Number.isFinite(parsed) || parsed <= 0) {
    return 0
  }
  return Math.floor(parsed)
}

function clampPage(page, maxPageCount) {
  const max = Math.max(1, Number(maxPageCount) || 1)
  const normalized = Number(page)
  if (!Number.isFinite(normalized) || normalized <= 0) {
    return 1
  }
  if (normalized > max) {
    return max
  }
  return Math.floor(normalized)
}

// Watch route query parameters for start/end page changes
watch(() => route.query.start, () => {
  const newStartPage = parsePageQueryValue(route.query.start)
  if (newStartPage !== routeStartPage) {
    routeStartPage = newStartPage
    void loadBundle()
  }
})

watch(() => route.query.end, () => {
  const newEndPage = parsePageQueryValue(route.query.end)
  if (newEndPage !== routeEndPage) {
    routeEndPage = newEndPage
    void loadBundle()
  }
})

// Watch for boundary breach
watch(currentPage, (newPage) => {
  if (hasLockedWindow.value && !boundaryOverrideFlag.value && newPage > lockedTargetPage.value) {
    showBoundaryModal.value = true
    boundaryTaskID.value = 'read-1' // Default task ID for reading tasks
  }
})

// Boundary breach modal handlers
function closeBoundaryModal() {
  showBoundaryModal.value = false
  showCustomInput.value = false
  customEndPage.value = 0
}

async function handleSystemDefined() {
  closeBoundaryModal()
  boundaryOverrideFlag.value = true
  // Complete the session and return to dashboard
  await completeSession()
  router.push('/dashboard')
}

async function handleCurrent() {
  try {
    const response = await updateTaskBoundary(boundaryTaskID.value, currentPage.value)
    if (response.error) {
      console.error('Failed to update task boundary:', response.error)
      return
    }
    
    // Update the locked target page to current page
    lockedTargetPage.value = currentPage.value
    boundaryOverrideFlag.value = true
    closeBoundaryModal()
    
    // Update route query to reflect new boundary
    router.push({
      path: route.path,
      query: {
        ...route.query,
        end: String(currentPage.value)
      }
    })
  } catch (err) {
    console.error('Error updating task boundary:', err)
  }
}

async function handleCustom() {
  if (!customEndPage.value || customEndPage.value < currentPage.value) {
    alert('Custom end page must be at least the current page.')
    return
  }
  
  if (customEndPage.value > pageCount.value) {
    alert('Custom end page cannot exceed total page count.')
    return
  }
  
  try {
    const response = await updateTaskBoundary(boundaryTaskID.value, customEndPage.value)
    if (response.error) {
      console.error('Failed to update task boundary:', response.error)
      return
    }
    
    // Update the locked target page to custom value
    lockedTargetPage.value = customEndPage.value
    boundaryOverrideFlag.value = true
    closeBoundaryModal()
    
    // Update route query to reflect new boundary
    router.push({
      path: route.path,
      query: {
        ...route.query,
        end: String(customEndPage.value)
      }
    })
  } catch (err) {
    console.error('Error updating task boundary:', err)
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
}

.lock-meta {
  margin: 0;
  font-size: 13px;
  color: var(--muted-text);
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

.pdf-wrap {
  border: 1px solid var(--surface-container-low);
  border-radius: 10px;
  overflow: hidden;
  min-height: 480px;
  background: #f5f6f8;
}

.pdf-frame {
  width: 100%;
  min-height: 640px;
  border: 0;
  display: block;
  background: #fff;
}

.complete-session {
  display: grid;
  gap: 8px;
  justify-items: start;
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

.chat {
  display: grid;
  gap: 10px;
  align-content: start;
}

.chat.closed {
  padding: 10px 8px;
  gap: 0;
}

.chat.closed .chat-head {
  flex-direction: column;
}

.chat.closed h2 {
  writing-mode: vertical-rl;
  text-orientation: mixed;
  transform: rotate(180deg);
  font-size: 14px;
  word-break: break-word;
}

.chat.closed .chat-head button.ghost {
  width: 100%;
  padding: 8px 4px;
  font-size: 11px;
  white-space: normal;
  word-break: break-word;
}

.chat-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 8px;
}

.chat-context {
  margin: 0;
  font-size: 13px;
  color: var(--muted-text);
}

.messages {
  max-height: 320px;
  overflow: auto;
  display: grid;
  gap: 8px;
  padding-right: 3px;
}

.msg {
  border-radius: 10px;
  padding: 9px 10px;
  display: grid;
  gap: 4px;
}

.msg.user {
  background: color-mix(in srgb, var(--primary) 14%, var(--surface-container-lowest));
}

.msg.assistant {
  background: var(--surface-container-low);
}

.msg .role {
  margin: 0;
  font-size: 11px;
  text-transform: uppercase;
  letter-spacing: 0.06em;
  color: var(--muted-text);
  font-weight: 700;
}

.msg p {
  margin: 0;
  font-size: 14px;
  line-height: 1.5;
}

.markdown-body {
  font-size: 14px;
  line-height: 1.6;
}

.markdown-body :first-child {
  margin-top: 0;
}

.markdown-body :last-child {
  margin-bottom: 0;
}

.markdown-body p,
.markdown-body ul,
.markdown-body ol,
.markdown-body pre,
.markdown-body blockquote {
  margin: 0 0 8px;
}

.markdown-body code {
  background: var(--surface-container-low);
  border-radius: 6px;
  padding: 1px 5px;
  font-size: 12px;
}

.markdown-body pre {
  background: var(--surface-container-low);
  border-radius: 8px;
  padding: 8px;
  overflow-x: auto;
}

.markdown-body pre code {
  background: transparent;
  padding: 0;
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

.empty {
  color: var(--muted-text);
  background: var(--surface-container-low);
  border-radius: 10px;
  padding: 12px;
  font-size: 14px;
}

@media (max-width: 1180px) {
  .controls {
    grid-template-columns: 1fr;
  }

  .layout,
  .layout.collapsed {
    grid-template-columns: 1fr;
  }
}
</style>

