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
                reader.loadingBundle.value || completingSession.value || !activeTaskID.value
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
        <div v-else class="pdf-wrap">
          <iframe
            :key="iframeKey"
            class="pdf-frame"
            :src="reader.pdfSource.value"
            title="Notebook PDF"
          ></iframe>
        </div>

        <p v-if="isTaskFlow && completionMessage" class="completion-message">
          {{ completionMessage }}
        </p>
        <p v-if="isTaskFlow && completionError" class="error">{{ completionError }}</p>
      </article>

      <aside class="panel chat" :class="{ closed: chat.chatCollapsed.value }">
        <div class="chat-head">
          <h2>AI Chat</h2>
          <button class="ghost" @click="chat.toggleChat">
            {{ chat.chatCollapsed.value ? 'Expand' : 'Collapse' }}
          </button>
        </div>

        <template v-if="!chat.chatCollapsed.value">
          <p class="chat-context">
            Using topic <strong>{{ reader.selectedTopicTitle.value || 'None' }}</strong>
            <span v-if="reader.selectedNotebookTitle.value"
              >from {{ reader.selectedNotebookTitle.value }}</span
            >
          </p>

          <div class="scope-bar">
            <div class="scope-main">
              <span class="scope-label">Retrieval Scope</span>
              <select v-model="chat.chatScope.value" class="scope-select">
                <option value="entire_notebook">Entire Notebook</option>
                <option value="current_chapter">Current Chapter</option>
                <option value="current_page">Current Page</option>
              </select>
            </div>
            <p class="scope-helper">Broader scopes search more of your notebook.</p>
          </div>

          <div ref="chat.messagesPane" class="messages">
            <article
              v-for="(msg, idx) in chat.chatMessages.value"
              :key="idx"
              class="msg"
              :class="msg.role"
            >
              <p class="role">{{ msg.role === 'user' ? 'You' : 'Tutor' }}</p>
              <p v-if="msg.role === 'user'">{{ msg.text }}</p>
              <div v-else class="markdown-body" v-html="chat.renderMarkdown(msg.text)"></div>
            </article>
          </div>

          <article v-if="chat.chatError.value" class="error">{{ chat.chatError.value }}</article>

          <label class="field">
            <span>Ask AI</span>
            <textarea
              v-model="chat.chatInput.value"
              :disabled="chat.chatLoading.value || !reader.selectedTopicID.value"
              placeholder="Ask about what you’re reading right now..."
            ></textarea>
          </label>

          <button
            class="primary"
            :disabled="
              chat.chatLoading.value ||
              !chat.chatInput.value.trim() ||
              !reader.selectedTopicID.value
            "
            @click="sendChat"
          >
            {{ chat.chatLoading.value ? 'Thinking...' : 'Send' }}
          </button>
        </template>
      </aside>
    </div>
  </section>
</template>

<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { completeReading } from '../services/appApi'
import { useReaderBase } from '../composables/useReaderBase'
import { useChat } from '../composables/useChat'

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

// Local state for completion
const completingSession = ref(false)
const completionMessage = ref('')
const completionError = ref('')
const iframeKey = ref(0)
const activeTaskID = ref('')

const isTaskFlow = computed(() => !!routeTaskID.value)

// Trust-based completion: user decides when reading is complete.
// Page navigation is for UI only and does not gate completion.

// Initialize on mount
onMounted(async () => {
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

// Navigation methods
function goPrev() {
  if (reader.goPrev()) {
    iframeKey.value++
  }
}

function goNext() {
  if (reader.goNext()) {
    iframeKey.value++
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
    const taskIDForCompletion = activeTaskID.value || routeTaskID.value
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

// Chat wrapper
async function sendChat() {
  await chat.sendMessage({
    topicID: reader.selectedTopicID.value,
    notebookID: reader.selectedNotebookID.value,
    currentPage: reader.currentPage.value,
    chapterStartPage: reader.topicStartPage.value,
    chapterEndPage: reader.topicEndPage.value,
  })
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

.scope-bar {
  display: grid;
  gap: 6px;
  padding: 10px;
  border-radius: 10px;
  background: color-mix(in srgb, var(--surface-container-low) 86%, transparent);
}

.scope-main {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
}

.scope-helper {
  margin: 0;
  font-size: 11px;
  color: var(--muted-text);
  line-height: 1.3;
}

.scope-label {
  font-size: 11px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.08em;
  color: var(--muted-text);
}

.scope-select {
  width: auto;
  min-width: 160px;
  padding: 8px 10px;
  font-size: 13px;
  border-radius: 10px;
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
</style>
